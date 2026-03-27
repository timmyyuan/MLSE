#include "mlse/Go/Conversion/BootstrapLowering.h"

#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/Func/Transforms/FuncConversions.h"
#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/Dialect/SCF/IR/SCF.h"
#include "mlir/Dialect/SCF/Transforms/Patterns.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "mlir/Transforms/DialectConversion.h"
#include "mlse/Go/IR/GoDialect.h"
#include "llvm/ADT/Hashing.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringExtras.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/FormatVariadic.h"
#include "llvm/Support/raw_ostream.h"
#include <type_traits>

namespace mlir::mlse::go::conversion {
namespace {

namespace go_dialect = ::mlir::go;

LLVM::LLVMPointerType getOpaquePointerType(MLIRContext *context) {
  return LLVM::LLVMPointerType::get(context);
}

IntegerType getI64Type(MLIRContext *context) {
  return IntegerType::get(context, 64);
}

LLVM::LLVMStructType getStringRuntimeType(MLIRContext *context) {
  return LLVM::LLVMStructType::getLiteral(
      context, {getOpaquePointerType(context), getI64Type(context)});
}

LLVM::LLVMStructType getSliceRuntimeType(MLIRContext *context) {
  return LLVM::LLVMStructType::getLiteral(
      context,
      {getOpaquePointerType(context), getI64Type(context), getI64Type(context)});
}

struct RuntimeHelperCallSpec {
  Location loc;
  ModuleOp module;
  llvm::StringRef base;
  ValueRange operands;
  TypeRange resultTypes;
};

struct ExactHelperCallSpec {
  Location loc;
  ModuleOp module;
  llvm::StringRef symbol;
  ValueRange operands;
  TypeRange resultTypes;
};

func::CallOp createRuntimeHelperCall(const RuntimeHelperCallSpec &spec,
                                     OpBuilder &builder);

func::FuncOp getOrCreateExactHelper(ModuleOp module, llvm::StringRef symbol,
                                    TypeRange argTypes, TypeRange resultTypes);

func::CallOp createPanicIndexCall(Location loc, ModuleOp module, Value index,
                                  Value length,
                                  ConversionPatternRewriter &rewriter);

struct GrowSliceCallSpec {
  Location loc;
  ModuleOp module;
  Value oldPtr;
  Value newLen;
  Value oldCap;
  Value num;
  Value elemSize;
  Type resultType;
};

func::CallOp createGrowSliceCall(const GrowSliceCallSpec &spec,
                                 ConversionPatternRewriter &rewriter);

struct MakeSliceCallSpec {
  Location loc;
  ModuleOp module;
  Value length;
  Value capacity;
  Type resultType;
};

func::CallOp createMakeSliceCall(const MakeSliceCallSpec &spec,
                                 ConversionPatternRewriter &rewriter);

Value castIntegerLike(Location loc, Value value, Type targetType,
                      ConversionPatternRewriter &rewriter) {
  Type sourceType = value.getType();
  if (sourceType == targetType)
    return value;

  if (isa<IndexType>(targetType))
    return rewriter.create<arith::IndexCastOp>(loc, targetType, value);

  auto targetInt = dyn_cast<IntegerType>(targetType);
  if (!targetInt)
    return {};

  if (isa<IndexType>(sourceType))
    return rewriter.create<arith::IndexCastOp>(loc, targetType, value);

  auto sourceInt = dyn_cast<IntegerType>(sourceType);
  if (!sourceInt)
    return {};

  if (sourceInt.getWidth() < targetInt.getWidth())
    return rewriter.create<arith::ExtSIOp>(loc, targetType, value);
  if (sourceInt.getWidth() > targetInt.getWidth())
    return rewriter.create<arith::TruncIOp>(loc, targetType, value);
  return value;
}

Value buildLengthResult(Location loc, Value lengthI64, Type resultType,
                        ConversionPatternRewriter &rewriter) {
  Value casted = castIntegerLike(loc, lengthI64, resultType, rewriter);
  if (casted)
    return casted;
  return {};
}

Value buildI64Constant(Location loc, int64_t value,
                       ConversionPatternRewriter &rewriter) {
  return rewriter.create<arith::ConstantIntOp>(loc, value, 64);
}

struct SliceRuntimeValueSpec {
  Location loc;
  Type resultType;
  Value data;
  Value length;
  Value capacity;
};

Value buildSliceRuntimeValue(const SliceRuntimeValueSpec &spec,
                             ConversionPatternRewriter &rewriter) {
  Value slice = rewriter.create<LLVM::UndefOp>(spec.loc, spec.resultType);
  slice = rewriter.create<LLVM::InsertValueOp>(spec.loc, spec.resultType, slice,
                                               spec.data,
                                               ArrayRef<int64_t>{0});
  slice = rewriter.create<LLVM::InsertValueOp>(spec.loc, spec.resultType, slice,
                                               spec.length,
                                               ArrayRef<int64_t>{1});
  slice = rewriter.create<LLVM::InsertValueOp>(spec.loc, spec.resultType, slice,
                                               spec.capacity,
                                               ArrayRef<int64_t>{2});
  return slice;
}

struct StringRuntimeValueSpec {
  Location loc;
  Type resultType;
  Value data;
  Value length;
};

Value buildStringRuntimeValue(const StringRuntimeValueSpec &spec,
                              ConversionPatternRewriter &rewriter) {
  Value str = rewriter.create<LLVM::UndefOp>(spec.loc, spec.resultType);
  str = rewriter.create<LLVM::InsertValueOp>(spec.loc, spec.resultType, str,
                                             spec.data,
                                             ArrayRef<int64_t>{0});
  str = rewriter.create<LLVM::InsertValueOp>(spec.loc, spec.resultType, str,
                                             spec.length,
                                             ArrayRef<int64_t>{1});
  return str;
}

FailureOr<int64_t> getStaticTypeSizeInBytes(Type type, ModuleOp module) {
  DataLayout layout(module);
  llvm::TypeSize size = layout.getTypeSize(type);
  if (size.isScalable())
    return failure();
  return static_cast<int64_t>(size.getFixedValue());
}

LLVM::GlobalOp getOrCreateStringGlobal(ModuleOp module, StringRef value) {
  MLIRContext *context = module.getContext();
  auto i8Ty = IntegerType::get(context, 8);
  auto arrayTy = LLVM::LLVMArrayType::get(i8Ty, value.size());
  auto valueAttr = StringAttr::get(context, value);
  auto unnamedAddr =
      LLVM::UnnamedAddrAttr::get(context, LLVM::UnnamedAddr::Global);

  std::string base = llvm::formatv(
                         "go.string.constant.{0:x16}",
                         static_cast<uint64_t>(llvm::hash_value(value)))
                         .str();
  std::string symbol = base;
  for (unsigned counter = 0;; ++counter) {
    if (auto existing = module.lookupSymbol<LLVM::GlobalOp>(symbol)) {
      if (existing.getGlobalType() == arrayTy && existing.getValue() &&
          existing.getValueAttr() == valueAttr)
        return existing;
      symbol = (base + "." + std::to_string(counter + 1));
      continue;
    }

    OpBuilder builder(module.getBodyRegion());
    builder.setInsertionPointToStart(module.getBody());
    return builder.create<LLVM::GlobalOp>(
        module.getLoc(), arrayTy, /*constant=*/true, symbol,
        LLVM::Linkage::Private, /*dso_local=*/false,
        /*thread_local_=*/false, /*externally_initialized=*/false, valueAttr,
        /*alignment=*/IntegerAttr(), /*addr_space=*/0, unnamedAddr,
        /*section=*/StringAttr(), /*comdat=*/SymbolRefAttr(),
        /*dbg_exprs=*/ArrayAttr(), LLVM::Visibility::Default);
  }
}

struct CheckedAccessSpec {
  Operation *op;
  Value index;
  Value length;
  Type resultType;
};

template <typename BuildSuccessFn>
LogicalResult replaceWithCheckedBranch(const CheckedAccessSpec &spec,
                                       ConversionPatternRewriter &rewriter,
                                       BuildSuccessFn &&buildSuccess) {
  Location loc = spec.op->getLoc();
  auto execute =
      rewriter.create<scf::ExecuteRegionOp>(loc, TypeRange{spec.resultType});

  Region &region = execute.getRegion();
  Block *entryBlock = rewriter.createBlock(&region);
  Block *thenBlock = rewriter.createBlock(&region);
  Block *elseBlock = rewriter.createBlock(&region);
  llvm::SmallVector<Location, 1> argLocs{loc};
  Block *mergeBlock =
      rewriter.createBlock(&region, {}, TypeRange{spec.resultType}, argLocs);

  rewriter.setInsertionPointToEnd(entryBlock);
  auto cmp = rewriter.create<arith::CmpIOp>(loc, arith::CmpIPredicate::ult,
                                            spec.index, spec.length);
  rewriter.create<cf::CondBranchOp>(loc, cmp, thenBlock, elseBlock);

  rewriter.setInsertionPointToEnd(thenBlock);
  Value successValue = buildSuccess();
  if (!successValue)
    return failure();
  rewriter.create<cf::BranchOp>(loc, mergeBlock, ValueRange{successValue});

  rewriter.setInsertionPointToEnd(elseBlock);
  ModuleOp module = spec.op->getParentOfType<ModuleOp>();
  createPanicIndexCall(loc, module, spec.index, spec.length, rewriter);
  rewriter.create<LLVM::UnreachableOp>(loc);

  rewriter.setInsertionPointToEnd(mergeBlock);
  rewriter.create<scf::YieldOp>(loc, mergeBlock->getArgument(0));

  rewriter.replaceOp(spec.op, execute.getResults());
  return success();
}

func::CallOp createPanicIndexCall(Location loc, ModuleOp module, Value index,
                                  Value length,
                                  ConversionPatternRewriter &rewriter) {
  llvm::SmallVector<Type, 2> argTypes{index.getType(), length.getType()};
  func::FuncOp callee =
      getOrCreateExactHelper(module, "runtime.panic.index", argTypes, {});
  return rewriter.create<func::CallOp>(loc, callee.getName(), TypeRange{},
                                       ValueRange{index, length});
}

func::CallOp createGrowSliceCall(const GrowSliceCallSpec &spec,
                                 ConversionPatternRewriter &rewriter) {
  llvm::SmallVector<Type, 5> argTypes{spec.oldPtr.getType(),
                                      spec.newLen.getType(),
                                      spec.oldCap.getType(), spec.num.getType(),
                                      spec.elemSize.getType()};
  func::FuncOp callee =
      getOrCreateExactHelper(spec.module, "runtime.growslice", argTypes,
                             TypeRange{spec.resultType});
  return rewriter.create<func::CallOp>(
      spec.loc, callee.getName(), TypeRange{spec.resultType},
      ValueRange{spec.oldPtr, spec.newLen, spec.oldCap, spec.num,
                 spec.elemSize});
}

func::CallOp createMakeSliceCall(const MakeSliceCallSpec &spec,
                                 ConversionPatternRewriter &rewriter) {
  auto i64Ty = getI64Type(rewriter.getContext());
  Value length = castIntegerLike(spec.loc, spec.length, i64Ty, rewriter);
  Value capacity = castIntegerLike(spec.loc, spec.capacity, i64Ty, rewriter);
  llvm::SmallVector<Type, 2> argTypes{length.getType(), capacity.getType()};
  func::FuncOp callee = getOrCreateExactHelper(
      spec.module, "runtime.makeslice", argTypes, TypeRange{spec.resultType});
  return rewriter.create<func::CallOp>(
      spec.loc, callee.getName(), TypeRange{spec.resultType},
      ValueRange{length, capacity});
}

Value extractStringData(Location loc, Value value,
                        ConversionPatternRewriter &rewriter) {
  return rewriter.create<LLVM::ExtractValueOp>(loc, getOpaquePointerType(rewriter.getContext()),
                                               value, ArrayRef<int64_t>{0});
}

Value extractStringLen(Location loc, Value value,
                       ConversionPatternRewriter &rewriter) {
  return rewriter.create<LLVM::ExtractValueOp>(loc, getI64Type(rewriter.getContext()),
                                               value, ArrayRef<int64_t>{1});
}

Value extractSliceData(Location loc, Value value,
                       ConversionPatternRewriter &rewriter) {
  return rewriter.create<LLVM::ExtractValueOp>(loc, getOpaquePointerType(rewriter.getContext()),
                                               value, ArrayRef<int64_t>{0});
}

Value extractSliceLen(Location loc, Value value,
                      ConversionPatternRewriter &rewriter) {
  return rewriter.create<LLVM::ExtractValueOp>(loc, getI64Type(rewriter.getContext()),
                                               value, ArrayRef<int64_t>{1});
}

Value extractSliceCap(Location loc, Value value,
                      ConversionPatternRewriter &rewriter) {
  return rewriter.create<LLVM::ExtractValueOp>(loc, getI64Type(rewriter.getContext()),
                                               value, ArrayRef<int64_t>{2});
}

Value buildI1Constant(Location loc, bool value,
                      ConversionPatternRewriter &rewriter) {
  return rewriter.create<arith::ConstantIntOp>(loc, value ? 1 : 0, 1);
}

Value buildSliceIsNil(Location loc, Value slice,
                      ConversionPatternRewriter &rewriter) {
  Value data = extractSliceData(loc, slice, rewriter);
  Value length = extractSliceLen(loc, slice, rewriter);
  Value capacity = extractSliceCap(loc, slice, rewriter);

  auto nullPtr = rewriter.create<LLVM::ZeroOp>(loc, data.getType());
  auto dataIsNull =
      rewriter.create<LLVM::ICmpOp>(loc, LLVM::ICmpPredicate::eq, data, nullPtr);

  Value zero = buildI64Constant(loc, 0, rewriter);
  auto lenIsZero =
      rewriter.create<arith::CmpIOp>(loc, arith::CmpIPredicate::eq, length, zero);
  auto capIsZero =
      rewriter.create<arith::CmpIOp>(loc, arith::CmpIPredicate::eq, capacity, zero);

  auto dataAndLen = rewriter.create<arith::AndIOp>(loc, dataIsNull, lenIsZero);
  return rewriter.create<arith::AndIOp>(loc, dataAndLen, capIsZero);
}

bool isExplicitNilOperand(Value value) {
  return value.getDefiningOp<go_dialect::NilOp>() != nullptr;
}

func::CallOp createExactHelperCall(const ExactHelperCallSpec &spec,
                                   ConversionPatternRewriter &rewriter) {
  llvm::SmallVector<Type, 4> operandTypes;
  operandTypes.reserve(spec.operands.size());
  for (Value operand : spec.operands)
    operandTypes.push_back(operand.getType());

  func::FuncOp callee =
      getOrCreateExactHelper(spec.module, spec.symbol, operandTypes,
                             spec.resultTypes);
  return rewriter.create<func::CallOp>(spec.loc, callee.getName(),
                                       spec.resultTypes, spec.operands);
}

std::string sanitizeSymbolFragment(Type type) {
  std::string text;
  llvm::raw_string_ostream os(text);
  type.print(os);
  os.flush();

  std::string out;
  out.reserve(text.size());
  for (char ch : text) {
    if (llvm::isAlnum(ch) || ch == '_')
      out.push_back(ch);
    else
      out.push_back('_');
  }

  size_t start = out.find_first_not_of('_');
  if (start == std::string::npos)
    return "opaque";
  size_t end = out.find_last_not_of('_');
  return out.substr(start, end - start + 1);
}

std::string runtimeHelperName(llvm::StringRef base, TypeRange argTypes,
                              TypeRange resultTypes) {
  std::string name = base.str();
  for (Type type : argTypes) {
    name += "__";
    name += sanitizeSymbolFragment(type);
  }
  for (Type type : resultTypes) {
    name += "__to__";
    name += sanitizeSymbolFragment(type);
  }
  return name;
}

func::FuncOp getOrCreateRuntimeHelper(ModuleOp module, llvm::StringRef base,
                                      TypeRange argTypes,
                                      TypeRange resultTypes) {
  std::string symbol = runtimeHelperName(base, argTypes, resultTypes);
  if (auto existing = module.lookupSymbol<func::FuncOp>(symbol))
    return existing;

  OpBuilder builder(module.getBodyRegion());
  builder.setInsertionPointToEnd(module.getBody());
  auto funcType = builder.getFunctionType(argTypes, resultTypes);
  auto func = builder.create<func::FuncOp>(module.getLoc(), symbol, funcType);
  func.setPrivate();
  return func;
}

func::FuncOp getOrCreateExactHelper(ModuleOp module, llvm::StringRef symbol,
                                    TypeRange argTypes, TypeRange resultTypes) {
  if (auto existing = module.lookupSymbol<func::FuncOp>(symbol))
    return existing;

  OpBuilder builder(module.getBodyRegion());
  builder.setInsertionPointToEnd(module.getBody());
  auto funcType = builder.getFunctionType(argTypes, resultTypes);
  auto func = builder.create<func::FuncOp>(module.getLoc(), symbol, funcType);
  func.setPrivate();
  return func;
}

func::CallOp createRuntimeHelperCall(const RuntimeHelperCallSpec &spec,
                                     OpBuilder &builder) {
  llvm::SmallVector<Type, 4> operandTypes;
  operandTypes.reserve(spec.operands.size());
  for (Value operand : spec.operands)
    operandTypes.push_back(operand.getType());

  func::FuncOp callee =
      getOrCreateRuntimeHelper(spec.module, spec.base, operandTypes,
                               spec.resultTypes);
  return builder.create<func::CallOp>(spec.loc, callee.getName(),
                                      spec.resultTypes, spec.operands);
}

class GoBootstrapTypeConverter final : public TypeConverter {
public:
  explicit GoBootstrapTypeConverter(MLIRContext *context) {
    addConversion([](Type type) { return type; });
    addConversion([context](go_dialect::StringType) -> Type {
      return getStringRuntimeType(context);
    });
    addConversion([context](go_dialect::ErrorType) -> Type {
      return getOpaquePointerType(context);
    });
    addConversion([context](go_dialect::NamedType) -> Type {
      return getOpaquePointerType(context);
    });
    addConversion([context](go_dialect::PointerType) -> Type {
      return getOpaquePointerType(context);
    });
    addConversion([context](go_dialect::SliceType) -> Type {
      return getSliceRuntimeType(context);
    });
    addConversion([this](FunctionType type) -> std::optional<Type> {
      llvm::SmallVector<Type, 4> inputs;
      llvm::SmallVector<Type, 4> results;
      if (failed(convertTypes(type.getInputs(), inputs)) ||
          failed(convertTypes(type.getResults(), results)))
        return std::nullopt;
      return FunctionType::get(type.getContext(), inputs, results);
    });
  }
};

class GoStringConstantOpLowering
    : public OpConversionPattern<go_dialect::StringConstantOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::StringConstantOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    (void)adaptor;
    Type resultType = getTypeConverter()->convertType(op.getResult().getType());
    if (!resultType)
      return failure();

    auto valueAttr = cast<StringAttr>(op->getAttr("value"));
    ModuleOp module = op->getParentOfType<ModuleOp>();
    auto global = getOrCreateStringGlobal(module, valueAttr.getValue());
    auto addr = rewriter.create<LLVM::AddressOfOp>(op.getLoc(), global);
    Value length = buildI64Constant(
        op.getLoc(), static_cast<int64_t>(valueAttr.getValue().size()),
        rewriter);
    Value result = buildStringRuntimeValue(
        StringRuntimeValueSpec{op.getLoc(), resultType, addr, length},
        rewriter);
    rewriter.replaceOp(op, result);
    return success();
  }
};

class GoNilOpLowering : public OpConversionPattern<go_dialect::NilOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::NilOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    (void)adaptor;
    Type resultType = getTypeConverter()->convertType(op.getResult().getType());
    if (!resultType)
      return failure();
    auto zero = rewriter.create<LLVM::ZeroOp>(op.getLoc(), resultType);
    rewriter.replaceOp(op, zero.getResult());
    return success();
  }
};

class GoMakeSliceOpLowering
    : public OpConversionPattern<go_dialect::MakeSliceOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::MakeSliceOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    ModuleOp module = op->getParentOfType<ModuleOp>();
    auto call = createMakeSliceCall(
        MakeSliceCallSpec{op.getLoc(), module, adaptor.getLength(),
                          adaptor.getCapacity(), resultTypes.front()},
        rewriter);
    rewriter.replaceOp(op, call.getResults());
    return success();
  }
};

class GoLenOpLowering : public OpConversionPattern<go_dialect::LenOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::LenOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    Value length;
    if (isa<go_dialect::StringType>(op.getValue().getType()))
      length = extractStringLen(op.getLoc(), adaptor.getValue(), rewriter);
    else
      length = extractSliceLen(op.getLoc(), adaptor.getValue(), rewriter);

    Value result = buildLengthResult(op.getLoc(), length, resultTypes.front(), rewriter);
    if (!result)
      return failure();
    rewriter.replaceOp(op, result);
    return success();
  }
};

class GoCapOpLowering : public OpConversionPattern<go_dialect::CapOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::CapOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    Value capacity = extractSliceCap(op.getLoc(), adaptor.getValue(), rewriter);
    Value result = buildLengthResult(op.getLoc(), capacity, resultTypes.front(), rewriter);
    if (!result)
      return failure();
    rewriter.replaceOp(op, result);
    return success();
  }
};

template <typename CompareOp>
class GoCompareOpLowering : public OpConversionPattern<CompareOp> {
public:
  using OpConversionPattern<CompareOp>::OpConversionPattern;

  LogicalResult
  matchAndRewrite(CompareOp op, typename CompareOp::Adaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(this->getTypeConverter()->convertTypes(op->getResultTypes(),
                                                      resultTypes)))
      return failure();

    Type operandTy = op.getLhs().getType();
    if (isa<go_dialect::PointerType>(operandTy) ||
        (isa<go_dialect::ErrorType>(operandTy) &&
         (isExplicitNilOperand(op.getLhs()) || isExplicitNilOperand(op.getRhs())))) {
      LLVM::ICmpPredicate predicate = std::is_same_v<CompareOp, go_dialect::EqOp>
                                          ? LLVM::ICmpPredicate::eq
                                          : LLVM::ICmpPredicate::ne;
      auto cmp = rewriter.create<LLVM::ICmpOp>(op.getLoc(), predicate,
                                               adaptor.getLhs(),
                                               adaptor.getRhs());
      rewriter.replaceOp(op, cmp.getResult());
      return success();
    }

    if (isa<go_dialect::SliceType>(operandTy) &&
        (isExplicitNilOperand(op.getLhs()) || isExplicitNilOperand(op.getRhs()))) {
      Value compared = adaptor.getLhs();
      if (isExplicitNilOperand(op.getLhs()) && !isExplicitNilOperand(op.getRhs()))
        compared = adaptor.getRhs();

      Value isNil = buildSliceIsNil(op.getLoc(), compared, rewriter);
      if (std::is_same_v<CompareOp, go_dialect::EqOp>) {
        rewriter.replaceOp(op, isNil);
        return success();
      }

      Value trueValue = buildI1Constant(op.getLoc(), true, rewriter);
      auto isNotNil = rewriter.create<arith::XOrIOp>(op.getLoc(), isNil, trueValue);
      rewriter.replaceOp(op, isNotNil.getResult());
      return success();
    }

    if (isa<go_dialect::StringType>(operandTy)) {
      ModuleOp module = op->template getParentOfType<ModuleOp>();
      llvm::StringRef symbol = std::is_same_v<CompareOp, go_dialect::EqOp>
                                   ? "runtime.eq.string"
                                   : "runtime.neq.string";
      auto call = createExactHelperCall(
          ExactHelperCallSpec{op.getLoc(), module, symbol,
                              ValueRange{adaptor.getLhs(), adaptor.getRhs()},
                              resultTypes},
          rewriter);
      rewriter.replaceOp(op, call.getResults());
      return success();
    }

    return failure();
  }
};

class GoIndexOpLowering : public OpConversionPattern<go_dialect::IndexOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::IndexOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    auto i64Ty = getI64Type(rewriter.getContext());
    Value length = extractStringLen(op.getLoc(), adaptor.getValue(), rewriter);
    Value data = extractStringData(op.getLoc(), adaptor.getValue(), rewriter);
    Value index = castIntegerLike(op.getLoc(), adaptor.getIndex(), i64Ty, rewriter);
    if (!index)
      return failure();

    return replaceWithCheckedBranch(
        CheckedAccessSpec{op.getOperation(), index, length,
                          resultTypes.front()},
        rewriter,
        [&]() -> Value {
          auto gep = rewriter.create<LLVM::GEPOp>(
              op.getLoc(), getOpaquePointerType(rewriter.getContext()),
              IntegerType::get(rewriter.getContext(), 8), data,
              ValueRange{index});
          auto load = rewriter.create<LLVM::LoadOp>(op.getLoc(),
                                                    resultTypes.front(),
                                                    gep.getResult());
          return load.getResult();
        });
  }
};

class GoElemAddrOpLowering
    : public OpConversionPattern<go_dialect::ElemAddrOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::ElemAddrOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    Type elemType = getTypeConverter()->convertType(
        cast<go_dialect::PointerType>(op.getResult().getType()).getPointee());
    if (!elemType)
      return failure();

    auto i64Ty = getI64Type(rewriter.getContext());
    Value length = extractSliceLen(op.getLoc(), adaptor.getBase(), rewriter);
    Value data = extractSliceData(op.getLoc(), adaptor.getBase(), rewriter);
    Value index = castIntegerLike(op.getLoc(), adaptor.getIndex(), i64Ty, rewriter);
    if (!index)
      return failure();

    return replaceWithCheckedBranch(
        CheckedAccessSpec{op.getOperation(), index, length,
                          resultTypes.front()},
        rewriter,
        [&]() -> Value {
          auto gep = rewriter.create<LLVM::GEPOp>(op.getLoc(),
                                                  resultTypes.front(), elemType,
                                                  data, ValueRange{index});
          return gep.getResult();
        });
  }
};

class GoAppendOpLowering : public OpConversionPattern<go_dialect::AppendOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::AppendOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    auto sliceType = dyn_cast<go_dialect::SliceType>(op.getSlice().getType());
    if (!sliceType)
      return failure();
    Type elemType =
        getTypeConverter()->convertType(sliceType.getElementType());
    if (!elemType)
      return failure();

    ModuleOp module = op->getParentOfType<ModuleOp>();
    FailureOr<int64_t> elemSize = getStaticTypeSizeInBytes(elemType, module);
    if (failed(elemSize))
      return failure();

    Location loc = op.getLoc();
    Value oldPtr = extractSliceData(loc, adaptor.getSlice(), rewriter);
    Value oldLen = extractSliceLen(loc, adaptor.getSlice(), rewriter);
    Value oldCap = extractSliceCap(loc, adaptor.getSlice(), rewriter);
    Value appendCount =
        buildI64Constant(loc, static_cast<int64_t>(adaptor.getValues().size()),
                         rewriter);
    Value elemSizeValue = buildI64Constant(loc, *elemSize, rewriter);
    Value newLen =
        rewriter.create<arith::AddIOp>(loc, oldLen, appendCount).getResult();
    Value needGrow =
        rewriter
            .create<arith::CmpIOp>(loc, arith::CmpIPredicate::ugt, newLen,
                                   oldCap)
            .getResult();

    Type sliceRuntimeType = resultTypes.front();
    auto ifOp =
        rewriter.create<scf::IfOp>(loc, TypeRange{sliceRuntimeType}, needGrow,
                                   /*withElseRegion=*/true);

    rewriter.setInsertionPointToStart(&ifOp.getThenRegion().front());
    auto grownSlice = createGrowSliceCall(
        GrowSliceCallSpec{loc, module, oldPtr, newLen, oldCap, appendCount,
                          elemSizeValue, sliceRuntimeType},
        rewriter);
    rewriter.create<scf::YieldOp>(loc, grownSlice.getResult(0));

    rewriter.setInsertionPointToStart(&ifOp.getElseRegion().front());
    Value sliceValue = buildSliceRuntimeValue(
        SliceRuntimeValueSpec{loc, sliceRuntimeType, oldPtr, newLen, oldCap},
        rewriter);
    rewriter.create<scf::YieldOp>(loc, sliceValue);

    rewriter.setInsertionPointAfter(ifOp);
    Value updatedSlice = ifOp.getResult(0);
    Value data = extractSliceData(loc, updatedSlice, rewriter);
    for (auto indexedValue : llvm::enumerate(adaptor.getValues())) {
      Value offset =
          buildI64Constant(loc, static_cast<int64_t>(indexedValue.index()),
                           rewriter);
      Value index =
          rewriter.create<arith::AddIOp>(loc, oldLen, offset).getResult();
      auto addr = rewriter.create<LLVM::GEPOp>(loc, getOpaquePointerType(
                                                        rewriter.getContext()),
                                               elemType, data,
                                               ValueRange{index});
      rewriter.create<LLVM::StoreOp>(loc, indexedValue.value(), addr.getResult(),
                                     0, false, false, false);
    }

    rewriter.replaceOp(op, updatedSlice);
    return success();
  }
};

class GoAppendSliceOpLowering
    : public OpConversionPattern<go_dialect::AppendSliceOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::AppendSliceOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    auto sliceType = dyn_cast<go_dialect::SliceType>(op.getResult().getType());
    if (!sliceType)
      return failure();
    Type elemType =
        getTypeConverter()->convertType(sliceType.getElementType());
    if (!elemType)
      return failure();

    ModuleOp module = op->getParentOfType<ModuleOp>();
    FailureOr<int64_t> elemSize = getStaticTypeSizeInBytes(elemType, module);
    if (failed(elemSize))
      return failure();

    Location loc = op.getLoc();
    Value oldPtr = extractSliceData(loc, adaptor.getDst(), rewriter);
    Value oldLen = extractSliceLen(loc, adaptor.getDst(), rewriter);
    Value oldCap = extractSliceCap(loc, adaptor.getDst(), rewriter);
    Value srcPtr = extractSliceData(loc, adaptor.getSrc(), rewriter);
    Value srcLen = extractSliceLen(loc, adaptor.getSrc(), rewriter);
    Value elemSizeValue = buildI64Constant(loc, *elemSize, rewriter);
    Value newLen =
        rewriter.create<arith::AddIOp>(loc, oldLen, srcLen).getResult();
    Value needGrow =
        rewriter
            .create<arith::CmpIOp>(loc, arith::CmpIPredicate::ugt, newLen,
                                   oldCap)
            .getResult();

    Type sliceRuntimeType = resultTypes.front();
    auto ifOp =
        rewriter.create<scf::IfOp>(loc, TypeRange{sliceRuntimeType}, needGrow,
                                   /*withElseRegion=*/true);

    rewriter.setInsertionPointToStart(&ifOp.getThenRegion().front());
    auto grownSlice = createGrowSliceCall(
        GrowSliceCallSpec{loc, module, oldPtr, newLen, oldCap, srcLen,
                          elemSizeValue, sliceRuntimeType},
        rewriter);
    rewriter.create<scf::YieldOp>(loc, grownSlice.getResult(0));

    rewriter.setInsertionPointToStart(&ifOp.getElseRegion().front());
    Value sliceValue = buildSliceRuntimeValue(
        SliceRuntimeValueSpec{loc, sliceRuntimeType, oldPtr, newLen, oldCap},
        rewriter);
    rewriter.create<scf::YieldOp>(loc, sliceValue);

    rewriter.setInsertionPointAfter(ifOp);
    Value updatedSlice = ifOp.getResult(0);
    Value dstData = extractSliceData(loc, updatedSlice, rewriter);
    auto dstAddr = rewriter.create<LLVM::GEPOp>(
        loc, getOpaquePointerType(rewriter.getContext()), elemType, dstData,
        ValueRange{oldLen});
    Value byteLen =
        rewriter.create<arith::MulIOp>(loc, srcLen, elemSizeValue).getResult();
    rewriter.create<LLVM::MemmoveOp>(loc, dstAddr.getResult(), srcPtr, byteLen,
                                     /*isVolatile=*/false);

    rewriter.replaceOp(op, updatedSlice);
    return success();
  }
};

class GoFieldAddrOpLowering
    : public OpConversionPattern<go_dialect::FieldAddrOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::FieldAddrOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    ModuleOp module = op->getParentOfType<ModuleOp>();
    llvm::SmallVector<Type, 4> operandTypes;
    operandTypes.reserve(adaptor.getOperands().size());
    for (Value operand : adaptor.getOperands())
      operandTypes.push_back(operand.getType());

    std::string symbol = ("runtime.field.addr." + op.getField()).str();
    func::FuncOp callee =
        getOrCreateExactHelper(module, symbol, operandTypes, resultTypes);
    auto call = rewriter.create<func::CallOp>(op.getLoc(), callee.getName(),
                                              resultTypes, adaptor.getOperands());
    rewriter.replaceOp(op, call.getResults());
    return success();
  }
};

class GoLoadOpLowering : public OpConversionPattern<go_dialect::LoadOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::LoadOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    llvm::SmallVector<Type, 1> resultTypes;
    if (failed(getTypeConverter()->convertTypes(op->getResultTypes(),
                                                resultTypes)))
      return failure();
    auto load = rewriter.create<LLVM::LoadOp>(op.getLoc(), resultTypes.front(),
                                              adaptor.getAddr(), 0, false, false,
                                              false, false);
    rewriter.replaceOp(op, load.getResult());
    return success();
  }
};

class GoStoreOpLowering : public OpConversionPattern<go_dialect::StoreOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(go_dialect::StoreOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    rewriter.create<LLVM::StoreOp>(op.getLoc(), adaptor.getValue(), adaptor.getAddr(),
                                   0, false, false, false);
    rewriter.eraseOp(op);
    return success();
  }
};

class FuncConstantOpLowering : public OpConversionPattern<func::ConstantOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(func::ConstantOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    (void)adaptor;
    Type loweredType = getTypeConverter()->convertType(op.getResult().getType());
    if (!loweredType)
      return failure();
    auto symbol = cast<FlatSymbolRefAttr>(op->getAttr("value"));
    auto lowered = rewriter.create<func::ConstantOp>(op.getLoc(), loweredType,
                                                     symbol);
    rewriter.replaceOp(op, lowered.getResult());
    return success();
  }
};

class FuncCallIndirectOpLowering
    : public OpConversionPattern<func::CallIndirectOp> {
public:
  using OpConversionPattern::OpConversionPattern;

  LogicalResult
  matchAndRewrite(func::CallIndirectOp op, OpAdaptor adaptor,
                  ConversionPatternRewriter &rewriter) const override {
    auto lowered = rewriter.create<func::CallIndirectOp>(
        op.getLoc(), adaptor.getCallee(), adaptor.getCalleeOperands());
    rewriter.replaceOp(op, lowered.getResults());
    return success();
  }
};

} // namespace

LogicalResult lowerGoBuiltins(ModuleOp module) {
  llvm::SmallVector<Operation *, 16> worklist;
  module.walk([&](Operation *op) {
    if (llvm::isa<go_dialect::LenOp, go_dialect::CapOp,
                  go_dialect::IndexOp, go_dialect::AppendOp,
                  go_dialect::AppendSliceOp>(op))
      worklist.push_back(op);
  });

  for (Operation *op : worklist) {
    if (auto lenOp = dyn_cast<go_dialect::LenOp>(op)) {
      OpBuilder builder(op);
      auto call = createRuntimeHelperCall(
          RuntimeHelperCallSpec{
              op->getLoc(),
              module,
              "__mlse_go_len",
              ValueRange{lenOp.getValue()},
              op->getResultTypes(),
          },
          builder);
      op->replaceAllUsesWith(call.getResults());
      op->erase();
      continue;
    }
    if (auto capOp = dyn_cast<go_dialect::CapOp>(op)) {
      OpBuilder builder(op);
      auto call = createRuntimeHelperCall(
          RuntimeHelperCallSpec{
              op->getLoc(),
              module,
              "__mlse_go_cap",
              ValueRange{capOp.getValue()},
              op->getResultTypes(),
          },
          builder);
      op->replaceAllUsesWith(call.getResults());
      op->erase();
      continue;
    }
    if (auto indexOp = dyn_cast<go_dialect::IndexOp>(op)) {
      OpBuilder builder(op);
      auto call = createRuntimeHelperCall(
          RuntimeHelperCallSpec{
              op->getLoc(),
              module,
              "__mlse_go_index",
              ValueRange{indexOp.getValue(), indexOp.getIndex()},
              op->getResultTypes(),
          },
          builder);
      op->replaceAllUsesWith(call.getResults());
      op->erase();
      continue;
    }
    if (auto appendOp = dyn_cast<go_dialect::AppendOp>(op)) {
      llvm::SmallVector<Value, 4> operands;
      operands.push_back(appendOp.getSlice());
      for (Value value : appendOp.getValues())
        operands.push_back(value);
      OpBuilder builder(op);
      auto call = createRuntimeHelperCall(
          RuntimeHelperCallSpec{
              op->getLoc(),
              module,
              "__mlse_go_append",
              operands,
              op->getResultTypes(),
          },
          builder);
      op->replaceAllUsesWith(call.getResults());
      op->erase();
      continue;
    }
    if (auto appendSliceOp = dyn_cast<go_dialect::AppendSliceOp>(op)) {
      OpBuilder builder(op);
      auto call = createRuntimeHelperCall(
          RuntimeHelperCallSpec{
              op->getLoc(),
              module,
              "__mlse_go_append_slice",
              ValueRange{appendSliceOp.getDst(), appendSliceOp.getSrc()},
              op->getResultTypes(),
          },
          builder);
      op->replaceAllUsesWith(call.getResults());
      op->erase();
      continue;
    }
  }

  return success();
}

LogicalResult lowerGoBootstrap(ModuleOp module) {
  MLIRContext *context = module.getContext();
  GoBootstrapTypeConverter typeConverter(context);
  ConversionTarget target(*context);
  RewritePatternSet patterns(context);

  populateFunctionOpInterfaceTypeConversionPattern<func::FuncOp>(patterns,
                                                                 typeConverter);
  populateCallOpTypeConversionPattern(patterns, typeConverter);
  populateBranchOpInterfaceTypeConversionPattern(patterns, typeConverter);
  populateReturnOpTypeConversionPattern(patterns, typeConverter);
  scf::populateSCFStructuralTypeConversionsAndLegality(typeConverter, patterns,
                                                       target);

  patterns.add<GoStringConstantOpLowering, GoNilOpLowering, GoMakeSliceOpLowering,
               GoLenOpLowering, GoCapOpLowering, GoCompareOpLowering<go_dialect::EqOp>,
               GoCompareOpLowering<go_dialect::NeqOp>, GoIndexOpLowering,
               GoElemAddrOpLowering, GoAppendOpLowering,
               GoAppendSliceOpLowering, GoFieldAddrOpLowering, GoLoadOpLowering,
               GoStoreOpLowering, FuncConstantOpLowering,
               FuncCallIndirectOpLowering>(typeConverter, context);

  target.addLegalOp<ModuleOp>();
  target.addIllegalDialect<go_dialect::GoDialect>();
  target.addDynamicallyLegalOp<func::FuncOp>([&](func::FuncOp op) {
    return typeConverter.isSignatureLegal(op.getFunctionType()) &&
           typeConverter.isLegal(&op.getBody());
  });
  target.markUnknownOpDynamicallyLegal([&](Operation *op) {
    if (isa<go_dialect::StringConstantOp, go_dialect::NilOp,
            go_dialect::MakeSliceOp, go_dialect::LenOp, go_dialect::EqOp,
            go_dialect::NeqOp,
            go_dialect::CapOp, go_dialect::IndexOp, go_dialect::ElemAddrOp,
            go_dialect::AppendOp, go_dialect::AppendSliceOp,
            go_dialect::FieldAddrOp,
            go_dialect::LoadOp, go_dialect::StoreOp, go_dialect::TodoOp,
            go_dialect::TodoValueOp>(op)) {
      return false;
    }
    return typeConverter.isLegal(op) &&
           llvm::all_of(op->getRegions(), [&](Region &region) {
             return typeConverter.isLegal(&region);
           });
  });

  return applyFullConversion(module, target, std::move(patterns));
}

} // namespace mlir::mlse::go::conversion

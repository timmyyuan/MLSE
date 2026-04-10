#include "mlse/Execution/Interpreter.h"

#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/Interfaces/DataLayoutInterfaces.h"
#include "llvm/ADT/DenseMap.h"
#include "llvm/ADT/ScopeExit.h"
#include "llvm/ADT/SmallString.h"
#include "llvm/Support/Casting.h"
#include <algorithm>
#include <cstdint>
#include <cstring>
#include <limits>
#include <optional>
#include <string>
#include <utility>
#include <vector>

namespace mlir::mlse::exec {

ExecValue ExecValue::makeUndef() { return {}; }

ExecValue ExecValue::makeInteger(unsigned width, uint64_t value) {
  ExecValue out;
  out.kind = Kind::Integer;
  out.bitWidth = width;
  out.bits = value;
  return out;
}

ExecValue ExecValue::makePointer(uint64_t rawPointer) {
  ExecValue out;
  out.kind = Kind::Pointer;
  out.bits = rawPointer;
  return out;
}

ExecValue ExecValue::makeAggregate(std::vector<ExecValue> values) {
  ExecValue out;
  out.kind = Kind::Aggregate;
  out.elements = std::move(values);
  return out;
}

namespace {

using ::mlir::LLVM::AddressOfOp;
using ::mlir::LLVM::AddOp;
using ::mlir::LLVM::BrOp;
using ::mlir::LLVM::CallOp;
using ::mlir::LLVM::CondBrOp;
using ::mlir::LLVM::ConstantOp;
using ::mlir::LLVM::ExtractValueOp;
using ::mlir::LLVM::GEPOp;
using ::mlir::LLVM::GlobalOp;
using ::mlir::LLVM::ICmpOp;
using ::mlir::LLVM::ICmpPredicate;
using ::mlir::LLVM::InsertValueOp;
using ::mlir::LLVM::LLVMFuncOp;
using ::mlir::LLVM::LLVMPointerType;
using ::mlir::LLVM::LLVMStructType;
using ::mlir::LLVM::LoadOp;
using ::mlir::LLVM::ReturnOp;
using ::mlir::LLVM::StoreOp;
using ::mlir::LLVM::UndefOp;
using ::mlir::LLVM::UnreachableOp;
using ::mlir::LLVM::ZeroOp;

constexpr uint64_t kNullPointer = 0;
constexpr uint64_t kPointerSize = 8;

uint64_t packPointer(uint32_t objectId, uint32_t offset) {
  return (static_cast<uint64_t>(objectId) << 32) | offset;
}

uint32_t pointerObjectId(uint64_t rawPointer) {
  return static_cast<uint32_t>(rawPointer >> 32);
}

uint32_t pointerOffset(uint64_t rawPointer) {
  return static_cast<uint32_t>(rawPointer & 0xffffffffULL);
}

uint64_t maskBits(unsigned width) {
  if (width == 0 || width >= 64)
    return std::numeric_limits<uint64_t>::max();
  return (uint64_t{1} << width) - 1;
}

uint64_t truncateBits(uint64_t bits, unsigned width) {
  return bits & maskBits(width);
}

int64_t signExtend(uint64_t bits, unsigned width) {
  if (width == 0 || width >= 64)
    return static_cast<int64_t>(bits);
  uint64_t signBit = uint64_t{1} << (width - 1);
  uint64_t masked = truncateBits(bits, width);
  if ((masked & signBit) == 0)
    return static_cast<int64_t>(masked);
  uint64_t extended = masked | (~maskBits(width));
  return static_cast<int64_t>(extended);
}

enum class HeapObjectKind {
  Bytes,
  DeferredSliceBuffer,
  BoxedAny,
};

struct HeapObject {
  HeapObjectKind kind = HeapObjectKind::Bytes;
  std::vector<uint8_t> bytes;
  bool immutable = false;
  uint64_t capacityElements = 0;
  uint64_t elementSize = 0;
  ExecValue boxedValue;
};

struct BranchTarget {
  Block *block = nullptr;
  llvm::SmallVector<ExecValue, 4> args;
};

struct Frame {
  LLVMFuncOp function;
  llvm::DenseMap<Value, ExecValue> values;
};

bool hasFunctionBody(LLVMFuncOp func) {
  return !func.getBody().empty();
}

void appendLE(std::vector<uint8_t> &out, uint64_t value, unsigned bytes) {
  for (unsigned i = 0; i < bytes; ++i)
    out.push_back(static_cast<uint8_t>((value >> (i * 8)) & 0xff));
}

FailureOr<uint64_t> readLE(llvm::ArrayRef<uint8_t> bytes) {
  if (bytes.size() > 8)
    return failure();
  uint64_t out = 0;
  for (size_t i = 0; i < bytes.size(); ++i)
    out |= static_cast<uint64_t>(bytes[i]) << (i * 8);
  return out;
}

} // namespace

struct ExecutionContext::Impl {
  explicit Impl(ModuleOp mod, llvm::raw_ostream &stdoutStream,
                llvm::raw_ostream &stderrStream)
      : module(mod), layout(mod), out(stdoutStream), err(stderrStream) {
    objects.emplace_back();
  }

  ModuleOp module;
  DataLayout layout;
  llvm::raw_ostream &out;
  llvm::raw_ostream &err;
  std::vector<HeapObject> objects;
  bool trapped = false;
  std::string trapMessage;
};

ExecutionContext::ExecutionContext(ModuleOp module, llvm::raw_ostream &out,
                                   llvm::raw_ostream &err)
    : impl(new Impl(module, out, err)) {}

ExecutionContext::~ExecutionContext() { delete impl; }

ModuleOp ExecutionContext::getModule() const { return impl->module; }

unsigned ExecutionContext::getBitWidth(Type type) const {
  if (auto intTy = dyn_cast<IntegerType>(type))
    return intTy.getWidth();
  if (isa<LLVMPointerType>(type))
    return 64;
  return 0;
}

uint64_t ExecutionContext::getTypeSize(Type type) const {
  if (isa<LLVMPointerType>(type))
    return kPointerSize;
  if (auto intTy = dyn_cast<IntegerType>(type))
    return std::max<unsigned>(1, intTy.getWidth() / 8);
  if (auto structTy = dyn_cast<LLVMStructType>(type)) {
    uint64_t size = 0;
    for (Type elemTy : structTy.getBody())
      size += getTypeSize(elemTy);
    return size;
  }
  if (auto arrayTy = dyn_cast<LLVM::LLVMArrayType>(type))
    return arrayTy.getNumElements() * getTypeSize(arrayTy.getElementType());
  llvm::TypeSize size = impl->layout.getTypeSize(type);
  if (size.isScalable())
    return 0;
  return size.getFixedValue();
}

ExecValue ExecutionContext::zeroValueForType(Type type) const {
  if (auto intTy = dyn_cast<IntegerType>(type))
    return ExecValue::makeInteger(intTy.getWidth(), 0);
  if (isa<LLVMPointerType>(type))
    return ExecValue::makePointer(kNullPointer);
  if (auto structTy = dyn_cast<LLVMStructType>(type)) {
    std::vector<ExecValue> elements;
    elements.reserve(structTy.getBody().size());
    for (Type elemTy : structTy.getBody())
      elements.push_back(zeroValueForType(elemTy));
    return ExecValue::makeAggregate(std::move(elements));
  }
  return ExecValue::makeUndef();
}

ExecValue ExecutionContext::undefValueForType(Type type) const {
  if (auto structTy = dyn_cast<LLVMStructType>(type)) {
    std::vector<ExecValue> elements;
    elements.reserve(structTy.getBody().size());
    for (Type elemTy : structTy.getBody())
      elements.push_back(undefValueForType(elemTy));
    return ExecValue::makeAggregate(std::move(elements));
  }
  return ExecValue::makeUndef();
}

uint64_t ExecutionContext::makeNullPointer() const { return kNullPointer; }

uint64_t ExecutionContext::allocateBytes(uint64_t size, bool immutable) {
  HeapObject object;
  object.kind = HeapObjectKind::Bytes;
  object.bytes.assign(size, 0);
  object.immutable = immutable;
  impl->objects.push_back(std::move(object));
  return packPointer(static_cast<uint32_t>(impl->objects.size() - 1), 0);
}

uint64_t ExecutionContext::allocateDeferredSliceBuffer(uint64_t capacityElements) {
  HeapObject object;
  object.kind = HeapObjectKind::DeferredSliceBuffer;
  object.capacityElements = capacityElements;
  impl->objects.push_back(std::move(object));
  return packPointer(static_cast<uint32_t>(impl->objects.size() - 1), 0);
}

uint64_t ExecutionContext::allocateGlobalBytes(llvm::StringRef bytes) {
  HeapObject object;
  object.kind = HeapObjectKind::Bytes;
  object.bytes.assign(bytes.bytes_begin(), bytes.bytes_end());
  object.immutable = true;
  impl->objects.push_back(std::move(object));
  return packPointer(static_cast<uint32_t>(impl->objects.size() - 1), 0);
}

uint64_t ExecutionContext::allocateBoxedAny(const ExecValue &value) {
  HeapObject object;
  object.kind = HeapObjectKind::BoxedAny;
  object.boxedValue = value;
  impl->objects.push_back(std::move(object));
  return packPointer(static_cast<uint32_t>(impl->objects.size() - 1), 0);
}

LogicalResult ExecutionContext::ensureBufferElementSize(uint64_t rawPointer,
                                                        uint64_t elemSize) {
  if (rawPointer == kNullPointer)
    return success();
  uint32_t objectId = pointerObjectId(rawPointer);
  if (objectId >= impl->objects.size())
    return failure();
  HeapObject &object = impl->objects[objectId];
  if (object.kind == HeapObjectKind::Bytes)
    return success();
  if (object.kind != HeapObjectKind::DeferredSliceBuffer)
    return failure();
  if (object.elementSize == 0) {
    object.elementSize = elemSize;
    object.kind = HeapObjectKind::Bytes;
    object.bytes.assign(object.capacityElements * elemSize, 0);
    return success();
  }
  if (object.elementSize != elemSize)
    return failure();
  object.kind = HeapObjectKind::Bytes;
  if (object.bytes.empty())
    object.bytes.assign(object.capacityElements * elemSize, 0);
  return success();
}

LogicalResult ExecutionContext::growBuffer(const GrowBufferRequest &request,
                                           uint64_t &newRawPointer) {
  newRawPointer = allocateBytes(request.newCapacityElements * request.elemSize);
  if (request.oldRawPointer == kNullPointer || request.oldLengthElements == 0)
    return success();
  if (failed(ensureBufferElementSize(request.oldRawPointer, request.elemSize)))
    return failure();
  uint32_t oldId = pointerObjectId(request.oldRawPointer);
  uint32_t oldOff = pointerOffset(request.oldRawPointer);
  HeapObject &oldObject = impl->objects[oldId];
  if (oldObject.kind != HeapObjectKind::Bytes)
    return failure();
  uint32_t newId = pointerObjectId(newRawPointer);
  HeapObject &newObject = impl->objects[newId];
  uint64_t copyBytes =
      std::min<uint64_t>(request.oldLengthElements, request.oldCapacityElements) *
      request.elemSize;
  if (oldOff + copyBytes > oldObject.bytes.size() || copyBytes > newObject.bytes.size())
    return failure();
  std::copy_n(oldObject.bytes.begin() + oldOff, copyBytes, newObject.bytes.begin());
  return success();
}

static FailureOr<llvm::ArrayRef<uint8_t>> getObjectByteWindow(ExecutionContext::Impl *impl,
                                                              uint64_t rawPointer,
                                                              uint64_t size) {
  if (rawPointer == kNullPointer)
    return failure();
  uint32_t objectId = pointerObjectId(rawPointer);
  uint32_t offset = pointerOffset(rawPointer);
  if (objectId >= impl->objects.size())
    return failure();
  HeapObject &object = impl->objects[objectId];
  if (object.kind != HeapObjectKind::Bytes)
    return failure();
  if (offset + size > object.bytes.size())
    return failure();
  return llvm::ArrayRef<uint8_t>(object.bytes.data() + offset, size);
}

static LogicalResult setObjectBytes(ExecutionContext::Impl *impl,
                                    uint64_t rawPointer,
                                    llvm::ArrayRef<uint8_t> payload) {
  if (rawPointer == kNullPointer)
    return failure();
  uint32_t objectId = pointerObjectId(rawPointer);
  uint32_t offset = pointerOffset(rawPointer);
  if (objectId >= impl->objects.size())
    return failure();
  HeapObject &object = impl->objects[objectId];
  if (object.kind != HeapObjectKind::Bytes || object.immutable)
    return failure();
  if (offset + payload.size() > object.bytes.size())
    return failure();
  std::copy(payload.begin(), payload.end(), object.bytes.begin() + offset);
  return success();
}

static void encodeValueRecursive(const ExecutionContext &ctx, Type type,
                                 const ExecValue &value,
                                 std::vector<uint8_t> &bytes) {
  if (auto intTy = dyn_cast<IntegerType>(type)) {
    appendLE(bytes, truncateBits(value.getIntegerBits(), intTy.getWidth()),
             std::max<unsigned>(1, intTy.getWidth() / 8));
    return;
  }
  if (isa<LLVMPointerType>(type)) {
    appendLE(bytes, value.getPointerRaw(), kPointerSize);
    return;
  }
  if (auto structTy = dyn_cast<LLVMStructType>(type)) {
    for (size_t i = 0; i < structTy.getBody().size(); ++i)
      encodeValueRecursive(ctx, structTy.getBody()[i], value.elements[i], bytes);
    return;
  }
}

static FailureOr<ExecValue> decodeValueRecursive(const ExecutionContext &ctx,
                                                 Type type,
                                                 llvm::ArrayRef<uint8_t> bytes,
                                                 size_t &offset) {
  if (auto intTy = dyn_cast<IntegerType>(type)) {
    unsigned size = std::max<unsigned>(1, intTy.getWidth() / 8);
    if (offset + size > bytes.size())
      return failure();
    FailureOr<uint64_t> raw = readLE(bytes.slice(offset, size));
    if (failed(raw))
      return failure();
    offset += size;
    return ExecValue::makeInteger(intTy.getWidth(),
                                  truncateBits(*raw, intTy.getWidth()));
  }
  if (isa<LLVMPointerType>(type)) {
    if (offset + kPointerSize > bytes.size())
      return failure();
    FailureOr<uint64_t> raw = readLE(bytes.slice(offset, kPointerSize));
    if (failed(raw))
      return failure();
    offset += kPointerSize;
    return ExecValue::makePointer(*raw);
  }
  if (auto structTy = dyn_cast<LLVMStructType>(type)) {
    std::vector<ExecValue> elements;
    elements.reserve(structTy.getBody().size());
    for (Type elemTy : structTy.getBody()) {
      FailureOr<ExecValue> decoded = decodeValueRecursive(ctx, elemTy, bytes, offset);
      if (failed(decoded))
        return failure();
      elements.push_back(*decoded);
    }
    return ExecValue::makeAggregate(std::move(elements));
  }
  return failure();
}

LogicalResult ExecutionContext::store(uint64_t rawPointer, Type type,
                                      const ExecValue &value) {
  if (failed(ensureBufferElementSize(rawPointer, getTypeSize(type))))
    return failure();
  std::vector<uint8_t> bytes;
  bytes.reserve(getTypeSize(type));
  encodeValueRecursive(*this, type, value, bytes);
  return setObjectBytes(impl, rawPointer, bytes);
}

FailureOr<ExecValue> ExecutionContext::load(uint64_t rawPointer, Type type) {
  if (failed(ensureBufferElementSize(rawPointer, getTypeSize(type))))
    return failure();
  FailureOr<llvm::ArrayRef<uint8_t>> bytes = getObjectByteWindow(impl, rawPointer, getTypeSize(type));
  if (failed(bytes))
    return failure();
  size_t offset = 0;
  return decodeValueRecursive(*this, type, *bytes, offset);
}

FailureOr<uint64_t> ExecutionContext::gep(uint64_t rawPointer, Type elemType,
                                          int64_t index) {
  if (rawPointer == kNullPointer)
    return failure();
  if (failed(ensureBufferElementSize(rawPointer, getTypeSize(elemType))))
    return failure();
  uint32_t objectId = pointerObjectId(rawPointer);
  uint32_t offset = pointerOffset(rawPointer);
  uint64_t stride = getTypeSize(elemType);
  int64_t newOffset = static_cast<int64_t>(offset) + index * static_cast<int64_t>(stride);
  if (newOffset < 0 || newOffset > std::numeric_limits<uint32_t>::max())
    return failure();
  return packPointer(objectId, static_cast<uint32_t>(newOffset));
}

FailureOr<std::string> ExecutionContext::readGoString(const ExecValue &value) {
  if (!value.isAggregate() || value.elements.size() != 2 ||
      !value.elements[0].isPointer() || !value.elements[1].isInteger())
    return failure();
  uint64_t rawPointer = value.elements[0].getPointerRaw();
  uint64_t length = value.elements[1].getIntegerBits();
  if (length == 0)
    return std::string();
  FailureOr<llvm::ArrayRef<uint8_t>> bytes = getObjectByteWindow(impl, rawPointer, length);
  if (failed(bytes))
    return failure();
  return std::string(reinterpret_cast<const char *>(bytes->data()), bytes->size());
}

FailureOr<std::vector<uint64_t>> ExecutionContext::readPointerSlice(const ExecValue &sliceValue) {
  if (!sliceValue.isAggregate() || sliceValue.elements.size() != 3 ||
      !sliceValue.elements[0].isPointer() || !sliceValue.elements[1].isInteger())
    return failure();
  uint64_t rawPointer = sliceValue.elements[0].getPointerRaw();
  uint64_t length = sliceValue.elements[1].getIntegerBits();
  std::vector<uint64_t> out;
  out.reserve(length);
  if (length == 0)
    return out;
  if (failed(ensureBufferElementSize(rawPointer, kPointerSize)))
    return failure();
  FailureOr<llvm::ArrayRef<uint8_t>> bytes =
      getObjectByteWindow(impl, rawPointer, length * kPointerSize);
  if (failed(bytes))
    return failure();
  for (uint64_t i = 0; i < length; ++i) {
    FailureOr<uint64_t> raw = readLE(bytes->slice(i * kPointerSize, kPointerSize));
    if (failed(raw))
      return failure();
    out.push_back(*raw);
  }
  return out;
}

FailureOr<std::string> ExecutionContext::formatBoxedAny(uint64_t rawPointer) {
  if (rawPointer == kNullPointer)
    return std::string("<nil>");
  uint32_t objectId = pointerObjectId(rawPointer);
  if (objectId >= impl->objects.size())
    return failure();
  HeapObject &object = impl->objects[objectId];
  if (object.kind != HeapObjectKind::BoxedAny)
    return failure();
  const ExecValue &value = object.boxedValue;
  if (value.isInteger())
    return std::to_string(signExtend(value.getIntegerBits(), value.bitWidth));
  if (value.isAggregate() && value.elements.size() == 2)
    return readGoString(value);
  if (value.isPointer() && value.getPointerRaw() == kNullPointer)
    return std::string("<nil>");
  return std::string("<unsupported>");
}

void ExecutionContext::appendStdout(llvm::StringRef text) { impl->out << text; }

void ExecutionContext::appendStderr(llvm::StringRef text) { impl->err << text; }

void ExecutionContext::trap(llvm::StringRef message) {
  impl->trapped = true;
  impl->trapMessage = std::string(message);
}

bool ExecutionContext::isTrapped() const { return impl->trapped; }

llvm::StringRef ExecutionContext::getTrapMessage() const {
  return impl->trapMessage;
}

namespace {

class Interpreter {
public:
  Interpreter(ModuleOp module, RuntimeBundle &runtime, ExecutionContext &ctx)
      : module(module), runtime(runtime), ctx(ctx) {
    for (LLVMFuncOp func : module.getOps<LLVMFuncOp>())
      functions.try_emplace(func.getSymName(), func);
    for (GlobalOp global : module.getOps<GlobalOp>()) {
      std::optional<Attribute> value = global.getValue();
      if (!value)
        continue;
      auto stringAttr = dyn_cast<StringAttr>(*value);
      if (!stringAttr)
        continue;
      globals.try_emplace(global.getSymName(), ctx.allocateGlobalBytes(stringAttr.getValue()));
    }
  }

  FailureOr<LLVMFuncOp> resolveEntry(llvm::StringRef requested) {
    if (!requested.empty()) {
      auto it = functions.find(requested);
      if (it == functions.end() || !hasFunctionBody(it->second))
        return failure();
      return it->second;
    }
    for (llvm::StringRef candidate : {"main.main", "main"}) {
      auto it = functions.find(candidate);
      if (it != functions.end() && hasFunctionBody(it->second))
        return it->second;
    }
    for (auto &[name, func] : functions) {
      if (!hasFunctionBody(func))
        continue;
      if (func.isPrivate())
        continue;
      if (func.getFunctionType().getNumParams() == 0)
        return func;
    }
    return failure();
  }

  LogicalResult run(LLVMFuncOp entry, ExecutionResult &result) {
    llvm::SmallVector<ExecValue, 4> outputs;
    if (failed(executeFunction(entry, {}, outputs)))
      return failure();
    result.ok = !ctx.isTrapped();
    result.trapped = ctx.isTrapped();
    if (ctx.isTrapped())
      result.trapMessage = ctx.getTrapMessage().str();
    return success();
  }

private:
  FailureOr<ExecValue> lookup(Frame &frame, Value value) {
    auto it = frame.values.find(value);
    if (it == frame.values.end())
      return failure();
    return it->second;
  }

  LogicalResult storeResult(Frame &frame, Value result, const ExecValue &value) {
    frame.values[result] = value;
    return success();
  }

  LogicalResult enterBlock(Frame &frame, Block *block, llvm::ArrayRef<ExecValue> args) {
    if (block->getNumArguments() != args.size())
      return failure();
    for (auto [arg, value] : llvm::zip(block->getArguments(), args))
      frame.values[arg] = value;
    return success();
  }

  FailureOr<ExecValue> evalValue(Frame &frame, Value value) { return lookup(frame, value); }

  FailureOr<BranchTarget> executeTerminator(Frame &frame, Operation &op,
                                            llvm::SmallVectorImpl<ExecValue> &returns,
                                            bool &returned) {
    if (auto br = dyn_cast<BrOp>(op)) {
      BranchTarget target;
      target.block = br.getDest();
      for (Value operand : br.getDestOperands()) {
        FailureOr<ExecValue> value = evalValue(frame, operand);
        if (failed(value))
          return failure();
        target.args.push_back(*value);
      }
      return target;
    }
    if (auto condBr = dyn_cast<CondBrOp>(op)) {
      FailureOr<ExecValue> conditionValue = evalValue(frame, condBr.getCondition());
      if (failed(conditionValue) || !conditionValue->isInteger())
        return failure();
      bool taken = conditionValue->getIntegerBits() != 0;
      BranchTarget target;
      target.block = taken ? condBr.getTrueDest() : condBr.getFalseDest();
      auto operands = taken ? condBr.getTrueDestOperands() : condBr.getFalseDestOperands();
      for (Value operand : operands) {
        FailureOr<ExecValue> value = evalValue(frame, operand);
        if (failed(value))
          return failure();
        target.args.push_back(*value);
      }
      return target;
    }
    if (auto ret = dyn_cast<ReturnOp>(op)) {
      returned = true;
      for (Value operand : ret.getOperands()) {
        FailureOr<ExecValue> value = evalValue(frame, operand);
        if (failed(value))
          return failure();
        returns.push_back(*value);
      }
      return BranchTarget{};
    }
    if (isa<UnreachableOp>(op)) {
      returned = true;
      if (!ctx.isTrapped())
        ctx.trap("trap: reached llvm.unreachable");
      return BranchTarget{};
    }
    return failure();
  }

  LogicalResult executeCall(Frame &frame, CallOp op) {
    llvm::SmallVector<ExecValue, 4> args;
    args.reserve(op.getArgOperands().size());
    for (Value operand : op.getArgOperands()) {
      FailureOr<ExecValue> value = evalValue(frame, operand);
      if (failed(value))
        return failure();
      args.push_back(*value);
    }

    llvm::SmallVector<ExecValue, 4> results;
    if (std::optional<llvm::StringRef> callee = op.getCallee()) {
      auto it = functions.find(*callee);
      if (it != functions.end() && hasFunctionBody(it->second)) {
        if (failed(executeFunction(it->second, args, results)))
          return failure();
      } else {
        if (failed(runtime.call(*callee, args, op.getResultTypes(), ctx, results))) {
          llvm::SmallString<128> message("mlse-run: unhandled runtime symbol ");
          message += *callee;
          ctx.trap(message);
          return failure();
        }
      }
    } else {
      ctx.trap("mlse-run: indirect llvm.call is not supported in the current MVP");
      return failure();
    }

    if (results.size() != op.getNumResults()) {
      ctx.trap("mlse-run: call result arity mismatch");
      return failure();
    }
    for (auto [result, value] : llvm::zip(op.getResults(), results))
      frame.values[result] = value;
    return success();
  }

  LogicalResult executeOperation(Frame &frame, Operation &op) {
    if (auto constant = dyn_cast<ConstantOp>(op)) {
      Attribute attr = constant.getValue();
      auto intAttr = dyn_cast<IntegerAttr>(attr);
      if (!intAttr)
        return failure();
      auto intTy = dyn_cast<IntegerType>(intAttr.getType());
      if (!intTy)
        return failure();
      frame.values[constant.getResult()] =
          ExecValue::makeInteger(intTy.getWidth(),
                                 truncateBits(intAttr.getValue().getZExtValue(),
                                              intTy.getWidth()));
      return success();
    }
    if (auto zero = dyn_cast<ZeroOp>(op)) {
      frame.values[zero.getResult()] = ctx.zeroValueForType(zero.getType());
      return success();
    }
    if (auto undef = dyn_cast<UndefOp>(op)) {
      frame.values[undef.getResult()] = ctx.undefValueForType(undef.getType());
      return success();
    }
    if (auto addressOf = dyn_cast<AddressOfOp>(op)) {
      auto it = globals.find(addressOf.getGlobalName());
      if (it == globals.end())
        return failure();
      frame.values[addressOf.getResult()] = ExecValue::makePointer(it->second);
      return success();
    }
    if (auto add = dyn_cast<AddOp>(op)) {
      FailureOr<ExecValue> lhs = evalValue(frame, add.getLhs());
      FailureOr<ExecValue> rhs = evalValue(frame, add.getRhs());
      if (failed(lhs) || failed(rhs) || !lhs->isInteger() || !rhs->isInteger())
        return failure();
      unsigned width = lhs->bitWidth;
      frame.values[add.getResult()] =
          ExecValue::makeInteger(width,
                                 truncateBits(lhs->getIntegerBits() + rhs->getIntegerBits(),
                                              width));
      return success();
    }
    if (auto cmp = dyn_cast<ICmpOp>(op)) {
      FailureOr<ExecValue> lhs = evalValue(frame, cmp.getLhs());
      FailureOr<ExecValue> rhs = evalValue(frame, cmp.getRhs());
      if (failed(lhs) || failed(rhs))
        return failure();
      bool result = false;
      switch (cmp.getPredicate()) {
      case ICmpPredicate::eq:
        result = lhs->bits == rhs->bits;
        break;
      case ICmpPredicate::ne:
        result = lhs->bits != rhs->bits;
        break;
      case ICmpPredicate::ugt:
        result = lhs->getIntegerBits() > rhs->getIntegerBits();
        break;
      case ICmpPredicate::uge:
        result = lhs->getIntegerBits() >= rhs->getIntegerBits();
        break;
      case ICmpPredicate::ult:
        result = lhs->getIntegerBits() < rhs->getIntegerBits();
        break;
      case ICmpPredicate::ule:
        result = lhs->getIntegerBits() <= rhs->getIntegerBits();
        break;
      case ICmpPredicate::sgt:
        result = signExtend(lhs->getIntegerBits(), lhs->bitWidth) >
                 signExtend(rhs->getIntegerBits(), rhs->bitWidth);
        break;
      case ICmpPredicate::sge:
        result = signExtend(lhs->getIntegerBits(), lhs->bitWidth) >=
                 signExtend(rhs->getIntegerBits(), rhs->bitWidth);
        break;
      case ICmpPredicate::slt:
        result = signExtend(lhs->getIntegerBits(), lhs->bitWidth) <
                 signExtend(rhs->getIntegerBits(), rhs->bitWidth);
        break;
      case ICmpPredicate::sle:
        result = signExtend(lhs->getIntegerBits(), lhs->bitWidth) <=
                 signExtend(rhs->getIntegerBits(), rhs->bitWidth);
        break;
      }
      frame.values[cmp.getResult()] = ExecValue::makeInteger(1, result ? 1 : 0);
      return success();
    }
    if (auto extract = dyn_cast<ExtractValueOp>(op)) {
      FailureOr<ExecValue> container = evalValue(frame, extract.getContainer());
      if (failed(container))
        return failure();
      ExecValue current = *container;
      for (int64_t index : extract.getPosition()) {
        if (!current.isAggregate() ||
            index < 0 || static_cast<size_t>(index) >= current.elements.size())
          return failure();
        current = current.elements[static_cast<size_t>(index)];
      }
      frame.values[extract.getResult()] = current;
      return success();
    }
    if (auto insert = dyn_cast<InsertValueOp>(op)) {
      FailureOr<ExecValue> container = evalValue(frame, insert.getContainer());
      FailureOr<ExecValue> value = evalValue(frame, insert.getValue());
      if (failed(container) || failed(value))
        return failure();
      ExecValue out = *container;
      ExecValue *cursor = &out;
      llvm::ArrayRef<int64_t> position = insert.getPosition();
      for (size_t i = 0; i < position.size(); ++i) {
        int64_t index = position[i];
        if (!cursor->isAggregate() ||
            index < 0 || static_cast<size_t>(index) >= cursor->elements.size())
          return failure();
        if (i + 1 == position.size()) {
          cursor->elements[static_cast<size_t>(index)] = *value;
        } else {
          cursor = &cursor->elements[static_cast<size_t>(index)];
        }
      }
      frame.values[insert.getResult()] = out;
      return success();
    }
    if (auto gep = dyn_cast<GEPOp>(op)) {
      auto indices = gep.getIndices();
      if (indices.size() != 1)
        return failure();
      FailureOr<ExecValue> base = evalValue(frame, gep.getBase());
      if (failed(base) || !base->isPointer())
        return failure();
      int64_t indexValue = 0;
      auto index = indices[0];
      if (auto attr = index.template dyn_cast<IntegerAttr>()) {
        indexValue = attr.getInt();
      } else {
        auto operand = llvm::cast<Value>(index);
        FailureOr<ExecValue> dynamicIndex = evalValue(frame, operand);
        if (failed(dynamicIndex) || !dynamicIndex->isInteger())
          return failure();
        indexValue = signExtend(dynamicIndex->getIntegerBits(), dynamicIndex->bitWidth);
      }
      FailureOr<uint64_t> raw = ctx.gep(base->getPointerRaw(), gep.getElemType(), indexValue);
      if (failed(raw))
        return failure();
      frame.values[gep.getResult()] = ExecValue::makePointer(*raw);
      return success();
    }
    if (auto load = dyn_cast<LoadOp>(op)) {
      FailureOr<ExecValue> addr = evalValue(frame, load.getAddr());
      if (failed(addr) || !addr->isPointer())
        return failure();
      FailureOr<ExecValue> value = ctx.load(addr->getPointerRaw(), load.getResult().getType());
      if (failed(value))
        return failure();
      frame.values[load.getResult()] = *value;
      return success();
    }
    if (auto store = dyn_cast<StoreOp>(op)) {
      FailureOr<ExecValue> value = evalValue(frame, store.getValue());
      FailureOr<ExecValue> addr = evalValue(frame, store.getAddr());
      if (failed(value) || failed(addr) || !addr->isPointer())
        return failure();
      return ctx.store(addr->getPointerRaw(), store.getValue().getType(), *value);
    }
    if (auto call = dyn_cast<CallOp>(op))
      return executeCall(frame, call);
    llvm::SmallString<128> message("mlse-run: unsupported op ");
    message += op.getName().getStringRef();
    ctx.trap(message);
    return failure();
  }

  LogicalResult executeFunction(LLVMFuncOp func,
                                llvm::ArrayRef<ExecValue> args,
                                llvm::SmallVectorImpl<ExecValue> &returns) {
    if (!hasFunctionBody(func)) {
      ctx.trap("mlse-run: attempted to execute declaration");
      return failure();
    }
    Frame frame;
    frame.function = func;
    Block &entry = func.getBody().front();
    if (entry.getNumArguments() != args.size()) {
      ctx.trap("mlse-run: function argument arity mismatch");
      return failure();
    }
    if (failed(enterBlock(frame, &entry, args)))
      return failure();

    Block *currentBlock = &entry;
    while (currentBlock) {
      for (Operation &op : *currentBlock) {
        if (ctx.isTrapped())
          return failure();
        if (op.hasTrait<OpTrait::IsTerminator>()) {
          bool returned = false;
          FailureOr<BranchTarget> target = executeTerminator(frame, op, returns, returned);
          if (failed(target)) {
            llvm::SmallString<128> message("mlse-run: failed to execute terminator ");
            message += op.getName().getStringRef();
            ctx.trap(message);
            return failure();
          }
          if (returned)
            return success();
          currentBlock = target->block;
          if (failed(enterBlock(frame, currentBlock, target->args))) {
            ctx.trap("mlse-run: failed to enter destination block");
            return failure();
          }
          goto next_block;
        }
        if (failed(executeOperation(frame, op))) {
          if (!ctx.isTrapped()) {
            llvm::SmallString<128> message("mlse-run: failed to execute op ");
            message += op.getName().getStringRef();
            ctx.trap(message);
          }
          return failure();
        }
      }
      ctx.trap("mlse-run: block terminated without LLVM terminator");
      return failure();
    next_block:
      continue;
    }
    ctx.trap("mlse-run: fell off function without return");
    return failure();
  }

  ModuleOp module;
  RuntimeBundle &runtime;
  ExecutionContext &ctx;
  llvm::DenseMap<llvm::StringRef, LLVMFuncOp> functions;
  llvm::DenseMap<llvm::StringRef, uint64_t> globals;
};

} // namespace

LogicalResult runModule(ModuleOp module,
                        const ExecutionOptions &options,
                        RuntimeBundle &runtime,
                        ExecutionStreams streams,
                        ExecutionResult &result) {
  ExecutionContext ctx(module, streams.out, streams.err);
  Interpreter interpreter(module, runtime, ctx);
  FailureOr<LLVMFuncOp> entry = interpreter.resolveEntry(options.entry);
  if (failed(entry)) {
    result.exitCode = 1;
    result.trapMessage = "mlse-run: failed to resolve entry function";
    return failure();
  }
  if (failed(interpreter.run(*entry, result))) {
    result.exitCode = 1;
    if (ctx.isTrapped())
      result.trapMessage = ctx.getTrapMessage().str();
    return failure();
  }
  result.ok = !ctx.isTrapped();
  result.trapped = ctx.isTrapped();
  if (ctx.isTrapped())
    result.trapMessage = ctx.getTrapMessage().str();
  return success();
}

} // namespace mlir::mlse::exec

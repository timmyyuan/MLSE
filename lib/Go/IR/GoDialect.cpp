#include "mlse/Go/IR/GoDialect.h"
#include "llvm/ADT/TypeSwitch.h"
#include "mlir/IR/Builders.h"
#include "mlir/IR/DialectImplementation.h"

using namespace mlir;
using namespace mlir::go;

#include "mlse/Go/IR/GoDialect.cpp.inc"

void GoDialect::initialize() {
  addTypes<
#define GET_TYPEDEF_LIST
#include "mlse/Go/IR/GoTypes.cpp.inc"
      >();
  addOperations<
#define GET_OP_LIST
#include "mlse/Go/IR/GoOps.cpp.inc"
      >();
}

#define GET_TYPEDEF_CLASSES
#include "mlse/Go/IR/GoTypes.cpp.inc"

#define GET_OP_CLASSES
#include "mlse/Go/IR/GoOps.cpp.inc"

static bool isSupportedCompareType(Type type) {
  return isa<StringType, PointerType>(type);
}

static bool isExplicitNilOperand(Value value) {
  return value.getDefiningOp<NilOp>() != nullptr;
}

static bool isNilOnlyCompareType(Type type) {
  return isa<ErrorType, SliceType>(type);
}

template <typename CompareOp>
static LogicalResult verifyCompareOp(CompareOp op) {
  Type lhsTy = op.getLhs().getType();
  Type rhsTy = op.getRhs().getType();
  if (lhsTy != rhsTy)
    return op.emitOpError() << "expects matching operand types, but got "
                            << lhsTy << " and " << rhsTy;

  if (isSupportedCompareType(lhsTy))
    return success();

  if (isNilOnlyCompareType(lhsTy)) {
    if (isExplicitNilOperand(op.getLhs()) || isExplicitNilOperand(op.getRhs()))
      return success();
    return op.emitOpError() << "currently only supports " << lhsTy
                            << " comparisons against go.nil";
  }

  return op.emitOpError() << "expects !go.string, !go.ptr<...>, !go.error"
                          << " (with go.nil), or !go.slice<...> (with go.nil), but got "
                          << lhsTy;
}

LogicalResult EqOp::verify() { return verifyCompareOp(*this); }

LogicalResult NeqOp::verify() { return verifyCompareOp(*this); }

LogicalResult IndexOp::verify() {
  Type containerTy = getValue().getType();
  Type resultTy = getResult().getType();

  if (isa<StringType>(containerTy)) {
    IntegerType i8Ty = IntegerType::get(getContext(), 8);
    if (resultTy != i8Ty)
      return emitOpError() << "expects result type i8 for string operand, but got "
                           << resultTy;
    return success();
  }

  return emitOpError() << "expects string operand, but got " << containerTy;
}

LogicalResult ElemAddrOp::verify() {
  auto sliceTy = dyn_cast<SliceType>(getBase().getType());
  if (!sliceTy)
    return emitOpError() << "expects slice operand, but got " << getBase().getType();

  auto resultTy = dyn_cast<PointerType>(getResult().getType());
  if (!resultTy)
    return emitOpError() << "expects pointer result, but got "
                         << getResult().getType();

  if (resultTy.getPointee() != sliceTy.getElementType())
    return emitOpError() << "expects result pointee type "
                         << sliceTy.getElementType() << " for slice operand, but got "
                         << resultTy.getPointee();

  return success();
}

LogicalResult AppendOp::verify() {
  auto sliceTy = dyn_cast<SliceType>(getSlice().getType());
  if (!sliceTy)
    return emitOpError() << "expects slice operand, but got " << getSlice().getType();

  if (getResult().getType() != sliceTy)
    return emitOpError() << "expects result type " << sliceTy << ", but got "
                         << getResult().getType();

  if (getValues().empty())
    return emitOpError() << "expects at least one appended value";

  Type elementTy = sliceTy.getElementType();
  for (auto [index, value] : llvm::enumerate(getValues())) {
    if (value.getType() != elementTy)
      return emitOpError() << "expects appended value #" << index
                           << " to have element type " << elementTy << ", but got "
                           << value.getType();
  }
  return success();
}

LogicalResult AppendSliceOp::verify() {
  auto dstTy = dyn_cast<SliceType>(getDst().getType());
  if (!dstTy)
    return emitOpError() << "expects destination slice operand, but got "
                         << getDst().getType();

  auto srcTy = dyn_cast<SliceType>(getSrc().getType());
  if (!srcTy)
    return emitOpError() << "expects source slice operand, but got "
                         << getSrc().getType();

  if (dstTy.getElementType() != srcTy.getElementType())
    return emitOpError() << "expects source element type "
                         << dstTy.getElementType() << ", but got "
                         << srcTy.getElementType();

  if (getResult().getType() != dstTy)
    return emitOpError() << "expects result type " << dstTy << ", but got "
                         << getResult().getType();

  return success();
}

LogicalResult FieldAddrOp::verify() {
  auto resultTy = dyn_cast<PointerType>(getResult().getType());
  if (!resultTy)
    return emitOpError() << "expects pointer result, but got "
                         << getResult().getType();

  if (getField().empty())
    return emitOpError() << "expects non-empty field name";

  Type baseTy = getBase().getType();
  if (!isa<NamedType, PointerType>(baseTy))
    return emitOpError() << "expects named or pointer base, but got " << baseTy;

  if (auto offsetAttr = (*this)->getAttrOfType<IntegerAttr>("offset")) {
    if (offsetAttr.getInt() < 0)
      return emitOpError() << "expects non-negative offset, but got "
                           << offsetAttr.getInt();
  }

  return success();
}

LogicalResult LoadOp::verify() {
  auto addrTy = dyn_cast<PointerType>(getAddr().getType());
  if (!addrTy)
    return emitOpError() << "expects pointer operand, but got "
                         << getAddr().getType();

  if (addrTy.getPointee() != getResult().getType())
    return emitOpError() << "expects result type " << addrTy.getPointee()
                         << " for pointer operand, but got "
                         << getResult().getType();
  return success();
}

LogicalResult StoreOp::verify() {
  auto addrTy = dyn_cast<PointerType>(getAddr().getType());
  if (!addrTy)
    return emitOpError() << "expects pointer operand, but got "
                         << getAddr().getType();

  if (addrTy.getPointee() != getValue().getType())
    return emitOpError() << "expects stored value type " << addrTy.getPointee()
                         << " for pointer operand, but got "
                         << getValue().getType();
  return success();
}

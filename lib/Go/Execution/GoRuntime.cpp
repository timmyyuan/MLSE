#include "mlse/Go/Execution/GoRuntime.h"

#include "mlir/IR/BuiltinTypes.h"
#include "llvm/ADT/SmallString.h"

namespace mlir::mlse::go::exec {
namespace {

using mlir::mlse::exec::ExecValue;
using mlir::mlse::exec::ExecutionContext;

FailureOr<int64_t> asI64(const ExecValue &value) {
  if (!value.isInteger())
    return failure();
  unsigned width = value.bitWidth;
  if (width == 0 || width >= 64)
    return static_cast<int64_t>(value.getIntegerBits());
  uint64_t masked = value.getIntegerBits() & ((uint64_t{1} << width) - 1);
  uint64_t signBit = uint64_t{1} << (width - 1);
  if ((masked & signBit) == 0)
    return static_cast<int64_t>(masked);
  return static_cast<int64_t>(masked | ~((uint64_t{1} << width) - 1));
}

ExecValue makeI1(bool value) {
  return ExecValue::makeInteger(1, value ? 1 : 0);
}

ExecValue makeI64(int64_t value) {
  return ExecValue::makeInteger(64, static_cast<uint64_t>(value));
}

ExecValue makeSlice(uint64_t ptrRaw, int64_t length, int64_t capacity) {
  std::vector<ExecValue> elements;
  elements.reserve(3);
  elements.push_back(ExecValue::makePointer(ptrRaw));
  elements.push_back(makeI64(length));
  elements.push_back(makeI64(capacity));
  return ExecValue::makeAggregate(std::move(elements));
}

} // namespace

LogicalResult GoRuntimeBundle::call(llvm::StringRef symbol,
                                    llvm::ArrayRef<ExecValue> args,
                                    mlir::TypeRange resultTypes,
                                    ExecutionContext &ctx,
                                    llvm::SmallVectorImpl<ExecValue> &results) {
  (void)resultTypes;

  if (symbol == "runtime.makeslice") {
    if (args.size() != 2)
      return failure();
    FailureOr<int64_t> length = asI64(args[0]);
    FailureOr<int64_t> capacity = asI64(args[1]);
    if (failed(length) || failed(capacity))
      return failure();
    uint64_t ptrRaw = ctx.makeNullPointer();
    if (*capacity > 0)
      ptrRaw = ctx.allocateDeferredSliceBuffer(static_cast<uint64_t>(*capacity));
    results.push_back(makeSlice(ptrRaw, *length, *capacity));
    return success();
  }

  if (symbol == "runtime.growslice") {
    if (args.size() != 5)
      return failure();
    if (!args[0].isPointer())
      return failure();
    FailureOr<int64_t> newLength = asI64(args[1]);
    FailureOr<int64_t> oldCapacity = asI64(args[2]);
    FailureOr<int64_t> num = asI64(args[3]);
    FailureOr<int64_t> elemSize = asI64(args[4]);
    if (failed(newLength) || failed(oldCapacity) || failed(num) || failed(elemSize))
      return failure();
    int64_t oldLength = *newLength - *num;
    int64_t newCapacity = std::max<int64_t>(*newLength, std::max<int64_t>(1, *oldCapacity * 2));
    uint64_t newPtrRaw = ctx.makeNullPointer();
    mlir::mlse::exec::GrowBufferRequest request;
    request.oldRawPointer = args[0].getPointerRaw();
    request.oldLengthElements = static_cast<uint64_t>(std::max<int64_t>(0, oldLength));
    request.oldCapacityElements = static_cast<uint64_t>(std::max<int64_t>(0, *oldCapacity));
    request.newCapacityElements = static_cast<uint64_t>(newCapacity);
    request.elemSize = static_cast<uint64_t>(*elemSize);
    if (failed(ctx.growBuffer(request, newPtrRaw))) {
      return failure();
    }
    results.push_back(makeSlice(newPtrRaw, *newLength, newCapacity));
    return success();
  }

  if (symbol == "runtime.any.box.i64" || symbol == "runtime.any.box.string") {
    if (args.size() != 1)
      return failure();
    results.push_back(ExecValue::makePointer(ctx.allocateBoxedAny(args[0])));
    return success();
  }

  if (symbol == "runtime.eq.string" || symbol == "runtime.neq.string") {
    if (args.size() != 2)
      return failure();
    FailureOr<std::string> lhs = ctx.readGoString(args[0]);
    FailureOr<std::string> rhs = ctx.readGoString(args[1]);
    if (failed(lhs) || failed(rhs))
      return failure();
    bool equal = *lhs == *rhs;
    results.push_back(makeI1(symbol == "runtime.eq.string" ? equal : !equal));
    return success();
  }

  if (symbol == "runtime.fmt.Print" || symbol == "runtime.fmt.Println") {
    if (args.size() != 1)
      return failure();
    FailureOr<std::vector<uint64_t>> boxedArgs = ctx.readPointerSlice(args[0]);
    if (failed(boxedArgs))
      return failure();
    llvm::SmallString<128> line;
    for (size_t i = 0; i < boxedArgs->size(); ++i) {
      FailureOr<std::string> text = ctx.formatBoxedAny((*boxedArgs)[i]);
      if (failed(text))
        return failure();
      if (i != 0)
        line += " ";
      line += *text;
    }
    if (symbol == "runtime.fmt.Println")
      line += "\n";
    ctx.appendStdout(line);
    return success();
  }

  if (symbol == "runtime.panic.index") {
    if (args.size() != 2)
      return failure();
    FailureOr<int64_t> index = asI64(args[0]);
    FailureOr<int64_t> length = asI64(args[1]);
    if (failed(index) || failed(length))
      return failure();
    llvm::SmallString<96> message;
    message += "panic: runtime error: index out of range [";
    message += std::to_string(*index);
    message += "] with length ";
    message += std::to_string(*length);
    ctx.trap(message);
    return success();
  }

  return failure();
}

} // namespace mlir::mlse::go::exec

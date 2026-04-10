#ifndef MLSE_EXECUTION_INTERPRETER_H
#define MLSE_EXECUTION_INTERPRETER_H

#include "llvm/ADT/ArrayRef.h"
#include "llvm/ADT/SmallVector.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/raw_ostream.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/IR/Types.h"
#include "mlir/Support/LogicalResult.h"
#include <cstdint>
#include <string>
#include <vector>

namespace mlir::mlse::exec {

struct ExecValue {
  enum class Kind {
    Undef,
    Integer,
    Pointer,
    Aggregate,
  };

  Kind kind = Kind::Undef;
  unsigned bitWidth = 0;
  uint64_t bits = 0;
  std::vector<ExecValue> elements;

  static ExecValue makeUndef();
  static ExecValue makeInteger(unsigned bitWidth, uint64_t bits);
  static ExecValue makePointer(uint64_t rawPointer);
  static ExecValue makeAggregate(std::vector<ExecValue> elements);

  bool isUndef() const { return kind == Kind::Undef; }
  bool isInteger() const { return kind == Kind::Integer; }
  bool isPointer() const { return kind == Kind::Pointer; }
  bool isAggregate() const { return kind == Kind::Aggregate; }

  uint64_t getIntegerBits() const { return bits; }
  uint64_t getPointerRaw() const { return bits; }
};

struct ExecutionOptions {
  std::string entry;
};

struct ExecutionResult {
  bool ok = false;
  bool trapped = false;
  int exitCode = 0;
  std::string trapMessage;
};

struct ExecutionStreams {
  llvm::raw_ostream &out;
  llvm::raw_ostream &err;
};

struct GrowBufferRequest {
  uint64_t oldRawPointer = 0;
  uint64_t oldLengthElements = 0;
  uint64_t oldCapacityElements = 0;
  uint64_t newCapacityElements = 0;
  uint64_t elemSize = 0;
};

class ExecutionContext;

class RuntimeBundle {
public:
  virtual ~RuntimeBundle() = default;

  virtual LogicalResult call(llvm::StringRef symbol,
                             llvm::ArrayRef<ExecValue> args,
                             mlir::TypeRange resultTypes,
                             ExecutionContext &ctx,
                             llvm::SmallVectorImpl<ExecValue> &results) = 0;
};

class ExecutionContext {
public:
  struct Impl;

  ExecutionContext(ModuleOp module, llvm::raw_ostream &out, llvm::raw_ostream &err);
  ~ExecutionContext();

  ExecutionContext(const ExecutionContext &) = delete;
  ExecutionContext &operator=(const ExecutionContext &) = delete;

  ModuleOp getModule() const;

  ExecValue zeroValueForType(Type type) const;
  ExecValue undefValueForType(Type type) const;

  unsigned getBitWidth(Type type) const;
  uint64_t getTypeSize(Type type) const;

  uint64_t makeNullPointer() const;

  uint64_t allocateBytes(uint64_t size, bool immutable = false);
  uint64_t allocateDeferredSliceBuffer(uint64_t capacityElements);
  uint64_t allocateGlobalBytes(llvm::StringRef bytes);
  uint64_t allocateBoxedAny(const ExecValue &value);

  LogicalResult ensureBufferElementSize(uint64_t rawPointer, uint64_t elemSize);
  LogicalResult growBuffer(const GrowBufferRequest &request,
                           uint64_t &newRawPointer);

  LogicalResult store(uint64_t rawPointer, Type type, const ExecValue &value);
  FailureOr<ExecValue> load(uint64_t rawPointer, Type type);
  FailureOr<uint64_t> gep(uint64_t rawPointer, Type elemType, int64_t index);

  FailureOr<std::string> readGoString(const ExecValue &value);
  FailureOr<std::vector<uint64_t>> readPointerSlice(const ExecValue &sliceValue);
  FailureOr<std::string> formatBoxedAny(uint64_t rawPointer);

  void appendStdout(llvm::StringRef text);
  void appendStderr(llvm::StringRef text);

  void trap(llvm::StringRef message);
  bool isTrapped() const;
  llvm::StringRef getTrapMessage() const;

private:
  Impl *impl = nullptr;
};

LogicalResult runModule(ModuleOp module,
                        const ExecutionOptions &options,
                        RuntimeBundle &runtime,
                        ExecutionStreams streams,
                        ExecutionResult &result);

} // namespace mlir::mlse::exec

#endif // MLSE_EXECUTION_INTERPRETER_H

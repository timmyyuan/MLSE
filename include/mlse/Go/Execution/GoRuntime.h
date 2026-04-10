#ifndef MLSE_GO_EXECUTION_GORUNTIME_H
#define MLSE_GO_EXECUTION_GORUNTIME_H

#include "mlse/Execution/Interpreter.h"

namespace mlir::mlse::go::exec {

class GoRuntimeBundle final : public mlse::exec::RuntimeBundle {
public:
  LogicalResult call(llvm::StringRef symbol,
                     llvm::ArrayRef<mlse::exec::ExecValue> args,
                     mlir::TypeRange resultTypes,
                     mlse::exec::ExecutionContext &ctx,
                     llvm::SmallVectorImpl<mlse::exec::ExecValue> &results) override;
};

} // namespace mlir::mlse::go::exec

#endif // MLSE_GO_EXECUTION_GORUNTIME_H

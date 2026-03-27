#ifndef MLSE_GO_CONVERSION_BOOTSTRAP_LOWERING_H
#define MLSE_GO_CONVERSION_BOOTSTRAP_LOWERING_H

#include "mlir/IR/BuiltinOps.h"
#include "mlir/Support/LogicalResult.h"

namespace mlir::mlse::go::conversion {

LogicalResult lowerGoBuiltins(ModuleOp module);
LogicalResult lowerGoBootstrap(ModuleOp module);

} // namespace mlir::mlse::go::conversion

#endif // MLSE_GO_CONVERSION_BOOTSTRAP_LOWERING_H

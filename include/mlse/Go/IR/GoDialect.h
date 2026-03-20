#ifndef MLSE_GO_IR_GODIALECT_H
#define MLSE_GO_IR_GODIALECT_H

#include "mlir/Bytecode/BytecodeOpInterface.h"
#include "mlse/Go/IR/GoTypes.h"
#include "mlir/IR/Dialect.h"
#include "mlir/IR/OpDefinition.h"
#include "mlir/IR/OpImplementation.h"
#include "mlir/Interfaces/SideEffectInterfaces.h"

#include "mlse/Go/IR/GoDialect.h.inc"

#define GET_OP_CLASSES
#include "mlse/Go/IR/GoOps.h.inc"

#endif // MLSE_GO_IR_GODIALECT_H

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

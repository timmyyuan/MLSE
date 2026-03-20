set(LLVM_TARGET_DEFINITIONS GoBase.td)
mlir_tablegen(GoDialect.h.inc -gen-dialect-decls -dialect=go)
mlir_tablegen(GoDialect.cpp.inc -gen-dialect-defs -dialect=go)
add_public_tablegen_target(MLSEGoDialectIncGen)

set(LLVM_TARGET_DEFINITIONS GoTypes.td)
mlir_tablegen(GoTypes.h.inc -gen-typedef-decls -typedefs-dialect=go)
mlir_tablegen(GoTypes.cpp.inc -gen-typedef-defs -typedefs-dialect=go)
add_public_tablegen_target(MLSEGoTypesIncGen)

set(LLVM_TARGET_DEFINITIONS GoOps.td)
mlir_tablegen(GoOps.h.inc -gen-op-decls)
mlir_tablegen(GoOps.cpp.inc -gen-op-defs)
add_public_tablegen_target(MLSEGoOpsIncGen)

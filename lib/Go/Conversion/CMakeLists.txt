add_mlir_library(MLSEGoBootstrapLowering
  BootstrapLowering.cpp

  LINK_LIBS PUBLIC
  MLSEGoDialect
  MLIRArithDialect
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRFuncTransforms
  MLIRIR
  MLIRLLVMDialect
  MLIRSCFDialect
  MLIRSCFTransforms
  MLIRSupport
  MLIRTransforms
)

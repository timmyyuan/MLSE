add_mlir_library(MLSEGoExecution
  GoRuntime.cpp

  LINK_LIBS PUBLIC
  MLSEExecutionCore
  MLIRIR
  MLIRLLVMDialect
)

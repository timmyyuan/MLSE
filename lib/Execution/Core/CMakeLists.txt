add_mlir_library(MLSEExecutionCore
  Interpreter.cpp

  LINK_LIBS PUBLIC
  MLIRIR
  MLIRLLVMDialect
  MLIRParser
  MLIRSupport
)

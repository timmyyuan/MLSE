add_mlir_tool(mlse-run
  mlse-run.cpp
)

target_link_libraries(mlse-run PRIVATE
  MLSEExecutionCore
  MLSEGoExecution
  MLIRIR
  MLIRLLVMDialect
  MLIRParser
  MLIRSupport
)

mlir_check_all_link_libraries(mlse-run)

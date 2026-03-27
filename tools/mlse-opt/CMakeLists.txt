add_mlir_tool(mlse-opt
  mlse-opt.cpp
)

target_link_libraries(mlse-opt PRIVATE
  MLSEGoBootstrapLowering
  MLSEGoDialect
  MLIRArithDialect
  MLIRControlFlowDialect
  MLIRFuncDialect
  MLIRIR
  MLIRParser
  MLIRSCFDialect
  MLIRSupport
)

mlir_check_all_link_libraries(mlse-opt)

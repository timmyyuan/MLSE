add_mlir_dialect_library(MLSEGoDialect
  GoDialect.cpp

  DEPENDS
  MLSEGoDialectIncGen
  MLSEGoOpsIncGen
  MLSEGoTypesIncGen

  LINK_LIBS PUBLIC
  MLIRIR
)

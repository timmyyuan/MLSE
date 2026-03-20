#include "mlse/Go/IR/GoDialect.h"
#include "mlir/Dialect/Arith/IR/Arith.h"
#include "mlir/Dialect/ControlFlow/IR/ControlFlowOps.h"
#include "mlir/Dialect/Func/IR/FuncOps.h"
#include "mlir/Dialect/SCF/IR/SCF.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/IR/DialectRegistry.h"
#include "mlir/IR/MLIRContext.h"
#include "mlir/Parser/Parser.h"
#include "llvm/Support/CommandLine.h"
#include "llvm/Support/FileSystem.h"
#include "llvm/Support/InitLLVM.h"
#include "llvm/Support/MemoryBuffer.h"
#include "llvm/Support/SourceMgr.h"
#include "llvm/Support/raw_ostream.h"

int main(int argc, char **argv) {
  llvm::InitLLVM initLLVM(argc, argv);
  llvm::cl::opt<std::string> inputFilename(
      llvm::cl::Positional,
      llvm::cl::desc("<input.mlir>"),
      llvm::cl::Required);
  llvm::cl::ParseCommandLineOptions(argc, argv, "MLSE Go dialect bootstrap\n");

  mlir::DialectRegistry registry;
  registry.insert<mlir::arith::ArithDialect,
                  mlir::cf::ControlFlowDialect,
                  mlir::func::FuncDialect,
                  mlir::go::GoDialect,
                  mlir::scf::SCFDialect>();
  mlir::MLIRContext context(registry);
  context.loadAllAvailableDialects();

  auto inputFile = llvm::MemoryBuffer::getFileOrSTDIN(inputFilename);
  if (!inputFile) {
    llvm::errs() << "mlse-opt: failed to open " << inputFilename << "\n";
    return 1;
  }

  llvm::SourceMgr sourceMgr;
  sourceMgr.AddNewSourceBuffer(std::move(*inputFile), llvm::SMLoc());
  auto module = mlir::parseSourceFile<mlir::ModuleOp>(sourceMgr, &context);
  if (!module) {
    return 1;
  }

  module->print(llvm::outs());
  return 0;
}

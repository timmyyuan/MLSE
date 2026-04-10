#include "mlse/Execution/Interpreter.h"
#include "mlse/Go/Execution/GoRuntime.h"
#include "mlir/Dialect/LLVMIR/LLVMDialect.h"
#include "mlir/IR/BuiltinOps.h"
#include "mlir/IR/DialectRegistry.h"
#include "mlir/IR/MLIRContext.h"
#include "mlir/Parser/Parser.h"
#include "llvm/ADT/StringRef.h"
#include "llvm/Support/CommandLine.h"
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
  llvm::cl::opt<std::string> entry(
      "entry",
      llvm::cl::desc("Entry llvm.func symbol (defaults to main.main, main, or the first public zero-arg function)"),
      llvm::cl::init(""));
  llvm::cl::ParseCommandLineOptions(argc, argv, "MLSE LLVM-dialect runner\n");

  if (!llvm::StringRef(inputFilename).ends_with(".mlir")) {
    llvm::errs() << "mlse-run: only LLVM-dialect MLIR inputs are supported in the current MVP\n";
    return 1;
  }

  mlir::DialectRegistry registry;
  registry.insert<mlir::LLVM::LLVMDialect>();
  mlir::MLIRContext context(registry);
  context.loadAllAvailableDialects();

  auto inputFile = llvm::MemoryBuffer::getFileOrSTDIN(inputFilename);
  if (!inputFile) {
    llvm::errs() << "mlse-run: failed to open " << inputFilename << "\n";
    return 1;
  }

  llvm::SourceMgr sourceMgr;
  sourceMgr.AddNewSourceBuffer(std::move(*inputFile), llvm::SMLoc());
  auto module = mlir::parseSourceFile<mlir::ModuleOp>(sourceMgr, &context);
  if (!module) {
    return 1;
  }

  mlir::mlse::exec::ExecutionOptions options;
  options.entry = entry;
  mlir::mlse::exec::ExecutionResult result;
  mlir::mlse::exec::ExecutionStreams streams{llvm::outs(), llvm::errs()};
  mlir::mlse::go::exec::GoRuntimeBundle runtime;
  if (mlir::failed(mlir::mlse::exec::runModule(*module, options, runtime, streams,
                                               result))) {
    if (!result.trapMessage.empty())
      llvm::errs() << result.trapMessage << "\n";
    return result.exitCode == 0 ? 1 : result.exitCode;
  }
  if (result.trapped && !result.trapMessage.empty())
    llvm::errs() << result.trapMessage << "\n";
  return result.exitCode;
}

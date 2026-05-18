package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/yuanting/MLSE/internal/symbolicdiff"
)

func main() {
	opts, pipelineOpts, runPipeline := parseFlags()
	result, err := symbolicdiff.PrepareCase(context.Background(), opts)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-diff: %v\n", err)
		os.Exit(1)
	}
	if !runPipeline {
		printJSON(result)
		return
	}

	pipelineOpts.CasesRoot = result.CasesRoot
	pipelineOpts.CaseName = result.CaseName
	if pipelineOpts.EmitDir == "" {
		pipelineOpts.EmitDir = filepath.Join(opts.EmitDir, "probe")
	}
	if pipelineOpts.RunKLEE && pipelineOpts.ExpectStatus == "" {
		pipelineOpts.ExpectStatus = "ok"
	}
	fmt.Fprintf(os.Stderr, "mlse-diff: generated case %s in %s\n", result.CaseName, result.CaseDir)
	if err := symbolicdiff.RunPipeline(context.Background(), pipelineOpts, os.Stdout, os.Stderr); err != nil {
		fmt.Fprintf(os.Stderr, "mlse-diff: %v\n", err)
		os.Exit(1)
	}
}

func parseFlags() (symbolicdiff.PrepareOptions, symbolicdiff.PipelineOptions, bool) {
	var prepare symbolicdiff.PrepareOptions
	var pipeline symbolicdiff.PipelineOptions
	runPipeline := flag.Bool("run-pipeline", true, "run scripts/mlse-diff-go-pipeline-probe.py after generating the case")
	flag.StringVar(&prepare.File, "file", "", "changed Go file path inside the target repo")
	flag.StringVar(&prepare.Function, "function", "", "entry function name used on both sides")
	flag.StringVar(&prepare.OldFunction, "old-function", "", "entry function name in the old commit")
	flag.StringVar(&prepare.NewFunction, "new-function", "", "entry function name in the new commit")
	flag.StringVar(&prepare.EmitDir, "emit", filepath.Join("artifacts", "mlse-diff"), "artifact directory")
	flag.StringVar(&prepare.CaseName, "case", "", "generated symbolic-diff case name")
	flag.StringVar(&prepare.ExpectedStatus, "expected", "equivalent", "expected KLEE result: equivalent or counterexample")
	flag.IntVar(&prepare.SliceLength, "slice-len", 1, "concrete slice length for the current []int KLEE model")
	flag.StringVar(&pipeline.Python, "python", "python3", "Python executable for the existing probe script")
	flag.StringVar(&pipeline.PipelineScript, "pipeline", "", "path to scripts/mlse-diff-go-pipeline-probe.py")
	flag.BoolVar(&pipeline.RunKLEE, "run-klee", false, "ask the probe to run KLEE after producing old/new bitcode")
	flag.StringVar(&pipeline.ExpectStatus, "expect-pipeline-status", "", "optional expected probe summary status, usually ok")
	registerToolFlags(&pipeline.Tools)
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [flags] <repo> <old-commit> <new-commit>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Generate a function-level symbolic-diff case from a Git diff and run the existing Go/KLEE probe.")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() != 3 {
		flag.Usage()
		os.Exit(2)
	}
	prepare.Repo = flag.Arg(0)
	prepare.OldCommit = flag.Arg(1)
	prepare.NewCommit = flag.Arg(2)
	return prepare, pipeline, *runPipeline
}

func registerToolFlags(tools *symbolicdiff.ToolPaths) {
	flag.StringVar(&tools.MLSEGo, "mlse-go-bin", "", "path or name for mlse-go")
	flag.StringVar(&tools.MLSEOpt, "mlse-opt-bin", "", "path or name for mlse-opt")
	flag.StringVar(&tools.MLIROpt, "mlir-opt-bin", "", "path or name for mlir-opt")
	flag.StringVar(&tools.MLIRTranslate, "mlir-translate-bin", "", "path or name for mlir-translate")
	flag.StringVar(&tools.LLVMAs, "llvm-as-bin", "", "path or name for llvm-as")
	flag.StringVar(&tools.LLVMLink, "llvm-link-bin", "", "path or name for llvm-link")
	flag.StringVar(&tools.Clang, "clang-bin", "", "path or name for clang")
	flag.StringVar(&tools.KLEE, "klee-bin", "", "path or name for klee")
}

func printJSON(value any) {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-diff: marshal result: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(data))
}

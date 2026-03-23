package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yuanting/MLSE/internal/goirllvmexp"
)

type cliConfig struct {
	emit       string
	sliceModel goirllvmexp.SliceModel
	inputPath  string
}

func newFlagSet(stderr io.Writer) (*flag.FlagSet, *string, *string) {
	fs := flag.NewFlagSet("mlse-goir-llvm-exp", flag.ContinueOnError)
	fs.SetOutput(stderr)
	emit := fs.String("emit", "llvm", "Output kind: llvm or llvm-dialect")
	sliceModel := fs.String("slice-model", string(goirllvmexp.SliceModelMin), "Slice runtime model: min or cap")
	fs.Usage = func() {
		fmt.Fprintf(fs.Output(), "Usage: %s [-emit=llvm|llvm-dialect] [-slice-model=min|cap] [input.goir]\n", fs.Name())
		fmt.Fprintln(fs.Output(), "Translate a tiny experimental MLSE GoIR subset to LLVM dialect MLIR or LLVM IR.")
	}
	return fs, emit, sliceModel
}

func parseCLIArgs(args []string, stderr io.Writer) (cliConfig, error) {
	fs, emit, sliceModel := newFlagSet(stderr)
	if err := fs.Parse(args); err != nil {
		return cliConfig{}, err
	}
	if fs.NArg() > 1 {
		fs.Usage()
		return cliConfig{}, fmt.Errorf("expected at most one input file")
	}

	model, err := goirllvmexp.ParseSliceModel(*sliceModel)
	if err != nil {
		return cliConfig{}, err
	}

	cfg := cliConfig{
		emit:       *emit,
		sliceModel: model,
	}
	if fs.NArg() == 1 {
		cfg.inputPath = fs.Arg(0)
	}
	return cfg, nil
}

func run(args []string, stdin io.Reader, stdout io.Writer, stderr io.Writer) int {
	cfg, err := parseCLIArgs(args, stderr)
	if err != nil {
		if err == flag.ErrHelp {
			return 2
		}
		fmt.Fprintf(stderr, "mlse-goir-llvm-exp: %v\n", err)
		return 2
	}

	var data []byte
	if cfg.inputPath != "" {
		data, err = os.ReadFile(cfg.inputPath)
	} else {
		data, err = io.ReadAll(stdin)
	}
	if err != nil {
		fmt.Fprintf(stderr, "mlse-goir-llvm-exp: %v\n", err)
		return 1
	}

	opts := goirllvmexp.LoweringOptions{SliceModel: cfg.sliceModel}
	var out string
	switch cfg.emit {
	case "llvm":
		out, err = goirllvmexp.TranslateModuleWithOptions(string(data), opts)
	case "llvm-dialect":
		out, err = goirllvmexp.LowerToLLVMDialectModuleWithOptions(string(data), opts)
	default:
		fmt.Fprintf(stderr, "mlse-goir-llvm-exp: unsupported -emit value %q\n", cfg.emit)
		return 2
	}
	if err != nil {
		fmt.Fprintf(stderr, "mlse-goir-llvm-exp: %v\n", err)
		return 1
	}
	fmt.Fprint(stdout, out)
	return 0
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

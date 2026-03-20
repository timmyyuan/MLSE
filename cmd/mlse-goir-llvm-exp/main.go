package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/yuanting/MLSE/internal/goirllvmexp"
)

func main() {
	emit := flag.String("emit", "llvm", "Output kind: llvm or llvm-dialect")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-emit=llvm|llvm-dialect] [input.goir]\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Translate a tiny experimental MLSE GoIR subset to LLVM dialect MLIR or LLVM IR.")
	}
	flag.Parse()
	if flag.NArg() > 1 {
		flag.Usage()
		os.Exit(2)
	}

	var (
		data []byte
		err  error
	)
	if flag.NArg() == 1 {
		data, err = os.ReadFile(flag.Arg(0))
	} else {
		data, err = io.ReadAll(os.Stdin)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-goir-llvm-exp: %v\n", err)
		os.Exit(1)
	}

	var out string
	switch *emit {
	case "llvm":
		out, err = goirllvmexp.TranslateModule(string(data))
	case "llvm-dialect":
		out, err = goirllvmexp.LowerToLLVMDialectModule(string(data))
	default:
		fmt.Fprintf(os.Stderr, "mlse-goir-llvm-exp: unsupported -emit value %q\n", *emit)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-goir-llvm-exp: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuanting/MLSE/internal/gofrontend"
)

func main() {
	emitMode := flag.String("emit", "formal", "output mode: formal or goir-like")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s [-emit=formal|goir-like] <input.go>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Emit a formal go dialect module or the legacy GoIR-like prototype from Go source.")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	var (
		out string
		err error
	)
	switch *emitMode {
	case "formal":
		out, err = gofrontend.CompileFileFormal(flag.Arg(0))
	case "goir-like":
		out, err = gofrontend.CompileFileGoIRLike(flag.Arg(0))
	default:
		fmt.Fprintf(os.Stderr, "mlse-go: unsupported -emit value %q\n", *emitMode)
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-go: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

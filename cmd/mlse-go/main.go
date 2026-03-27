package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/yuanting/MLSE/internal/gofrontend"
)

func main() {
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <input.go>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Emit the formal go dialect bridge from Go source.")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	out, err := gofrontend.CompileFile(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-go: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)
}

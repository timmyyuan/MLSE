package main

import (
	"flag"
	"fmt"
	"os"

	"golang.org/x/tools/go/packages"
	"golang.org/x/tools/go/ssa"
	"golang.org/x/tools/go/ssa/ssautil"
)

func main() {
	pkgPattern := flag.String("package", "", "Go package import path to load")
	funcName := flag.String("func", "Target", "Top-level function name to dump")
	flag.Parse()

	if *pkgPattern == "" {
		fmt.Fprintln(os.Stderr, "missing required -package")
		os.Exit(2)
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
		Env: os.Environ(),
	}
	pkgs, err := packages.Load(cfg, *pkgPattern)
	if err != nil {
		fmt.Fprintf(os.Stderr, "packages.Load(%q): %v\n", *pkgPattern, err)
		os.Exit(1)
	}
	if packages.PrintErrors(pkgs) > 0 {
		os.Exit(1)
	}

	prog, ssaPkgs := ssautil.AllPackages(pkgs, ssa.SanityCheckFunctions)
	prog.Build()

	for _, ssaPkg := range ssaPkgs {
		if ssaPkg == nil || ssaPkg.Pkg == nil || ssaPkg.Pkg.Name() != "main" {
			continue
		}
		member, ok := ssaPkg.Members[*funcName]
		if !ok {
			continue
		}
		fn, ok := member.(*ssa.Function)
		if !ok {
			fmt.Fprintf(os.Stderr, "%s.%s is not a function\n", ssaPkg.Pkg.Name(), *funcName)
			os.Exit(1)
		}
		fmt.Fprintf(os.Stdout, "// package: %s\n", ssaPkg.Pkg.Path())
		fmt.Fprintf(os.Stdout, "// function: %s.%s\n\n", ssaPkg.Pkg.Name(), *funcName)
		fn.WriteTo(os.Stdout)
		return
	}

	fmt.Fprintf(os.Stdout, "// main.%s not found in %s\n", *funcName, *pkgPattern)
}

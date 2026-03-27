package gofrontend

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
)

// CompileFile keeps the default entry aligned with the formal bridge.
func CompileFile(path string) (string, error) {
	return CompileFileFormal(path)
}

// CompileFileFormal parses one Go file and prints one formal MLIR module.
func CompileFileFormal(path string) (string, error) {
	file, funcs, typed, err := parseModule(path)
	if err != nil {
		return "", err
	}

	module := newFormalModuleContext(file, funcs, typed)
	var buf bytes.Buffer
	buf.WriteString("module {\n")
	for _, fn := range funcs {
		buf.WriteString(emitFormalFunc(fn, module))
	}
	buf.WriteString(module.emitGeneratedFuncs())
	buf.WriteString(module.emitExternDecls())
	buf.WriteString("}\n")
	return buf.String(), nil
}

func parseModule(path string) (*ast.File, []*ast.FuncDecl, *formalTypeContext, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}

	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name.Name < funcs[j].Name.Name })
	return file, funcs, buildFormalTypeContext(path, fset, file), nil
}

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
	if typed != nil {
		typed.goarch = module.goarch
		typed.sizes = buildFormalSizes(module.goarch)
	}
	var body bytes.Buffer
	for _, fn := range funcs {
		body.WriteString(emitFormalFunc(fn, module))
	}
	body.WriteString(emitFormalGeneratedFuncs(module))
	body.WriteString(emitFormalExternDecls(module))

	var buf bytes.Buffer
	if attrs := module.emitScopeTableAttr(); attrs != "" {
		buf.WriteString("module attributes {" + attrs + "} {\n")
	} else {
		buf.WriteString("module {\n")
	}
	buf.WriteString(body.String())
	buf.WriteString("}\n")
	return buf.String(), nil
}

func parseModule(path string) (*ast.File, []*ast.FuncDecl, *formalTypeContext, error) {
	if file, typed, err := loadFormalFileWithPackages(path); err == nil && file != nil {
		funcs := collectFormalFuncs(file)
		return file, funcs, typed, nil
	}

	file, funcs, typed, err := parseModuleFallback(path)
	if err != nil {
		return nil, nil, nil, err
	}
	return file, funcs, typed, nil
}

func parseModuleFallback(path string) (*ast.File, []*ast.FuncDecl, *formalTypeContext, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, nil, nil, err
	}

	funcs := collectFormalFuncs(file)
	return file, funcs, buildFormalTypeContext(path, fset, file), nil
}

func collectFormalFuncs(file *ast.File) []*ast.FuncDecl {
	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name.Name < funcs[j].Name.Name })
	return funcs
}

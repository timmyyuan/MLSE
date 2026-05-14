package gofrontend

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strings"
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
	packageFiles := collectFormalFallbackPackageFiles(path, fset, file)
	return file, funcs, buildFormalTypeContextFromFiles(path, fset, file, packageFiles), nil
}

func collectFormalFuncs(file *ast.File) []*ast.FuncDecl {
	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool {
		if funcs[i].Name.Name == funcs[j].Name.Name {
			return funcs[i].Pos() < funcs[j].Pos()
		}
		return funcs[i].Name.Name < funcs[j].Name.Name
	})
	return funcs
}

func collectFormalFallbackPackageFiles(path string, fset *token.FileSet, mainFile *ast.File) []*ast.File {
	files := []*ast.File{mainFile}
	if mainFile == nil || mainFile.Name == nil {
		return files
	}

	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return files
	}
	mainAbs, err := filepath.Abs(path)
	if err != nil {
		mainAbs = filepath.Clean(path)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if filepath.Ext(name) != ".go" || strings.HasSuffix(name, "_test.go") {
			continue
		}
		candidate := filepath.Join(dir, name)
		candidateAbs, err := filepath.Abs(candidate)
		if err != nil {
			candidateAbs = filepath.Clean(candidate)
		}
		if sameFormalFilePath(candidateAbs, mainAbs) {
			continue
		}
		peer, err := parser.ParseFile(fset, candidateAbs, nil, parser.ParseComments)
		if err != nil || peer == nil || peer.Name == nil {
			continue
		}
		if peer.Name.Name != mainFile.Name.Name {
			continue
		}
		files = append(files, peer)
	}
	return files
}

package gofrontend

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
)

type env struct {
	locals     map[string]string
	localTypes map[string]string
	order      []string
	tempID     int
}

func newEnv() *env {
	return &env{locals: make(map[string]string), localTypes: make(map[string]string)}
}

func (e *env) define(name string) string {
	return e.defineTyped(name, "!go.any")
}

func (e *env) defineTyped(name string, ty string) string {
	if existing, ok := e.locals[name]; ok {
		if ty != "" {
			e.localTypes[name] = ty
		}
		return existing
	}
	e.locals[name] = "%" + sanitizeName(name)
	e.localTypes[name] = ty
	e.order = append(e.order, name)
	return e.locals[name]
}

func (e *env) use(name string) string {
	if v, ok := e.locals[name]; ok {
		return v
	}
	return e.define(name)
}

func (e *env) typeOf(name string) string {
	if ty, ok := e.localTypes[name]; ok && ty != "" {
		return ty
	}
	return "!go.any"
}

func (e *env) temp(prefix string) string {
	e.tempID++
	return fmt.Sprintf("%%%s%d", sanitizeName(prefix), e.tempID)
}

func CompileFile(path string) (string, error) {
	return CompileFileFormal(path)
}

func CompileFileFormal(path string) (string, error) {
	funcs, err := parseFuncs(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString("module {\n")
	for _, fn := range funcs {
		buf.WriteString(emitFormalFunc(fn))
	}
	buf.WriteString("}\n")
	return buf.String(), nil
}

func CompileFileGoIRLike(path string) (string, error) {
	funcs, err := parseFuncs(path)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	buf.WriteString("module {\n")
	for _, fn := range funcs {
		rendered := emitFunc(fn)
		buf.WriteString(rendered)
	}
	buf.WriteString("}\n")
	return buf.String(), nil
}

func parseFuncs(path string) ([]*ast.FuncDecl, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name.Name < funcs[j].Name.Name })
	return funcs, nil
}

func emitFunc(fn *ast.FuncDecl) string {
	env := newEnv()
	params := emitParams(fn.Type.Params, env)
	results := emitResultTypes(fn.Type.Results)

	var buf bytes.Buffer
	if len(results) == 0 {
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", ")))
	} else if len(results) == 1 {
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) -> %s {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", "), results[0]))
	} else {
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) -> (%s) {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", "), strings.Join(results, ", ")))
	}

	if fn.Body == nil {
		buf.WriteString("    mlse.unsupported_stmt \"missing_body\"\n")
		buf.WriteString("  }\n")
		return buf.String()
	}

	for _, stmt := range fn.Body.List {
		buf.WriteString(emitStmt(stmt, env))
	}
	buf.WriteString("  }\n")
	return buf.String()
}

func emitParams(fields *ast.FieldList, env *env) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := goTypeToMLIR(field.Type)
		if len(field.Names) == 0 {
			name := env.temp("arg")
			out = append(out, fmt.Sprintf("%s: %s", name, ty))
			continue
		}
		for _, name := range field.Names {
			ssa := env.defineTyped(name.Name, ty)
			out = append(out, fmt.Sprintf("%s: %s", ssa, ty))
		}
	}
	return out
}

func emitResultTypes(fields *ast.FieldList) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := goTypeToMLIR(field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, ty)
		}
	}
	return out
}

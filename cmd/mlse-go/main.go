package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"sort"
	"strings"
)

type env struct {
	locals map[string]string
	order  []string
}

func newEnv() *env {
	return &env{locals: make(map[string]string)}
}

func (e *env) define(name string) string {
	if existing, ok := e.locals[name]; ok {
		return existing
	}
	e.locals[name] = "%" + name
	e.order = append(e.order, name)
	return e.locals[name]
}

func (e *env) use(name string) (string, bool) {
	v, ok := e.locals[name]
	return v, ok
}

func main() {
	input := flag.String("", "", "")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <input.go>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Emit a tiny MLIR-like module from a tiny Go subset.")
	}
	flag.Parse()
	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	path := flag.Arg(0)
	out, err := compileFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "mlse-go: %v\n", err)
		os.Exit(1)
	}
	fmt.Print(out)

	_ = input
}

func compileFile(path string) (string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, 0)
	if err != nil {
		return "", err
	}

	var funcs []*ast.FuncDecl
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if ok {
			funcs = append(funcs, fn)
		}
	}
	sort.Slice(funcs, func(i, j int) bool { return funcs[i].Name.Name < funcs[j].Name.Name })

	var buf bytes.Buffer
	buf.WriteString("module {\n")
	for _, fn := range funcs {
		rendered, err := emitFunc(fn)
		if err != nil {
			return "", fmt.Errorf("function %s: %w", fn.Name.Name, err)
		}
		buf.WriteString(rendered)
	}
	buf.WriteString("}\n")
	return buf.String(), nil
}

func emitFunc(fn *ast.FuncDecl) (string, error) {
	if fn.Body == nil {
		return "", fmt.Errorf("function body is required")
	}
	env := newEnv()
	params, err := emitParams(fn.Type.Params, env)
	if err != nil {
		return "", err
	}
	retType, err := emitResultType(fn.Type.Results)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if retType == "" {
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) {\n", fn.Name.Name, strings.Join(params, ", ")))
	} else {
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) -> %s {\n", fn.Name.Name, strings.Join(params, ", "), retType))
	}

	for _, stmt := range fn.Body.List {
		line, err := emitStmt(stmt, env)
		if err != nil {
			return "", err
		}
		buf.WriteString(line)
	}
	buf.WriteString("  }\n")
	return buf.String(), nil
}

func emitParams(fields *ast.FieldList, env *env) ([]string, error) {
	if fields == nil || len(fields.List) == 0 {
		return nil, nil
	}
	var out []string
	for _, field := range fields.List {
		ty, err := goTypeToMLIR(field.Type)
		if err != nil {
			return nil, err
		}
		for _, name := range field.Names {
			ssa := env.define(name.Name)
			out = append(out, fmt.Sprintf("%s: %s", ssa, ty))
		}
	}
	return out, nil
}

func emitResultType(fields *ast.FieldList) (string, error) {
	if fields == nil || len(fields.List) == 0 {
		return "", nil
	}
	if len(fields.List) != 1 || len(fields.List[0].Names) > 1 {
		return "", fmt.Errorf("only a single unnamed result is supported")
	}
	return goTypeToMLIR(fields.List[0].Type)
}

func emitStmt(stmt ast.Stmt, env *env) (string, error) {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		if s.Tok != token.DEFINE {
			return "", fmt.Errorf("only := assignments are supported")
		}
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return "", fmt.Errorf("only single-value assignments are supported")
		}
		ident, ok := s.Lhs[0].(*ast.Ident)
		if !ok {
			return "", fmt.Errorf("assignment lhs must be an identifier")
		}
		value, ty, err := emitExpr(s.Rhs[0], env)
		if err != nil {
			return "", err
		}
		name := env.define(ident.Name)
		return fmt.Sprintf("    %s = %s : %s\n", name, value, ty), nil
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return "", fmt.Errorf("only single-value return is supported")
		}
		value, ty, err := emitExpr(s.Results[0], env)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("    return %s : %s\n", value, ty), nil
	default:
		return "", fmt.Errorf("unsupported statement %T", stmt)
	}
}

func emitExpr(expr ast.Expr, env *env) (value string, ty string, err error) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		if e.Kind != token.INT {
			return "", "", fmt.Errorf("only integer literals are supported")
		}
		return e.Value, "i32", nil
	case *ast.Ident:
		v, ok := env.use(e.Name)
		if !ok {
			return "", "", fmt.Errorf("undefined identifier %q", e.Name)
		}
		return v, "i32", nil
	case *ast.BinaryExpr:
		lhs, lhsTy, err := emitExpr(e.X, env)
		if err != nil {
			return "", "", err
		}
		rhs, rhsTy, err := emitExpr(e.Y, env)
		if err != nil {
			return "", "", err
		}
		if lhsTy != rhsTy {
			return "", "", fmt.Errorf("type mismatch: %s vs %s", lhsTy, rhsTy)
		}
		op, err := binaryOpToMLIR(e.Op)
		if err != nil {
			return "", "", err
		}
		return fmt.Sprintf("%s %s, %s", op, lhs, rhs), lhsTy, nil
	default:
		return "", "", fmt.Errorf("unsupported expression %T", expr)
	}
}

func binaryOpToMLIR(op token.Token) (string, error) {
	switch op {
	case token.ADD:
		return "arith.addi", nil
	case token.SUB:
		return "arith.subi", nil
	case token.MUL:
		return "arith.muli", nil
	case token.QUO:
		return "arith.divsi", nil
	default:
		return "", fmt.Errorf("unsupported binary operator %s", op)
	}
}

func goTypeToMLIR(expr ast.Expr) (string, error) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", fmt.Errorf("unsupported type %T", expr)
	}
	switch ident.Name {
	case "int":
		return "i32", nil
	default:
		return "", fmt.Errorf("unsupported Go type %q", ident.Name)
	}
}

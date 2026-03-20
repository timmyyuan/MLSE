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
	"strconv"
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

func main() {
	input := flag.String("", "", "")
	flag.Usage = func() {
		fmt.Fprintf(flag.CommandLine.Output(), "Usage: %s <input.go>\n", os.Args[0])
		fmt.Fprintln(flag.CommandLine.Output(), "Emit a tiny MLIR-like module from Go source.")
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
	file, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
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
		rendered := emitFunc(fn)
		buf.WriteString(rendered)
	}
	buf.WriteString("}\n")
	return buf.String(), nil
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

func emitStmt(stmt ast.Stmt, env *env) string {
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return emitAssignStmt(s, env)
	case *ast.ReturnStmt:
		return emitReturnStmt(s, env)
	case *ast.ExprStmt:
		value, ty, prelude := emitExpr(s.X, env)
		return prelude + fmt.Sprintf("    mlse.expr %s : %s\n", value, ty)
	case *ast.DeclStmt:
		return emitDeclStmt(s, env)
	case *ast.IfStmt:
		return emitIfStmt(s, env)
	case *ast.ForStmt:
		return emitForStmt(s, env)
	case *ast.RangeStmt:
		return emitRangeStmt(s, env)
	case *ast.BranchStmt:
		return fmt.Sprintf("    mlse.branch %q\n", s.Tok.String())
	case *ast.IncDecStmt:
		value, ty, prelude := emitExpr(s.X, env)
		return prelude + fmt.Sprintf("    mlse.%s %s : %s\n", strings.ToLower(s.Tok.String()), value, ty)
	case *ast.SwitchStmt:
		return emitSwitchStmt(s, env)
	case *ast.GoStmt:
		value, ty, prelude := emitExpr(s.Call, env)
		return prelude + fmt.Sprintf("    mlse.go %s : %s\n", value, ty)
	case *ast.DeferStmt:
		value, ty, prelude := emitExpr(s.Call, env)
		return prelude + fmt.Sprintf("    mlse.defer %s : %s\n", value, ty)
	case *ast.LabeledStmt:
		return fmt.Sprintf("    mlse.label @%s\n", sanitizeName(s.Label.Name)) + emitStmt(s.Stmt, env)
	case *ast.EmptyStmt:
		return ""
	default:
		return fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(stmt))
	}
}

func emitAssignStmt(s *ast.AssignStmt, env *env) string {
	var buf strings.Builder
	for i := 0; i < len(s.Lhs) && i < len(s.Rhs); i++ {
		value, ty, prelude := emitExpr(s.Rhs[i], env)
		buf.WriteString(prelude)
		ident, ok := s.Lhs[i].(*ast.Ident)
		if !ok {
			buf.WriteString(fmt.Sprintf("    mlse.assign_target %q = %s : %s\n", shortNodeName(s.Lhs[i]), value, ty))
			continue
		}
		name := env.defineTyped(ident.Name, ty)
		if s.Tok == token.ASSIGN {
			name = env.use(ident.Name)
		}
		buf.WriteString(fmt.Sprintf("    %s = %s : %s\n", name, value, ty))
	}
	if len(s.Lhs) != len(s.Rhs) {
		buf.WriteString(fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(s)))
	}
	return buf.String()
}

func emitReturnStmt(s *ast.ReturnStmt, env *env) string {
	if len(s.Results) == 0 {
		return "    return\n"
	}
	var values []string
	var tys []string
	var prelude strings.Builder
	for _, result := range s.Results {
		value, ty, inner := emitExpr(result, env)
		prelude.WriteString(inner)
		values = append(values, value)
		tys = append(tys, ty)
	}
	return prelude.String() + fmt.Sprintf("    return %s : %s\n", strings.Join(values, ", "), strings.Join(tys, ", "))
}

func emitDeclStmt(s *ast.DeclStmt, env *env) string {
	gen, ok := s.Decl.(*ast.GenDecl)
	if !ok {
		return fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(s))
	}
	var buf strings.Builder
	for _, spec := range gen.Specs {
		vs, ok := spec.(*ast.ValueSpec)
		if !ok {
			buf.WriteString(fmt.Sprintf("    mlse.unsupported_decl %q\n", shortNodeName(spec)))
			continue
		}
		for i, name := range vs.Names {
			ty := "!go.any"
			if vs.Type != nil {
				ty = goTypeToMLIR(vs.Type)
			}
			ssa := env.defineTyped(name.Name, ty)
			if i < len(vs.Values) {
				value, valueTy, prelude := emitExpr(vs.Values[i], env)
				buf.WriteString(prelude)
				if ty == "!go.any" {
					ty = valueTy
					env.defineTyped(name.Name, ty)
				}
				buf.WriteString(fmt.Sprintf("    %s = %s : %s\n", ssa, value, ty))
			} else {
				buf.WriteString(fmt.Sprintf("    %s = mlse.zero : %s\n", ssa, ty))
			}
		}
	}
	return buf.String()
}

func emitIfStmt(s *ast.IfStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	cond, ty, prelude := emitExpr(s.Cond, env)
	buf.WriteString(prelude)
	buf.WriteString(fmt.Sprintf("    mlse.if %s : %s {\n", cond, ty))
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	if s.Else != nil {
		buf.WriteString("    } else {\n")
		switch elseNode := s.Else.(type) {
		case *ast.BlockStmt:
			for _, stmt := range elseNode.List {
				buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
			}
		default:
			buf.WriteString(indentBlock(emitStmt(s.Else, env), 2))
		}
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitForStmt(s *ast.ForStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	cond := "true"
	ty := "i1"
	if s.Cond != nil {
		value, valueTy, prelude := emitExpr(s.Cond, env)
		buf.WriteString(prelude)
		cond, ty = value, valueTy
	}
	buf.WriteString(fmt.Sprintf("    mlse.for %s : %s {\n", cond, ty))
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	if s.Post != nil {
		buf.WriteString(indentBlock(emitStmt(s.Post, env), 2))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitRangeStmt(s *ast.RangeStmt, env *env) string {
	value, ty, prelude := emitExpr(s.X, env)
	var buf strings.Builder
	buf.WriteString(prelude)
	buf.WriteString(fmt.Sprintf("    mlse.range %s : %s {\n", value, ty))
	for _, stmt := range s.Body.List {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitSwitchStmt(s *ast.SwitchStmt, env *env) string {
	var buf strings.Builder
	if s.Init != nil {
		buf.WriteString(emitStmt(s.Init, env))
	}
	tag := "mlse.unit"
	ty := "!go.any"
	if s.Tag != nil {
		value, valueTy, prelude := emitExpr(s.Tag, env)
		buf.WriteString(prelude)
		tag, ty = value, valueTy
	}
	buf.WriteString(fmt.Sprintf("    mlse.switch %s : %s {\n", tag, ty))
	for _, stmt := range s.Body.List {
		clause, ok := stmt.(*ast.CaseClause)
		if !ok {
			buf.WriteString(indentBlock(fmt.Sprintf("    mlse.unsupported_stmt %q\n", shortNodeName(stmt)), 1))
			continue
		}
		buf.WriteString(indentBlock(emitCaseClause(clause, env), 1))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitCaseClause(clause *ast.CaseClause, env *env) string {
	var buf strings.Builder
	if len(clause.List) == 0 {
		buf.WriteString("    default {\n")
	} else {
		values := make([]string, 0, len(clause.List))
		caseTy := "!go.any"
		var prelude strings.Builder
		for idx, expr := range clause.List {
			value, ty, inner := emitExpr(expr, env)
			prelude.WriteString(inner)
			values = append(values, value)
			if idx == 0 {
				caseTy = ty
			}
		}
		buf.WriteString(prelude.String())
		buf.WriteString(fmt.Sprintf("    case %s : %s {\n", strings.Join(values, ", "), caseTy))
	}
	for _, stmt := range clause.Body {
		buf.WriteString(indentBlock(emitStmt(stmt, env), 2))
	}
	buf.WriteString("    }\n")
	return buf.String()
}

func emitExpr(expr ast.Expr, env *env) (value string, ty string, prelude string) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return e.Value, "i32", ""
		case token.STRING:
			return strconv.Quote(strings.Trim(e.Value, "\"`")), "!go.string", ""
		default:
			return fmt.Sprintf("mlse.literal(%q)", e.Value), "!go.any", ""
		}
	case *ast.Ident:
		if e.Name == "nil" {
			return "mlse.nil", "!go.nil", ""
		}
		if e.Name == "true" || e.Name == "false" {
			return e.Name, "i1", ""
		}
		return env.use(e.Name), env.typeOf(e.Name), ""
	case *ast.BinaryExpr:
		lhs, lhsTy, lhsPrelude := emitExpr(e.X, env)
		rhs, rhsTy, rhsPrelude := emitExpr(e.Y, env)
		resultTy := lhsTy
		if resultTy == "!go.any" {
			resultTy = rhsTy
		}
		op := binaryOpToMLIR(e.Op)
		if strings.HasPrefix(op, "mlse.") {
			resultTy = "!go.any"
		}
		return fmt.Sprintf("%s %s, %s", op, lhs, rhs), resultTy, lhsPrelude + rhsPrelude
	case *ast.CallExpr:
		var prelude strings.Builder
		fun, _, funPrelude := emitExpr(e.Fun, env)
		prelude.WriteString(funPrelude)
		args := make([]string, 0, len(e.Args))
		for _, arg := range e.Args {
			value, _, inner := emitExpr(arg, env)
			prelude.WriteString(inner)
			args = append(args, value)
		}
		tmp := env.temp("call")
		prelude.WriteString(fmt.Sprintf("    %s = mlse.call %s(%s) : !go.any\n", tmp, fun, strings.Join(args, ", ")))
		return tmp, "!go.any", prelude.String()
	case *ast.SelectorExpr:
		x, _, inner := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.select %s.%s", x, sanitizeName(e.Sel.Name)), "!go.any", inner
	case *ast.StarExpr:
		x, _, inner := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.load %s", x), "!go.any", inner
	case *ast.UnaryExpr:
		x, ty, inner := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.unary_%s %s", sanitizeName(e.Op.String()), x), ty, inner
	case *ast.ParenExpr:
		return emitExpr(e.X, env)
	case *ast.CompositeLit:
		tmp := env.temp("lit")
		return tmp, goTypeToMLIR(e.Type), fmt.Sprintf("    %s = mlse.composite %q : %s\n", tmp, shortNodeName(e), goTypeToMLIR(e.Type))
	case *ast.IndexExpr:
		x, _, px := emitExpr(e.X, env)
		idx, _, pi := emitExpr(e.Index, env)
		return fmt.Sprintf("mlse.index %s[%s]", x, idx), "!go.any", px + pi
	case *ast.SliceExpr:
		x, _, px := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.slice %s", x), "!go.any", px
	case *ast.TypeAssertExpr:
		x, _, px := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.typeassert %s", x), goTypeToMLIR(e.Type), px
	case *ast.FuncLit:
		tmp := env.temp("funclit")
		return tmp, "!go.func", fmt.Sprintf("    %s = mlse.funclit\n", tmp)
	case *ast.KeyValueExpr:
		k, _, pk := emitExpr(e.Key, env)
		v, _, pv := emitExpr(e.Value, env)
		return fmt.Sprintf("mlse.kv %s, %s", k, v), "!go.kv", pk + pv
	default:
		return fmt.Sprintf("mlse.unsupported_expr(%q)", shortNodeName(expr)), "!go.any", ""
	}
}

func binaryOpToMLIR(op token.Token) string {
	switch op {
	case token.ADD:
		return "arith.addi"
	case token.SUB:
		return "arith.subi"
	case token.MUL:
		return "arith.muli"
	case token.QUO:
		return "arith.divsi"
	case token.EQL:
		return "arith.cmpi_eq"
	case token.NEQ:
		return "arith.cmpi_ne"
	case token.GTR:
		return "arith.cmpi_gt"
	case token.LSS:
		return "arith.cmpi_lt"
	case token.GEQ:
		return "arith.cmpi_ge"
	case token.LEQ:
		return "arith.cmpi_le"
	case token.LAND:
		return "mlse.and"
	case token.LOR:
		return "mlse.or"
	default:
		return "mlse.binop_" + sanitizeName(op.String())
	}
}

func goTypeToMLIR(expr ast.Expr) string {
	switch t := expr.(type) {
	case nil:
		return "!go.unit"
	case *ast.Ident:
		switch t.Name {
		case "int":
			return "i32"
		case "bool":
			return "i1"
		case "string":
			return "!go.string"
		case "error":
			return "!go.error"
		case "any", "interface{}":
			return "!go.any"
		default:
			return "!go.named<\"" + sanitizeName(t.Name) + "\">"
		}
	case *ast.StarExpr:
		return "!go.ptr<" + goTypeToMLIR(t.X) + ">"
	case *ast.SelectorExpr:
		return "!go.sel<\"" + sanitizeName(renderSelector(t)) + "\">"
	case *ast.ArrayType:
		if t.Len == nil {
			return "!go.slice<" + goTypeToMLIR(t.Elt) + ">"
		}
		return "!go.array<" + goTypeToMLIR(t.Elt) + ">"
	case *ast.MapType:
		return "!go.map<" + goTypeToMLIR(t.Key) + "," + goTypeToMLIR(t.Value) + ">"
	case *ast.InterfaceType:
		return "!go.interface"
	case *ast.FuncType:
		return "!go.func"
	case *ast.StructType:
		return "!go.struct"
	case *ast.ChanType:
		return "!go.chan<" + goTypeToMLIR(t.Value) + ">"
	case *ast.Ellipsis:
		return "!go.vararg<" + goTypeToMLIR(t.Elt) + ">"
	case *ast.ParenExpr:
		return goTypeToMLIR(t.X)
	default:
		return "!go.any"
	}
}

func renderSelector(s *ast.SelectorExpr) string {
	parts := []string{sanitizeName(s.Sel.Name)}
	for {
		switch x := s.X.(type) {
		case *ast.Ident:
			parts = append([]string{sanitizeName(x.Name)}, parts...)
			return strings.Join(parts, ".")
		case *ast.SelectorExpr:
			parts = append([]string{sanitizeName(x.Sel.Name)}, parts...)
			s = x
		default:
			return strings.Join(parts, ".")
		}
	}
}

func shortNodeName(node any) string {
	name := fmt.Sprintf("%T", node)
	return strings.TrimPrefix(name, "*ast.")
}

func sanitizeName(name string) string {
	if name == "" {
		return "anon"
	}
	var b strings.Builder
	for _, r := range name {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '.' {
			b.WriteRune(r)
		} else {
			b.WriteRune('_')
		}
	}
	return b.String()
}

func indentBlock(text string, levels int) string {
	if text == "" {
		return ""
	}
	indent := strings.Repeat("  ", levels)
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		if line == "" {
			continue
		}
		lines[i] = indent + line
	}
	return strings.Join(lines, "\n")
}

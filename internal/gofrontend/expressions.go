package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

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
		if !isAtomicMLIRValue(lhs) {
			tmp := env.temp("bin")
			lhsPrelude += fmt.Sprintf("    %s = %s : %s\n", tmp, lhs, lhsTy)
			lhs = tmp
		}
		if !isAtomicMLIRValue(rhs) {
			tmp := env.temp("bin")
			rhsPrelude += fmt.Sprintf("    %s = %s : %s\n", tmp, rhs, rhsTy)
			rhs = tmp
		}
		resultTy := lhsTy
		if e.Op == token.LAND || e.Op == token.LOR {
			resultTy = "i1"
		}
		if resultTy == "!go.any" {
			resultTy = rhsTy
		}
		op := binaryOpToMLIR(e.Op)
		if strings.HasPrefix(op, "mlse.") {
			resultTy = "!go.any"
		}
		return fmt.Sprintf("%s %s, %s", op, lhs, rhs), resultTy, lhsPrelude + rhsPrelude
	case *ast.CallExpr:
		if ident, ok := e.Fun.(*ast.Ident); ok && ident.Name == "make" && len(e.Args) > 0 {
			targetTy := goTypeToMLIR(e.Args[0])
			var prelude strings.Builder
			for _, arg := range e.Args[1:] {
				_, _, inner := emitExpr(arg, env)
				prelude.WriteString(inner)
			}
			tmp := env.temp("make")
			prelude.WriteString(fmt.Sprintf("    %s = mlse.zero : %s\n", tmp, targetTy))
			return tmp, targetTy, prelude.String()
		}
		if builtin, ok := builtinCallName(e.Fun); ok {
			return emitBuiltinCallExpr(builtin, e, env)
		}
		if len(e.Args) == 1 {
			targetTy := goTypeToMLIR(e.Fun)
			if targetTy != "!go.any" {
				value, _, inner := emitExpr(e.Args[0], env)
				return value, targetTy, inner
			}
		}
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
		resultTy := inferCallResultType(e, env)
		prelude.WriteString(fmt.Sprintf("    %s = mlse.call %s(%s) : %s\n", tmp, fun, strings.Join(args, ", "), resultTy))
		return tmp, resultTy, prelude.String()
	case *ast.SelectorExpr:
		x, _, inner := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.select %s.%s", x, sanitizeName(e.Sel.Name)), "!go.any", inner
	case *ast.StarExpr:
		x, _, inner := emitExpr(e.X, env)
		return fmt.Sprintf("mlse.load %s", x), "!go.any", inner
	case *ast.UnaryExpr:
		x, ty, inner := emitExpr(e.X, env)
		switch e.Op {
		case token.NOT:
			return fmt.Sprintf("mlse.not %s", x), "i1", inner
		case token.AND:
			return fmt.Sprintf("mlse.addr %s", x), "!go.ptr<" + ty + ">", inner
		case token.SUB:
			return fmt.Sprintf("mlse.neg %s", x), ty, inner
		default:
			return fmt.Sprintf("mlse.unary_%s %s", sanitizeName(e.Op.String()), x), ty, inner
		}
	case *ast.ParenExpr:
		return emitExpr(e.X, env)
	case *ast.CompositeLit:
		litTy := goTypeToMLIR(e.Type)
		tmp := env.temp("lit")
		return tmp, litTy, fmt.Sprintf("    %s = mlse.composite %q : %s\n", tmp, shortNodeName(e), litTy)
	case *ast.IndexExpr:
		x, xTy, px := emitExpr(e.X, env)
		idx, _, pi := emitExpr(e.Index, env)
		return fmt.Sprintf("mlse.index %s[%s]", x, idx), rangeValueType(xTy), px + pi
	case *ast.SliceExpr:
		return emitSliceExpr(e, env)
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

func isAtomicMLIRValue(value string) bool {
	if value == "" || strings.ContainsAny(value, " ,") {
		return false
	}
	if strings.HasPrefix(value, "\"") && strings.HasSuffix(value, "\"") {
		return true
	}
	return true
}

func builtinCallName(expr ast.Expr) (string, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok {
		return "", false
	}
	switch ident.Name {
	case "len", "cap", "copy", "append":
		return ident.Name, true
	default:
		return "", false
	}
}

func emitBuiltinCallExpr(name string, call *ast.CallExpr, env *env) (string, string, string) {
	var prelude strings.Builder
	args := make([]string, 0, len(call.Args))
	for _, arg := range call.Args {
		value, valueTy, inner := emitExpr(arg, env)
		value, inner = materializeExprValue(value, valueTy, inner, env, name+"_arg")
		prelude.WriteString(inner)
		args = append(args, value)
	}
	resultTy := inferCallResultType(call, env)
	tmp := env.temp("call")
	prelude.WriteString(formatBuiltinCall(tmp, name, args, resultTy))
	return tmp, resultTy, prelude.String()
}

func formatBuiltinCall(dest, name string, args []string, resultTy string) string {
	return fmt.Sprintf("    %s = mlse.call %%%s(%s) : %s\n", dest, sanitizeName(name), strings.Join(args, ", "), resultTy)
}

func emitSliceExpr(expr *ast.SliceExpr, env *env) (string, string, string) {
	base, baseTy, prelude := emitExpr(expr.X, env)
	base, prelude = materializeExprValue(base, baseTy, prelude, env, "slice_base")
	low, lowPrelude := emitSliceBound(expr.Low, env, "slice_low")
	high, highPrelude := emitSliceBound(expr.High, env, "slice_high")
	prelude += lowPrelude + highPrelude

	resultTy := inferSliceExprType(expr, baseTy)
	if expr.Slice3 || expr.Max != nil {
		return fmt.Sprintf("mlse.slice %s", base), resultTy, prelude
	}
	if low == "" && high == "" {
		return fmt.Sprintf("mlse.slice %s", base), resultTy, prelude
	}
	return fmt.Sprintf("mlse.slice %s[%s:%s]", base, low, high), resultTy, prelude
}

func emitSliceBound(expr ast.Expr, env *env, prefix string) (string, string) {
	if expr == nil {
		return "", ""
	}
	value, ty, prelude := emitExpr(expr, env)
	return materializeExprValue(value, ty, prelude, env, prefix)
}

func materializeExprValue(value, ty, prelude string, env *env, prefix string) (string, string) {
	if isAtomicMLIRValue(value) {
		return value, prelude
	}
	tmp := env.temp(prefix)
	return tmp, prelude + fmt.Sprintf("    %s = %s : %s\n", tmp, value, ty)
}

func inferExprType(expr ast.Expr, env *env) string {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "i32"
		case token.STRING:
			return "!go.string"
		}
	case *ast.Ident:
		switch e.Name {
		case "nil":
			return "!go.nil"
		case "true", "false":
			return "i1"
		default:
			return env.typeOf(e.Name)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ, token.LAND, token.LOR:
			return "i1"
		default:
			lhsTy := inferExprType(e.X, env)
			if lhsTy != "!go.any" {
				return lhsTy
			}
			return inferExprType(e.Y, env)
		}
	case *ast.CallExpr:
		return inferCallResultType(e, env)
	case *ast.CompositeLit:
		return goTypeToMLIR(e.Type)
	case *ast.UnaryExpr:
		switch e.Op {
		case token.NOT:
			return "i1"
		case token.AND:
			return "!go.ptr<" + inferExprType(e.X, env) + ">"
		default:
			return inferExprType(e.X, env)
		}
	case *ast.IndexExpr:
		return rangeValueType(inferExprType(e.X, env))
	case *ast.SliceExpr:
		return inferExprType(e.X, env)
	}
	return "!go.any"
}

func inferCallResultType(call *ast.CallExpr, env *env) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "len", "cap", "copy":
			return "i32"
		case "append":
			if len(call.Args) > 0 {
				return inferExprType(call.Args[0], env)
			}
		case "make":
			if len(call.Args) > 0 {
				return goTypeToMLIR(call.Args[0])
			}
		default:
			if len(call.Args) == 1 {
				return goTypeToMLIR(fun)
			}
		}
	case *ast.SelectorExpr:
		switch renderSelector(fun) {
		case "fmt.Sprintf":
			return "!go.string"
		case "fmt.Errorf":
			return "!go.error"
		}
	}
	return "!go.any"
}

func inferSliceExprType(expr *ast.SliceExpr, baseTy string) string {
	if strings.HasPrefix(baseTy, "!go.string") {
		return "!go.string"
	}
	if strings.HasPrefix(baseTy, "!go.slice<") {
		return baseTy
	}
	if strings.HasPrefix(baseTy, "!go.array<") {
		elem := strings.TrimSuffix(strings.TrimPrefix(baseTy, "!go.array<"), ">")
		return "!go.slice<" + elem + ">"
	}
	return "!go.any"
}

func isIntegerLikeType(ty string) bool {
	switch ty {
	case "i1", "i8", "i16", "i32", "i64":
		return true
	}
	return strings.HasPrefix(ty, "!go.named<")
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

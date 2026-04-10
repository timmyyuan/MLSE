package gofrontend

import (
	"fmt"
	"go/ast"
	"go/types"
	"strings"
)

func extractSingleReturnExpr(stmts []ast.Stmt) (ast.Expr, bool) {
	if len(stmts) != 1 {
		return nil, false
	}
	ret, ok := stmts[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, false
	}
	return ret.Results[0], true
}

func extractTrailingReturnExprs(stmts []ast.Stmt) ([]ast.Stmt, []ast.Expr, bool) {
	if len(stmts) == 0 {
		return nil, nil, false
	}
	ret, ok := stmts[len(stmts)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) == 0 {
		return nil, nil, false
	}
	return stmts[:len(stmts)-1], ret.Results, true
}

func isFormalBuiltinPanicCall(call *ast.CallExpr, env *formalEnv) bool {
	if call == nil {
		return false
	}
	ident, ok := formalUnparenExpr(call.Fun).(*ast.Ident)
	if !ok || ident.Name != "panic" {
		return false
	}
	if env != nil {
		if _, ok := env.locals[ident.Name]; ok {
			return false
		}
	}
	if env == nil || env.module == nil || env.module.typed == nil || env.module.typed.info == nil {
		return true
	}
	obj := env.module.typed.info.ObjectOf(ident)
	if obj == nil {
		return true
	}
	builtin, ok := obj.(*types.Builtin)
	return ok && builtin.Name() == "panic"
}

func emitFormalTerminatingStmt(stmt ast.Stmt, env *formalEnv) (string, bool) {
	exprStmt, ok := stmt.(*ast.ExprStmt)
	if !ok {
		return "", false
	}
	call, ok := exprStmt.X.(*ast.CallExpr)
	if !ok || !isFormalBuiltinPanicCall(call, env) {
		return "", false
	}
	text, lowered := emitFormalCallStmt(call, env)
	return text, lowered
}

// emitFormalReturnExprOperands lowers explicit return operands and coerces them to function result types.
func emitFormalReturnExprOperands(exprs []ast.Expr, resultTypes []string, env *formalEnv) ([]string, []string, string, bool) {
	if len(exprs) != len(resultTypes) {
		return nil, nil, "", false
	}
	var (
		values []string
		types  []string
		buf    strings.Builder
	)
	for i, expr := range exprs {
		hint := resultTypes[i]
		value, ty, prelude := emitFormalExpr(expr, hint, env)
		buf.WriteString(prelude)
		if hint != "" && normalizeFormalType(ty) != normalizeFormalType(hint) {
			if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, ty, hint, env); ok {
				buf.WriteString(coercedPrelude)
				value = coercedValue
				ty = coercedTy
			} else {
				todoValue, todoTy, todoPrelude := emitFormalTodoValue("return_type_mismatch", hint, env)
				buf.WriteString(todoPrelude)
				value = todoValue
				ty = todoTy
			}
		}
		values = append(values, value)
		types = append(types, normalizeFormalType(ty))
	}
	return values, types, buf.String(), true
}

func emitFormalYieldLine(values []string, types []string, env *formalEnv) string {
	return emitFormalLinef(nil, env, "        scf.yield %s : %s", strings.Join(values, ", "), strings.Join(types, ", "))
}

func formalMultiResultRefs(base string, arity int) []string {
	if arity <= 1 {
		return []string{base}
	}
	values := make([]string, 0, arity)
	for i := 0; i < arity; i++ {
		values = append(values, fmt.Sprintf("%s#%d", base, i))
	}
	return values
}

func formalIfResultBinding(base string, arity int) string {
	if arity <= 1 {
		return base
	}
	return fmt.Sprintf("%s:%d", base, arity)
}

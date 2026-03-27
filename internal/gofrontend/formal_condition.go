package gofrontend

import "go/ast"

// emitFormalCondition produces an `i1` condition for structured control flow.
func emitFormalCondition(expr ast.Expr, env *formalEnv) (string, string, bool) {
	cond, condTy, prelude := emitFormalExpr(expr, "i1", env)
	if condTy == "i1" {
		return cond, prelude, true
	}
	if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(cond, condTy, "i1", env); ok && coercedTy == "i1" {
		return coercedValue, prelude + coercedPrelude, true
	}
	return "", prelude, false
}

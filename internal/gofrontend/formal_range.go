// This file contains range-specific type inference helpers used by `emitFormalRangeStmt`.
package gofrontend

import (
	"go/ast"
	"go/token"
)

func inferFormalRangeValueType(name string, sourceTy string, body *ast.BlockStmt, env *formalEnv) string {
	valueTy := formalIndexResultType(sourceTy)
	if !isFormalOpaquePlaceholderType(valueTy) {
		return valueTy
	}
	if env != nil {
		existingTy := normalizeFormalType(env.typeOf(name))
		if !isFormalOpaquePlaceholderType(existingTy) {
			return existingTy
		}
	}
	if body == nil || name == "" {
		return valueTy
	}
	usedTy := inferFormalIdentUsageType(body, name, env)
	if !isFormalOpaquePlaceholderType(usedTy) {
		return usedTy
	}
	return valueTy
}

func inferFormalIdentUsageType(node ast.Node, name string, env *formalEnv) string {
	if node == nil || name == "" {
		return formalOpaqueType("value")
	}

	best := formalOpaqueType("value")
	var stack []ast.Node
	ast.Inspect(node, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		parent := ast.Node(nil)
		if len(stack) != 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)

		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name != name || isFormalDefinitionIdent(parent, ident) {
			return true
		}

		hint := inferFormalIdentContextType(ident, parent, env)
		best = chooseFormalCommonType(best, hint)
		return true
	})
	return normalizeFormalType(best)
}

func inferFormalIdentContextType(ident *ast.Ident, parent ast.Node, env *formalEnv) string {
	switch node := parent.(type) {
	case *ast.BinaryExpr:
		other := node.X
		if node.X == ident {
			other = node.Y
		}
		switch node.Op {
		case token.ADD:
			if isFormalStringLikeExpr(other, env) {
				return "!go.string"
			}
		case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ:
			if isFormalStringLikeExpr(other, env) {
				return "!go.string"
			}
		case token.LAND, token.LOR:
			return "i1"
		}
	case *ast.CallExpr:
		for i, arg := range node.Args {
			if arg != ident {
				continue
			}
			if hint := formalCallArgHint(node, i, env); hint != "" {
				return hint
			}
		}
	}
	return formalOpaqueType("value")
}

func formalCallArgHint(call *ast.CallExpr, index int, env *formalEnv) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "len", "cap":
			if index == 0 {
				return formalOpaqueType("value")
			}
		}
	case *ast.SelectorExpr:
		switch renderSelector(fun) {
		case "strings.Contains", "strings.Split", "strings.HasPrefix", "strings.HasSuffix":
			return "!go.string"
		}
	}
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && index < len(sig.params) {
		return normalizeFormalType(sig.params[index])
	}
	return ""
}

func isFormalStringLikeExpr(expr ast.Expr, env *formalEnv) bool {
	switch e := expr.(type) {
	case *ast.BasicLit:
		return e.Kind == token.STRING
	case *ast.BinaryExpr:
		return e.Op == token.ADD && (isFormalStringLikeExpr(e.X, env) || isFormalStringLikeExpr(e.Y, env))
	case *ast.CallExpr:
		return normalizeFormalType(inferFormalCallResultType(e, "", env)) == "!go.string"
	case *ast.Ident, *ast.ParenExpr, *ast.SelectorExpr:
		return normalizeFormalType(inferFormalExprType(expr, env)) == "!go.string"
	}
	return normalizeFormalType(inferFormalExprType(expr, env)) == "!go.string"
}

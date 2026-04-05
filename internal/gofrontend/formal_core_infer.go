package gofrontend

import (
	"go/ast"
	"go/token"
)

func formalBinaryOperandHint(expr *ast.BinaryExpr, env *formalEnv) string {
	switch expr.Op {
	case token.LAND, token.LOR:
		return "i1"
	}
	if isFormalNilExpr(expr.X) {
		return normalizeFormalType(inferFormalExprType(expr.Y, env))
	}
	if isFormalNilExpr(expr.Y) {
		return normalizeFormalType(inferFormalExprType(expr.X, env))
	}

	lhsTy := inferFormalExprType(expr.X, env)
	rhsTy := inferFormalExprType(expr.Y, env)
	hint := chooseFormalCommonType(lhsTy, rhsTy)
	if !isFormalOpaquePlaceholderType(hint) {
		return hint
	}
	if !isFormalOpaquePlaceholderType(normalizeFormalType(lhsTy)) {
		return normalizeFormalType(lhsTy)
	}
	return normalizeFormalType(rhsTy)
}

func inferFormalExprType(expr ast.Expr, env *formalEnv) string {
	if env != nil && env.module != nil {
		if ty, ok := formalTypedExprType(expr, env.module); ok && isFormalTypedInfoUsableType(ty) {
			return ty
		}
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return formalTargetIntType(env.module)
		case token.FLOAT:
			return "f64"
		case token.STRING:
			return "!go.string"
		}
	case *ast.Ident:
		switch e.Name {
		case "nil":
			return "!go.error"
		case "true", "false":
			return "i1"
		default:
			if env != nil && env.module != nil {
				if sig, ok := lookupFormalDefinedFuncSig(env.module, formalTopLevelSymbol(env.module, e.Name)); ok {
					return formatFormalFuncType(sig.params, sig.results)
				}
			}
			return env.typeOf(e.Name)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ, token.LAND, token.LOR:
			return "i1"
		default:
			return inferFormalExprType(e.X, env)
		}
	case *ast.CallExpr:
		return inferFormalCallResultType(e, "", env)
	case *ast.FuncLit:
		return formalTypeExprToMLIR(e.Type, env.module)
	case *ast.IndexExpr:
		return formalIndexResultType(inferFormalExprType(e.X, env))
	case *ast.ParenExpr:
		return inferFormalExprType(e.X, env)
	case *ast.SelectorExpr:
		return formalOpaqueType("value")
	case *ast.StarExpr:
		return formalDerefType(inferFormalExprType(e.X, env))
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return formalTypeExprToMLIR(e.Type, env.module)
		}
		return formalOpaqueType("value")
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return "!go.ptr<" + inferFormalExprType(e.X, env) + ">"
		}
		return inferFormalExprType(e.X, env)
	}
	return formalOpaqueType("value")
}

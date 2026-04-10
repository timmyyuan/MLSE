package gofrontend

import (
	"go/ast"
	"go/token"
	"strings"
)

func emitFormalHelperCall(spec formalHelperCallSpec, env *formalEnv) (string, string) {
	resultTy := normalizeFormalType(spec.resultTy)
	symbol := registerFormalExtern(env.module, spec.base, spec.argTys, []string{resultTy})
	tmp := env.temp(spec.tempPrefix)
	return tmp, emitFormalLinef(nil, env, "    %s = func.call @%s(%s) : (%s) -> %s", tmp, symbol, strings.Join(spec.args, ", "), strings.Join(spec.argTys, ", "), resultTy)
}

func coerceFormalValueToHint(value string, valueTy string, hintedTy string, env *formalEnv) (string, string, string) {
	valueTy = normalizeFormalType(valueTy)
	if hintedTy == "" {
		return value, valueTy, ""
	}
	targetTy := normalizeFormalType(hintedTy)
	if isFormalOpaquePlaceholderType(targetTy) || valueTy == targetTy {
		return value, valueTy, ""
	}
	if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, valueTy, targetTy, env); ok {
		return coercedValue, normalizeFormalType(coercedTy), coercedPrelude
	}
	todoValue, todoTy, todoPrelude := emitFormalTodoValue("type_conversion", targetTy, env)
	return todoValue, normalizeFormalType(todoTy), todoPrelude
}

func isFormalOpaquePlaceholderType(ty string) bool {
	ty = normalizeFormalType(ty)
	return ty == formalOpaqueType("value") || ty == formalOpaqueType("result")
}

func chooseFormalCommonType(lhsTy string, rhsTy string) string {
	lhsTy = normalizeFormalType(lhsTy)
	rhsTy = normalizeFormalType(rhsTy)
	switch {
	case isFormalOpaquePlaceholderType(lhsTy) && !isFormalOpaquePlaceholderType(rhsTy):
		return rhsTy
	case isFormalOpaquePlaceholderType(rhsTy) && !isFormalOpaquePlaceholderType(lhsTy):
		return lhsTy
	case lhsTy == rhsTy:
		return lhsTy
	case lhsTy == formalOpaqueType("value"):
		return rhsTy
	case rhsTy == formalOpaqueType("value"):
		return lhsTy
	default:
		return lhsTy
	}
}

func isFormalDefinitionIdent(parent ast.Node, ident *ast.Ident) bool {
	switch node := parent.(type) {
	case *ast.AssignStmt:
		if node.Tok != token.DEFINE {
			return false
		}
		for _, lhs := range node.Lhs {
			if lhs == ident {
				return true
			}
		}
	case *ast.Field:
		for _, name := range node.Names {
			if name == ident {
				return true
			}
		}
	case *ast.ValueSpec:
		for _, name := range node.Names {
			if name == ident {
				return true
			}
		}
	case *ast.RangeStmt:
		return (node.Key == ident || node.Value == ident) && node.Tok == token.DEFINE
	case *ast.SelectorExpr:
		return node.Sel == ident
	}
	return false
}

func syntheticFormalAssignType(name string, env *formalEnv) string {
	if ty := env.typeOf(name); ty != formalOpaqueType("value") {
		return ty
	}
	switch name {
	case "ok", "found", "exists":
		return "i1"
	case "err":
		return "!go.error"
	default:
		return formalOpaqueType("value")
	}
}

func formalOpaqueType(name string) string {
	return "!go.named<\"" + sanitizeName(name) + "\">"
}

func normalizeFormalType(ty string) string {
	if ty == "" {
		return formalOpaqueType("value")
	}
	return ty
}

func normalizeFormalElementType(ty string) string {
	return normalizeFormalType(ty)
}

func normalizeFormalBoolConst(ty string, boolConst string) string {
	if normalizeFormalType(ty) != "i1" {
		return ""
	}
	switch boolConst {
	case "true", "false":
		return boolConst
	default:
		return ""
	}
}

func formalUnparenExpr(expr ast.Expr) ast.Expr {
	for {
		paren, ok := expr.(*ast.ParenExpr)
		if !ok {
			return expr
		}
		expr = paren.X
	}
}

func formalKnownBoolExpr(expr ast.Expr, env *formalEnv) (bool, bool) {
	switch node := formalUnparenExpr(expr).(type) {
	case *ast.Ident:
		switch node.Name {
		case "true":
			return true, true
		case "false":
			return false, true
		default:
			if env == nil {
				return false, false
			}
			return env.boolConstOf(node.Name)
		}
	case *ast.UnaryExpr:
		if node.Op != token.NOT {
			return false, false
		}
		value, ok := formalKnownBoolExpr(node.X, env)
		if !ok {
			return false, false
		}
		return !value, true
	default:
		return false, false
	}
}

func isFormalIntegerType(ty string) bool {
	switch ty {
	case "i8", "i16", "i32", "i64", "index":
		return true
	}
	return false
}

func isFormalNilableType(ty string) bool {
	return ty == "!go.error" || strings.HasPrefix(ty, "!go.ptr<") || strings.HasPrefix(ty, "!go.slice<")
}

func isFormalTypeConversionCall(call *ast.CallExpr, module *formalModuleContext) bool {
	if len(call.Args) != 1 {
		return false
	}
	return isFormalTypeExpr(call.Fun, module)
}

func isFormalTypeExpr(expr ast.Expr, module *formalModuleContext) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "any", "bool", "byte", "complex128", "complex64", "error", "float32", "float64", "int", "int16", "int32", "int64", "int8", "rune", "string", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr":
			return true
		default:
			return formalModuleIsNamedType(module, t.Name)
		}
	case *ast.StarExpr, *ast.ArrayType, *ast.InterfaceType, *ast.StructType, *ast.MapType, *ast.FuncType, *ast.ChanType, *ast.Ellipsis:
		return true
	case *ast.ParenExpr:
		return isFormalTypeExpr(t.X, module)
	default:
		return false
	}
}

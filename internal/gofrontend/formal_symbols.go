package gofrontend

import (
	"go/ast"
	"go/types"
)

func isFormalPackageSelector(expr *ast.SelectorExpr, env *formalEnv) bool {
	if env != nil && env.module != nil && env.module.typed != nil && env.module.typed.info != nil {
		root := selectorRootIdent(expr)
		if root != nil {
			if _, ok := env.module.typed.info.ObjectOf(root).(*types.PkgName); ok {
				return true
			}
		}
	}
	root := selectorRootIdent(expr)
	if root == nil || env == nil {
		return false
	}
	_, ok := env.locals[root.Name]
	return !ok
}

func selectorRootIdent(expr *ast.SelectorExpr) *ast.Ident {
	current := expr
	for {
		switch x := current.X.(type) {
		case *ast.Ident:
			return x
		case *ast.SelectorExpr:
			current = x
		default:
			return nil
		}
	}
}

func inferFormalCallResultType(call *ast.CallExpr, hintedTy string, env *formalEnv) string {
	if hintedTy != "" {
		return normalizeFormalType(hintedTy)
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "len", "cap", "copy":
			return "i32"
		case "append":
			if len(call.Args) > 0 {
				return inferFormalExprType(call.Args[0], env)
			}
			return "!go.slice<i32>"
		case "make":
			if len(call.Args) > 0 {
				return formalTypeExprToMLIR(call.Args[0], env.module)
			}
		}
	case *ast.SelectorExpr:
		switch renderSelector(fun) {
		case "strings.Split":
			return "!go.slice<!go.string>"
		case "fmt.Sprintf":
			return "!go.string"
		case "fmt.Errorf":
			return "!go.error"
		}
	}
	if sig, ok := formalExprFuncSig(call.Fun, env); ok {
		if len(sig.results) == 1 {
			return sig.results[0]
		}
	}
	return formalOpaqueType("result")
}

func goTypeToFormalMLIR(expr ast.Expr) string {
	switch t := expr.(type) {
	case nil:
		return formalOpaqueType("unit")
	case *ast.Ident:
		if builtinTy, ok := formalBuiltinType(t.Name); ok {
			return builtinTy
		}
		return "!go.named<\"" + sanitizeName(t.Name) + "\">"
	case *ast.SelectorExpr:
		return "!go.named<\"" + sanitizeName(renderSelector(t)) + "\">"
	case *ast.StarExpr:
		return "!go.ptr<" + goTypeToFormalMLIR(t.X) + ">"
	case *ast.ArrayType:
		if t.Len == nil {
			return "!go.slice<" + normalizeFormalElementType(goTypeToFormalMLIR(t.Elt)) + ">"
		}
		return formalOpaqueType("array")
	case *ast.MapType:
		return formalOpaqueType("map")
	case *ast.InterfaceType:
		return formalOpaqueType("interface")
	case *ast.FuncType:
		sig := formalFuncSigFromType(t, nil)
		return formatFormalFuncType(sig.params, sig.results)
	case *ast.StructType:
		return formalOpaqueType("struct")
	case *ast.ChanType:
		return formalOpaqueType("chan")
	case *ast.Ellipsis:
		return formalOpaqueType("vararg")
	case *ast.ParenExpr:
		return goTypeToFormalMLIR(t.X)
	default:
		return formalOpaqueType("type")
	}
}

func formalCalleeName(expr ast.Expr, module *formalModuleContext) string {
	switch callee := expr.(type) {
	case *ast.Ident:
		if module != nil {
			return module.topLevelSymbol(callee.Name)
		}
		return sanitizeName(callee.Name)
	case *ast.SelectorExpr:
		return formalPackageSelectorSymbol(callee, nil)
	default:
		return ""
	}
}

func formalCallSymbol(expr ast.Expr, argTys []string, resultTys []string, module *formalModuleContext) string {
	callee := ""
	if selector, ok := expr.(*ast.SelectorExpr); ok && module != nil && formalImportPathForSelector(selector, module) != "" {
		callee = formalPackageSelectorSymbol(selector, module)
	} else {
		callee = formalCalleeName(expr, module)
	}
	if callee == "" {
		return ""
	}
	if module == nil || module.isDefinedFunc(callee) {
		return callee
	}
	return module.registerExtern(callee, argTys, resultTys)
}

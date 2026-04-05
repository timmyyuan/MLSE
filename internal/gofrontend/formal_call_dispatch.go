package gofrontend

import (
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

func formalFuncSigFromType(fnType *ast.FuncType, module *formalModuleContext) formalFuncSig {
	if fnType == nil {
		return formalFuncSig{}
	}
	return formalFuncSig{
		params:  emitFieldTypes(fnType.Params, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
		results: emitFieldTypes(fnType.Results, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
	}
}

func formalFuncSigFromDecl(fn *ast.FuncDecl, module *formalModuleContext) formalFuncSig {
	if fn == nil {
		return formalFuncSig{}
	}
	return formalFuncSig{
		params:  emitFieldTypes(formalJoinFieldLists(fn.Recv, fn.Type.Params), func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
		results: emitFieldTypes(fn.Type.Results, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
	}
}

func emitFormalFuncLitExpr(lit *ast.FuncLit, hintedTy string, env *formalEnv) (string, string, string) {
	funcTy := formalTypeExprToMLIR(lit.Type, env.module)
	if funcTy == formalOpaqueType("func") {
		funcTy = normalizeFormalType(hintedTy)
	}
	sig, ok := parseFormalFuncType(funcTy)
	if !ok {
		return emitFormalTodoValue("FuncLit_type", normalizeFormalType(hintedTy), env)
	}

	captures := formalFuncLitCaptures(lit, env)
	if len(captures) != 0 {
		return emitFormalTodoValue("FuncLit_capture", funcTy, env)
	}

	symbol := reserveFormalFuncLitSymbol(env.module, sig, env.currentFunc)
	addFormalGeneratedFunc(env.module, emitFormalFuncBody(formalFuncBodySpec{
		name:      symbol,
		fnType:    lit.Type,
		body:      lit.Body,
		private:   true,
		scopeNode: lit,
	}, env.module))

	tmp := env.temp("funclit")
	return tmp, funcTy, emitFormalLinef(lit, env, "    %s = func.constant @%s : %s", tmp, symbol, funcTy)
}

func formalFuncLitCaptures(lit *ast.FuncLit, env *formalEnv) []string {
	if lit == nil || env == nil {
		return nil
	}

	localNames := collectFormalFuncLocalNames(lit.Type, lit.Body)
	seen := make(map[string]struct{})
	var stack []ast.Node
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		parent := ast.Node(nil)
		if len(stack) != 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)

		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name == "_" {
			return true
		}
		if isFormalDefinitionIdent(parent, ident) {
			return true
		}
		if _, ok := localNames[ident.Name]; ok {
			return true
		}
		if _, ok := env.locals[ident.Name]; ok {
			seen[ident.Name] = struct{}{}
		}
		return true
	})

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func collectFormalFuncLocalNames(fnType *ast.FuncType, body *ast.BlockStmt) map[string]struct{} {
	names := make(map[string]struct{})
	collectFormalFieldNames(names, fnType.Params)
	collectFormalFieldNames(names, fnType.Results)
	if body == nil {
		return names
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case nil:
			return false
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, lhs := range node.Lhs {
					ident, ok := lhs.(*ast.Ident)
					if ok && ident.Name != "_" {
						names[ident.Name] = struct{}{}
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				if ident, ok := node.Key.(*ast.Ident); ok && ident.Name != "_" {
					names[ident.Name] = struct{}{}
				}
				if ident, ok := node.Value.(*ast.Ident); ok && ident.Name != "_" {
					names[ident.Name] = struct{}{}
				}
			}
		case *ast.DeclStmt:
			gen, ok := node.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				return true
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if name.Name != "_" {
						names[name.Name] = struct{}{}
					}
				}
			}
		}
		return true
	})
	return names
}

func collectFormalFieldNames(out map[string]struct{}, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.Name != "_" {
				out[name.Name] = struct{}{}
			}
		}
	}
}

func emitFormalCallOperands(args []ast.Expr, env *formalEnv) ([]string, []string, string) {
	return emitFormalCallOperandsWithHints(args, nil, env)
}

func emitFormalCallOperandsWithHints(args []ast.Expr, hints []string, env *formalEnv) ([]string, []string, string) {
	var (
		values []string
		types  []string
		buf    strings.Builder
	)
	for i, arg := range args {
		hint := ""
		if i < len(hints) {
			hint = hints[i]
		}
		value, ty, prelude := emitFormalExpr(arg, hint, env)
		buf.WriteString(prelude)
		values = append(values, value)
		types = append(types, ty)
	}
	return values, types, buf.String()
}

func formalDirectCallSymbol(expr ast.Expr, argTys []string, resultTys []string, env *formalEnv) (string, bool) {
	switch callee := expr.(type) {
	case *ast.Ident:
		if env != nil {
			if _, ok := env.locals[callee.Name]; ok {
				return "", false
			}
		}
	case *ast.SelectorExpr:
		if !isFormalPackageSelector(callee, env) {
			return "", false
		}
	default:
		return "", false
	}

	symbol := formalCallSymbol(expr, argTys, resultTys, env.module)
	return symbol, symbol != ""
}

func formalExprFuncSig(expr ast.Expr, env *formalEnv) (formalFuncSig, bool) {
	if env != nil && env.module != nil {
		if sig, ok := formalTypedExprFuncSig(expr, env.module); ok {
			return sig, true
		}
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if env != nil {
			if binding, ok := env.locals[e.Name]; ok && binding.funcSig != nil {
				return *binding.funcSig, true
			}
		}
		if env != nil && env.module != nil {
			if sig, ok := lookupFormalDefinedFuncSig(env.module, formalTopLevelSymbol(env.module, e.Name)); ok {
				return sig, true
			}
		}
	case *ast.SelectorExpr:
		if env != nil && !isFormalPackageSelector(e, env) && env.module != nil {
			if symbol := formalMethodObjectSymbol(e, env.module); symbol != "" {
				if sig, ok := env.module.definedFuncs[symbol]; ok {
					return sig, true
				}
			}
			if sig, ok := lookupFormalDefinedFuncSig(env.module, formalMethodBaseSymbol(env.module, e.Sel.Name)); ok {
				return sig, true
			}
		}
	case *ast.FuncLit:
		return formalFuncSigFromType(e.Type, env.module), true
	}
	return parseFormalFuncType(inferFormalExprType(expr, env))
}

func emitFormalCallExpr(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string) {
	if isMakeBuiltin(call) {
		return emitFormalMakeCall(call, env)
	}
	if value, ty, prelude, ok := emitFormalBuiltinCall(call, hintedTy, env); ok {
		return value, ty, prelude
	}
	if isFormalTypeConversionCall(call, env.module) {
		targetTy := formalTypeExprToMLIR(call.Fun, env.module)
		value, valueTy, prelude := emitFormalExpr(call.Args[0], targetTy, env)
		if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, valueTy, targetTy, env); ok {
			return coercedValue, coercedTy, prelude + coercedPrelude
		}
		todoValue, todoTy, todoPrelude := emitFormalTodoValue("type_conversion", targetTy, env)
		return todoValue, todoTy, prelude + todoPrelude
	}
	if value, ty, prelude, ok := emitFormalStdlibCall(call, hintedTy, env); ok {
		return value, ty, prelude
	}

	argHints := []string(nil)
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	if value, ty, prelude, ok := emitFormalMethodCallExpr(call, hintedTy, env, argHints); ok {
		return value, ty, prelude
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	var buf strings.Builder
	buf.WriteString(prelude)

	resultTy := inferFormalCallResultType(call, hintedTy, env)
	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, []string{resultTy}, env); ok {
		tmp := env.temp("call")
		buf.WriteString(emitFormalLinef(call, env, "    %s = func.call @%s(%s) : (%s) -> %s", tmp, symbol, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
		return tmp, resultTy, buf.String()
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok {
		return emitFormalTodoValue("indirect_call", normalizeFormalType(hintedTy), env)
	}
	if len(sig.results) > 1 {
		return emitFormalTodoValue("indirect_call_multi_result", normalizeFormalType(resultTy), env)
	}

	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	buf.WriteString(calleePrelude)
	tmp := env.temp("call")
	buf.WriteString(emitFormalLinef(call, env, "    %s = func.call_indirect %s(%s) : (%s) -> %s", tmp, calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
	return tmp, resultTy, buf.String()
}

func emitFormalMakeCall(call *ast.CallExpr, env *formalEnv) (string, string, string) {
	targetTy := formalTypeExprToMLIR(call.Args[0], env.module)
	if !strings.HasPrefix(targetTy, "!go.slice<") {
		return emitFormalRuntimeMakeHelper(call.Args[1:], targetTy, env)
	}
	if len(call.Args) < 2 {
		return emitFormalTodoValue("make_missing_len", formalOpaqueType("make"), env)
	}
	length, lengthTy, lengthPrelude := emitFormalExpr(call.Args[1], formalTargetIntType(env.module), env)
	capacity := length
	capacityPrelude := ""
	if len(call.Args) > 2 {
		capacity, _, capacityPrelude = emitFormalExpr(call.Args[2], lengthTy, env)
	}
	tmp := env.temp("make")
	var buf strings.Builder
	buf.WriteString(lengthPrelude)
	buf.WriteString(capacityPrelude)
	buf.WriteString(emitFormalLinef(call, env, "    %s = go.make_slice %s, %s : %s to %s", tmp, length, capacity, lengthTy, targetTy))
	return tmp, targetTy, buf.String()
}

func emitFormalCallStmt(call *ast.CallExpr, env *formalEnv) (string, bool) {
	if isMakeBuiltin(call) || isNewBuiltin(call) {
		value, ty, prelude := emitFormalCallExpr(call, "", env)
		return prelude + emitFormalLinef(call, env, "    go.todo %q", "discarded_"+sanitizeName(ty)+"_"+sanitizeName(value)), true
	}
	if isFormalTypeConversionCall(call, env.module) {
		return emitFormalLinef(call, env, "    go.todo %q", "type_conversion_stmt"), true
	}
	if text, ok := emitFormalStdlibCallStmt(call, env); ok {
		return text, true
	}
	argHints := []string(nil)
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	if text, ok := emitFormalMethodCallStmt(call, env, argHints); ok {
		return text, true
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	var buf strings.Builder
	buf.WriteString(prelude)

	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, nil, env); ok {
		buf.WriteString(emitFormalLinef(call, env, "    func.call @%s(%s) : (%s) -> ()", symbol, strings.Join(args, ", "), strings.Join(argTys, ", ")))
		return buf.String(), true
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok {
		return emitFormalLinef(call, env, "    go.todo %q", "indirect_call_stmt"), true
	}
	if len(sig.results) != 0 {
		return emitFormalLinef(call, env, "    go.todo %q", "discarded_call_result"), true
	}
	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	buf.WriteString(calleePrelude)
	buf.WriteString(emitFormalLinef(call, env, "    func.call_indirect %s(%s) : (%s) -> ()", calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", ")))
	return buf.String(), true
}

func isMakeBuiltin(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "make" && len(call.Args) > 0
}

func isNewBuiltin(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "new" && len(call.Args) > 0
}

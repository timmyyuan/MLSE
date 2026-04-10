package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"
)

type formalCapturedValue struct {
	name  string
	value string
	ty    string
}

type formalCapturedCallInfo struct {
	captures []formalCapturedValue
	mutated  []string
}

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

func emitFormalImmediateCapturedFuncLitCall(call *ast.CallExpr, env *formalEnv) ([]string, []string, string, bool) {
	lit, ok := call.Fun.(*ast.FuncLit)
	if !ok || env == nil || env.module == nil {
		return nil, nil, "", false
	}

	info, ok := collectFormalCapturedFuncLitCallInfo(lit, env)
	if !ok || len(info.captures) == 0 {
		return nil, nil, "", false
	}
	if len(info.mutated) != 0 {
		return emitFormalImmediateMutatingCapturedFuncLitCall(call, lit, info, env)
	}

	sig := formalFuncSigFromType(lit.Type, env.module)
	argHints := []string(nil)
	if len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)

	captureValues := make([]string, 0, len(info.captures))
	captureTys := make([]string, 0, len(info.captures))
	for _, capture := range info.captures {
		captureValues = append(captureValues, capture.value)
		captureTys = append(captureTys, capture.ty)
	}

	symbol := reserveFormalFuncLitSymbol(env.module, formalFuncSig{
		params:  append(append([]string(nil), captureTys...), sig.params...),
		results: append([]string(nil), sig.results...),
	}, env.currentFunc)
	addFormalGeneratedFunc(env.module, emitFormalCapturedFuncLitBody(symbol, lit, info.captures, env.module))

	base := env.temp("call")
	var buf strings.Builder
	buf.WriteString(prelude)
	buf.WriteString(formatFormalMultiResultCall(formalMultiResultCallSpec{
		base:      base,
		callee:    symbol,
		args:      append(captureValues, args...),
		argTys:    append(captureTys, argTys...),
		resultTys: sig.results,
	}))
	return formalCallMultiResultRefs(base, sig.results), append([]string(nil), sig.results...), buf.String(), true
}

func collectFormalCapturedFuncLitCallInfo(lit *ast.FuncLit, env *formalEnv) (formalCapturedCallInfo, bool) {
	captureNames := formalFuncLitCaptures(lit, env)
	if len(captureNames) == 0 {
		return formalCapturedCallInfo{}, false
	}
	mutated := formalFuncLitMutatedCaptureNames(lit, captureNames)

	captures := make([]formalCapturedValue, 0, len(captureNames))
	for _, name := range captureNames {
		ty := env.typeOf(name)
		if isFormalOpaquePlaceholderType(ty) {
			return formalCapturedCallInfo{}, false
		}
		captures = append(captures, formalCapturedValue{
			name:  name,
			value: env.use(name),
			ty:    ty,
		})
	}
	return formalCapturedCallInfo{
		captures: captures,
		mutated:  mutated,
	}, true
}

func formalFuncLitMutatedCaptureNames(lit *ast.FuncLit, captureNames []string) []string {
	if lit == nil || lit.Body == nil || len(captureNames) == 0 {
		return nil
	}

	captured := make(map[string]struct{}, len(captureNames))
	for _, name := range captureNames {
		captured[name] = struct{}{}
	}

	mutated := make(map[string]struct{})
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case nil:
			return false
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			for _, lhs := range node.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok {
					continue
				}
				if _, ok := captured[ident.Name]; ok {
					mutated[ident.Name] = struct{}{}
				}
			}
		case *ast.IncDecStmt:
			ident, ok := node.X.(*ast.Ident)
			if ok {
				if _, ok := captured[ident.Name]; ok {
					mutated[ident.Name] = struct{}{}
				}
			}
		case *ast.RangeStmt:
			for _, expr := range []ast.Expr{node.Key, node.Value} {
				ident, ok := expr.(*ast.Ident)
				if !ok {
					continue
				}
				if _, ok := captured[ident.Name]; ok && node.Tok != token.DEFINE {
					mutated[ident.Name] = struct{}{}
				}
			}
		}
		return true
	})
	if len(mutated) == 0 {
		return nil
	}
	names := make([]string, 0, len(mutated))
	for name := range mutated {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func emitFormalImmediateMutatingCapturedFuncLitCall(call *ast.CallExpr, lit *ast.FuncLit, info formalCapturedCallInfo, env *formalEnv) ([]string, []string, string, bool) {
	if lit == nil || lit.Body == nil || len(info.mutated) == 0 {
		return nil, nil, "", false
	}

	sig := formalFuncSigFromType(lit.Type, env.module)
	argHints := []string(nil)
	if len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)

	workEnv := env.clone()
	restoreNode := workEnv.pushNode(lit)
	defer restoreNode()
	workEnv.resultTypes = append([]string(nil), sig.results...)
	for _, capture := range info.captures {
		workEnv.bindValue(capture.name, capture.value, capture.ty)
	}
	if !bindFormalFuncLitCallArgs(lit.Type.Params, args, argTys, workEnv) {
		return nil, nil, "", false
	}

	prefix, retExprs, ok := extractTrailingReturnExprs(lit.Body.List)
	if !ok {
		return nil, nil, "", false
	}
	bodyText, terminated := emitFormalRegionBlock(prefix, workEnv)
	if terminated {
		return nil, nil, "", false
	}
	retValues, retTypes, retPrelude, ok := emitFormalReturnExprOperands(retExprs, sig.results, workEnv)
	if !ok {
		return nil, nil, "", false
	}

	mutated := make([]formalCapturedValue, 0, len(info.mutated))
	for _, name := range info.mutated {
		ty := workEnv.typeOf(name)
		if isFormalOpaquePlaceholderType(ty) {
			return nil, nil, "", false
		}
		mutated = append(mutated, formalCapturedValue{
			name:  name,
			value: workEnv.use(name),
			ty:    ty,
		})
	}

	resultTypes := append([]string(nil), retTypes...)
	resultValues := append([]string(nil), retValues...)
	for _, capture := range mutated {
		resultTypes = append(resultTypes, capture.ty)
		resultValues = append(resultValues, capture.value)
	}

	base := env.temp("call")
	var buf strings.Builder
	buf.WriteString(prelude)
	var region strings.Builder
	region.WriteString(fmt.Sprintf("    %s = scf.execute_region -> (%s) {\n", formalIfResultBinding(base, len(resultTypes)), strings.Join(resultTypes, ", ")))
	region.WriteString(indentBlock(bodyText, 1))
	region.WriteString(indentBlock(retPrelude, 1))
	region.WriteString(emitFormalLinef(lit, env, "      scf.yield %s : %s", strings.Join(resultValues, ", "), strings.Join(resultTypes, ", ")))
	region.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(region.String(), lit, env))

	refs := formalMultiResultRefs(base, len(resultTypes))
	for i, capture := range mutated {
		env.bindValue(capture.name, refs[len(retTypes)+i], capture.ty)
	}
	syncFormalTempID(env, workEnv)
	return refs[:len(retTypes)], retTypes, buf.String(), true
}

func bindFormalFuncLitCallArgs(fields *ast.FieldList, values []string, types []string, env *formalEnv) bool {
	if fields == nil || len(fields.List) == 0 {
		return len(values) == 0 && len(types) == 0
	}
	index := 0
	for _, field := range fields.List {
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			if index >= len(values) || index >= len(types) {
				return false
			}
			if len(field.Names) != 0 {
				name := field.Names[i].Name
				if name != "_" {
					env.bindValue(name, values[index], types[index])
				}
			}
			index++
		}
	}
	return index == len(values) && index == len(types)
}

func emitFormalCapturedFuncLitBody(symbol string, lit *ast.FuncLit, captures []formalCapturedValue, module *formalModuleContext) string {
	env := newFormalEnv(module)
	restoreNode := env.pushNode(lit)
	defer restoreNode()
	env.currentFunc = sanitizeName(symbol)
	results := emitFormalResultTypes(lit.Type.Results, module)
	env.resultTypes = append([]string(nil), results...)

	params := make([]string, 0, len(captures))
	for _, capture := range captures {
		params = append(params, fmt.Sprintf("%s: %s", env.define(capture.name, capture.ty), capture.ty))
	}
	params = append(params, emitFormalParams(lit.Type.Params, env)...)

	var buf strings.Builder
	funcAttrs := ""
	if module != nil {
		funcAttrs = module.scopeAttrForNode(lit)
	}
	buf.WriteString(formatPrivateFuncHeaderWithAttrs(symbol, params, results, funcAttrs))

	terminated := false
	if lit.Body == nil {
		buf.WriteString(emitFormalLinef(lit, env, "    go.todo %q", "missing_body"))
	} else {
		normalizedBody := normalizeFormalTopLevelLabelsWithReserved(
			lit.Body.List,
			collectFormalReservedNames(lit.Type, lit.Body),
		)
		bodyText, term := emitFormalFuncBlock(normalizedBody, env, results)
		buf.WriteString(bodyText)
		terminated = term
	}
	if !terminated {
		if len(results) > 0 {
			buf.WriteString(emitFormalLinef(lit, env, "    go.todo %q", "implicit_return_placeholder"))
		}
		buf.WriteString(emitFormalFallbackReturn(results, env))
	}
	buf.WriteString("  }\n")
	return annotateFormalStructuredOp(buf.String(), lit, env)
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
		if formalIdentIsPackageLevelObject(ident, env.module) {
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

func formalIdentIsPackageLevelObject(ident *ast.Ident, module *formalModuleContext) bool {
	if ident == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return false
	}
	obj := module.typed.info.ObjectOf(ident)
	if obj == nil {
		return false
	}
	switch obj.(type) {
	case *types.PkgName:
		return true
	}
	pkg := obj.Pkg()
	if pkg == nil {
		return false
	}
	return obj.Parent() == pkg.Scope()
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

func formalCallActualResultType(call *ast.CallExpr, env *formalEnv) string {
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && len(sig.results) == 1 {
		return sig.results[0]
	}
	return inferFormalCallResultType(call, "", env)
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
	if values, types, prelude, ok := emitFormalImmediateCapturedFuncLitCall(call, env); ok && len(types) == 1 {
		coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(values[0], types[0], hintedTy, env)
		return coercedValue, coercedTy, prelude + coercedPrelude
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

	resultTy := formalCallActualResultType(call, env)
	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, []string{resultTy}, env); ok {
		tmp := env.temp("call")
		buf.WriteString(emitFormalLinef(call, env, "    %s = func.call @%s(%s) : (%s) -> %s", tmp, symbol, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
		coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(tmp, resultTy, hintedTy, env)
		buf.WriteString(coercedPrelude)
		return coercedValue, coercedTy, buf.String()
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
	coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(tmp, resultTy, hintedTy, env)
	buf.WriteString(coercedPrelude)
	return coercedValue, coercedTy, buf.String()
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
	if _, _, prelude, ok := emitFormalImmediateCapturedFuncLitCall(call, env); ok {
		return prelude, true
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

	resultTys := []string(nil)
	if sig, ok := formalExprFuncSig(call.Fun, env); ok {
		resultTys = append([]string(nil), sig.results...)
	} else {
		resultTy := inferFormalCallResultType(call, "", env)
		if !isFormalOpaquePlaceholderType(resultTy) {
			resultTys = []string{resultTy}
		}
	}

	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, resultTys, env); ok {
		switch len(resultTys) {
		case 0:
			buf.WriteString(emitFormalLinef(call, env, "    func.call @%s(%s) : (%s) -> ()", symbol, strings.Join(args, ", "), strings.Join(argTys, ", ")))
		case 1:
			tmp := env.temp("call")
			buf.WriteString(emitFormalLinef(call, env, "    %s = func.call @%s(%s) : (%s) -> %s", tmp, symbol, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTys[0]))
		default:
			base := env.temp("call")
			buf.WriteString(formatFormalMultiResultCall(formalMultiResultCallSpec{
				base:      base,
				callee:    symbol,
				args:      args,
				argTys:    argTys,
				resultTys: resultTys,
			}))
		}
		return buf.String(), true
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok {
		return emitFormalLinef(call, env, "    go.todo %q", "indirect_call_stmt"), true
	}
	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	buf.WriteString(calleePrelude)
	switch len(sig.results) {
	case 0:
		buf.WriteString(emitFormalLinef(call, env, "    func.call_indirect %s(%s) : (%s) -> ()", calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", ")))
	case 1:
		tmp := env.temp("call")
		buf.WriteString(emitFormalLinef(call, env, "    %s = func.call_indirect %s(%s) : (%s) -> %s", tmp, calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", "), sig.results[0]))
	default:
		base := env.temp("call")
		buf.WriteString(formatFormalMultiResultCall(formalMultiResultCallSpec{
			base:      base,
			callee:    calleeValue,
			args:      args,
			argTys:    argTys,
			resultTys: sig.results,
			indirect:  true,
		}))
	}
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

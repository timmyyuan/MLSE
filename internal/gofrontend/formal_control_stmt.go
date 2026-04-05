package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strings"
)

func emitFormalReturnStmt(s *ast.ReturnStmt, env *formalEnv, resultTypes []string) string {
	if len(s.Results) == 0 {
		return emitFormalReturnValues(resultTypes, env)
	}
	if len(resultTypes) == 0 {
		return emitFormalLinef(s, env, "    go.todo %q", "unexpected_return_value") + emitFormalLinef(s, env, "    return")
	}
	if len(s.Results) != len(resultTypes) {
		return emitFormalLinef(s, env, "    go.todo %q", "return_arity_mismatch") + emitFormalReturnValues(resultTypes, env)
	}
	if len(resultTypes) > 1 && len(s.Results) == 1 {
		return emitFormalLinef(s, env, "    go.todo %q", "multi_result_return_value") + emitFormalReturnValues(resultTypes, env)
	}

	var (
		values []string
		types  []string
		buf    strings.Builder
	)
	for i, result := range s.Results {
		hint := ""
		if i < len(resultTypes) {
			hint = resultTypes[i]
		}
		value, ty, prelude := emitFormalExpr(result, hint, env)
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
		types = append(types, ty)
	}
	buf.WriteString(emitFormalReturnLine(values, types, env))
	return buf.String()
}

func emitFormalExprStmt(s *ast.ExprStmt, env *formalEnv) string {
	if call, ok := s.X.(*ast.CallExpr); ok {
		text, ok := emitFormalCallStmt(call, env)
		if ok {
			return text
		}
	}
	_, _, prelude := emitFormalExpr(s.X, "", env)
	if prelude == "" {
		return emitFormalLinef(s, env, "    go.todo %q", "expr_stmt")
	}
	return prelude + emitFormalLinef(s, env, "    go.todo %q", "expr_stmt")
}

func emitFormalDeclStmt(s *ast.DeclStmt, env *formalEnv) string {
	gen, ok := s.Decl.(*ast.GenDecl)
	if !ok {
		return emitFormalLinef(s, env, "    go.todo %q", shortNodeName(s))
	}

	var buf strings.Builder
	for _, spec := range gen.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			buf.WriteString(emitFormalLinef(spec, env, "    go.todo %q", shortNodeName(spec)))
			continue
		}
		for i, name := range valueSpec.Names {
			if name.Name == "_" {
				continue
			}
			ty := formalOpaqueType("value")
			if valueSpec.Type != nil {
				ty = formalTypeExprToMLIR(valueSpec.Type, env.module)
			}
			if i < len(valueSpec.Values) {
				if valueSpec.Type == nil {
					ty = inferFormalExprType(valueSpec.Values[i], env)
				}
				value, valueTy, prelude := emitFormalExpr(valueSpec.Values[i], ty, env)
				buf.WriteString(prelude)
				env.bindValue(name.Name, value, valueTy)
				continue
			}
			value, prelude := emitFormalZeroValue(ty, env)
			buf.WriteString(prelude)
			env.bindValue(name.Name, value, ty)
		}
	}
	return buf.String()
}

func emitFormalIfStmt(s *ast.IfStmt, env *formalEnv) string {
	if s.Init != nil {
		return emitFormalIfStmtWithInit(s, env)
	}

	cond, prelude, ok := emitFormalCondition(s.Cond, env)
	if !ok {
		return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_condition")
	}

	thenEnv := env.clone()
	thenText, thenTerm := emitFormalRegionBlock(s.Body.List, thenEnv)
	if thenTerm {
		syncFormalTempID(env, thenEnv)
		return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_returning_region")
	}

	elseEnv := env.clone()
	elseText := ""
	hasElse := false
	if s.Else != nil {
		elseBlock, ok := s.Else.(*ast.BlockStmt)
		if !ok {
			return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_else")
		}
		hasElse = true
		var elseTerm bool
		elseText, elseTerm = emitFormalRegionBlock(elseBlock.List, elseEnv)
		if elseTerm {
			syncFormalTempID(env, thenEnv, elseEnv)
			return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_returning_region")
		}
	}

	mutated := formalMutatedOuterNames(env, thenEnv, elseEnv, hasElse)
	var buf strings.Builder
	buf.WriteString(prelude)
	switch len(mutated) {
	case 0:
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    scf.if %s {\n", cond))
		ifBuf.WriteString(indentBlock(thenText, 2))
		if hasElse {
			ifBuf.WriteString("    } else {\n")
			ifBuf.WriteString(indentBlock(elseText, 2))
		}
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
	case 1:
		name := mutated[0]
		thenValue := thenEnv.use(name)
		elseValue := env.use(name)
		ty := thenEnv.typeOf(name)
		if hasElse {
			elseValue = elseEnv.use(name)
			if ty == formalOpaqueType("value") {
				ty = elseEnv.typeOf(name)
			}
		}
		if ty == formalOpaqueType("value") {
			ty = env.typeOf(name)
		}
		if hasElse && normalizeFormalType(elseEnv.typeOf(name)) != normalizeFormalType(ty) {
			syncFormalTempID(env, thenEnv, elseEnv)
			return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_type_mismatch")
		}
		if !hasElse {
			elseText = ""
		}
		result := env.temp("if")
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", result, cond, ty))
		ifBuf.WriteString(indentBlock(thenText, 2))
		ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : %s", thenValue, ty))
		ifBuf.WriteString("    } else {\n")
		ifBuf.WriteString(indentBlock(elseText, 2))
		ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : %s", elseValue, ty))
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
		env.bindValue(name, result, ty)
	default:
		resultTypes := make([]string, 0, len(mutated))
		thenValues := make([]string, 0, len(mutated))
		elseValues := make([]string, 0, len(mutated))
		for _, name := range mutated {
			ty := thenEnv.typeOf(name)
			if hasElse {
				if ty == formalOpaqueType("value") {
					ty = elseEnv.typeOf(name)
				}
				if normalizeFormalType(elseEnv.typeOf(name)) != normalizeFormalType(ty) {
					syncFormalTempID(env, thenEnv, elseEnv)
					return prelude + emitFormalLinef(s, env, "    go.todo %q", "IfStmt_type_mismatch")
				}
			}
			if ty == formalOpaqueType("value") {
				ty = env.typeOf(name)
			}
			resultTypes = append(resultTypes, ty)
			thenValues = append(thenValues, thenEnv.use(name))
			if hasElse {
				elseValues = append(elseValues, elseEnv.use(name))
			} else {
				elseValues = append(elseValues, env.use(name))
			}
		}
		result := env.temp("if")
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
		ifBuf.WriteString(indentBlock(thenText, 2))
		ifBuf.WriteString(emitFormalYieldLine(thenValues, resultTypes, env))
		ifBuf.WriteString("    } else {\n")
		ifBuf.WriteString(indentBlock(elseText, 2))
		ifBuf.WriteString(emitFormalYieldLine(elseValues, resultTypes, env))
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
		resultValues := formalMultiResultRefs(result, len(resultTypes))
		for i, name := range mutated {
			env.bindValue(name, resultValues[i], resultTypes[i])
		}
	}
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String()
}

func emitFormalForStmt(s *ast.ForStmt, env *formalEnv) string {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return buf.String()
		}
	}

	if s.Cond == nil {
		buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt"))
		return buf.String()
	}

	ivName, upperExpr, ok := matchFormalCountedLoopCond(s.Cond)
	if !ok || len(s.Body.List) == 0 {
		buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt"))
		return buf.String()
	}

	bodyStmts := s.Body.List
	if s.Post != nil {
		if !isFormalLoopIncrement(s.Post, ivName) {
			buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt"))
			return buf.String()
		}
	} else {
		last := bodyStmts[len(bodyStmts)-1]
		if !isFormalLoopIncrement(last, ivName) {
			buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt"))
			return buf.String()
		}
		bodyStmts = bodyStmts[:len(bodyStmts)-1]
	}

	ivInit := env.use(ivName)
	ivTy := env.typeOf(ivName)
	if !isFormalIntegerType(ivTy) {
		buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt_iv_type"))
		return buf.String()
	}

	upper, upperTy, upperPrelude := emitFormalExpr(upperExpr, ivTy, env)
	if upperTy != ivTy {
		buf.WriteString(upperPrelude)
		buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "ForStmt_bound_type"))
		return buf.String()
	}

	carried := collectAssignedOuterNames(bodyStmts, env, ivName)

	lowerValue := ivInit
	upperValue := upper
	loopBoundTy := ivTy
	if ivTy != "index" {
		lowerIndex := env.temp("idx")
		upperIndex := env.temp("idx")
		upperPrelude += emitFormalLinef(s, env, "    %s = arith.index_cast %s : %s to index", lowerIndex, ivInit, ivTy)
		upperPrelude += emitFormalLinef(s, env, "    %s = arith.index_cast %s : %s to index", upperIndex, upper, ivTy)
		lowerValue = lowerIndex
		upperValue = upperIndex
		loopBoundTy = "index"
	}
	buf.WriteString(upperPrelude)

	step := env.temp("const")
	ivSSA := env.temp(sanitizeName(ivName) + "_iv")
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 1 : %s", step, loopBoundTy))

	if len(carried) == 0 {
		bodyEnv := env.clone()
		bodyPrelude := ""
		if ivTy == "index" {
			bodyEnv.bindValue(ivName, ivSSA, ivTy)
		} else {
			ivCast := bodyEnv.temp(sanitizeName(ivName) + "_body")
			bodyPrelude = emitFormalLinef(s, env, "    %s = arith.index_cast %s : index to %s", ivCast, ivSSA, ivTy)
			bodyEnv.bindValue(ivName, ivCast, ivTy)
		}
		bodyText, _, bodyTerm := emitFormalLoopBody(bodyStmts, bodyEnv, "", "")
		syncFormalTempID(env, bodyEnv)
		if bodyTerm {
			return buf.String() + emitFormalLinef(s, env, "    go.todo %q", "ForStmt_returning_body")
		}
		var forBuf strings.Builder
		forBuf.WriteString(fmt.Sprintf("    scf.for %s = %s to %s step %s {\n", ivSSA, lowerValue, upperValue, step))
		forBuf.WriteString(indentBlock(bodyPrelude, 2))
		forBuf.WriteString(indentBlock(bodyText, 2))
		forBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(forBuf.String(), s, env))
		exitIV, _, exitPrelude := emitFormalTodoValue("loop_iv_exit", ivTy, env)
		buf.WriteString(exitPrelude)
		env.bindValue(ivName, exitIV, ivTy)
		return buf.String()
	}

	carriedTys := make([]string, 0, len(carried))
	iterArgs := make([]string, 0, len(carried))
	result := env.temp("loop")
	bodyEnv := env.clone()
	bodyPrelude := ""
	if ivTy == "index" {
		bodyEnv.bindValue(ivName, ivSSA, ivTy)
	} else {
		ivCast := bodyEnv.temp(sanitizeName(ivName) + "_body")
		bodyPrelude = emitFormalLinef(s, env, "    %s = arith.index_cast %s : index to %s", ivCast, ivSSA, ivTy)
		bodyEnv.bindValue(ivName, ivCast, ivTy)
	}
	for _, name := range carried {
		ty := env.typeOf(name)
		carriedTys = append(carriedTys, ty)
		iterSSA := fmt.Sprintf("%%%s_iter", sanitizeName(name))
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", iterSSA, env.use(name)))
		bodyEnv.bindValue(name, iterSSA, ty)
	}
	bodyText, yieldValues, bodyTerm := emitFormalLoopBodyWithCarried(bodyStmts, bodyEnv, carried, carriedTys)
	syncFormalTempID(env, bodyEnv)
	if bodyTerm {
		return buf.String() + emitFormalLinef(s, env, "    go.todo %q", "ForStmt_returning_body")
	}
	var forBuf strings.Builder
	forBuf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(carriedTys)), ivSSA, lowerValue, upperValue, step, strings.Join(iterArgs, ", "), strings.Join(carriedTys, ", ")))
	forBuf.WriteString(indentBlock(bodyPrelude, 2))
	forBuf.WriteString(indentBlock(bodyText, 2))
	forBuf.WriteString(emitFormalYieldLine(yieldValues, carriedTys, env))
	forBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(forBuf.String(), s, env))
	resultValues := formalMultiResultRefs(result, len(carriedTys))
	for i, name := range carried {
		env.bindValue(name, resultValues[i], carriedTys[i])
	}
	exitIV, _, exitPrelude := emitFormalTodoValue("loop_iv_exit", ivTy, env)
	buf.WriteString(exitPrelude)
	env.bindValue(ivName, exitIV, ivTy)
	return buf.String()
}

func emitFormalRangeStmt(s *ast.RangeStmt, env *formalEnv) string {
	if s.Tok != token.DEFINE && s.Tok != token.ASSIGN {
		return emitFormalLinef(s, env, "    go.todo %q", "RangeStmt")
	}

	source, sourceTy, sourcePrelude := emitFormalExpr(s.X, "", env)
	lengthTmp, lengthPrelude, ok := emitFormalGoLenValue(source, sourceTy, formalTargetIntType(env.module), "rangelen", env)
	if !ok {
		lengthTmp, lengthPrelude = emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeRangeLenSymbol(sourceTy).String(),
				args:       []string{source},
				argTys:     []string{sourceTy},
				resultTy:   formalTargetIntType(env.module),
				tempPrefix: "rangelen",
			},
			env,
		)
	}

	lower := env.temp("idx")
	upper := env.temp("idx")
	step := env.temp("const")
	ivSSA := env.temp("range_iv")
	var buf strings.Builder
	buf.WriteString(sourcePrelude)
	buf.WriteString(lengthPrelude)
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 0 : index", lower))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.index_cast %s : %s to index", upper, lengthTmp, formalTargetIntType(env.module)))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 1 : index", step))

	keyName := rangeKeyName(s.Key)
	valueName := rangeKeyName(s.Value)
	excludes := make(map[string]struct{})
	for _, name := range []string{keyName, valueName} {
		if name != "" {
			excludes[name] = struct{}{}
		}
	}
	carried := collectAssignedOuterNamesWithExcludes(s.Body.List, env, excludes)

	if len(carried) == 0 {
		bodyEnv := env.clone()
		bodyPrelude := emitFormalRangeBindings(s, source, sourceTy, ivSSA, bodyEnv)
		bodyText, _, bodyTerm := emitFormalLoopBody(s.Body.List, bodyEnv, "", "")
		syncFormalTempID(env, bodyEnv)
		if bodyTerm {
			return buf.String() + emitFormalLinef(s, env, "    go.todo %q", "RangeStmt_returning_body")
		}
		var rangeBuf strings.Builder
		rangeBuf.WriteString(fmt.Sprintf("    scf.for %s = %s to %s step %s {\n", ivSSA, lower, upper, step))
		rangeBuf.WriteString(indentBlock(bodyPrelude, 2))
		rangeBuf.WriteString(indentBlock(bodyText, 2))
		rangeBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(rangeBuf.String(), s, env))
		return buf.String()
	}

	carriedTys := make([]string, 0, len(carried))
	iterArgs := make([]string, 0, len(carried))
	result := env.temp("range")
	bodyEnv := env.clone()
	for _, name := range carried {
		ty := env.typeOf(name)
		carriedTys = append(carriedTys, ty)
		iterSSA := fmt.Sprintf("%%%s_iter", sanitizeName(name))
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", iterSSA, env.use(name)))
		bodyEnv.bindValue(name, iterSSA, ty)
	}
	bodyPrelude := emitFormalRangeBindings(s, source, sourceTy, ivSSA, bodyEnv)
	bodyText, yieldValues, bodyTerm := emitFormalLoopBodyWithCarried(s.Body.List, bodyEnv, carried, carriedTys)
	syncFormalTempID(env, bodyEnv)
	if bodyTerm {
		return buf.String() + emitFormalLinef(s, env, "    go.todo %q", "RangeStmt_returning_body")
	}
	var rangeBuf strings.Builder
	rangeBuf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(carriedTys)), ivSSA, lower, upper, step, strings.Join(iterArgs, ", "), strings.Join(carriedTys, ", ")))
	rangeBuf.WriteString(indentBlock(bodyPrelude, 2))
	rangeBuf.WriteString(indentBlock(bodyText, 2))
	rangeBuf.WriteString(emitFormalYieldLine(yieldValues, carriedTys, env))
	rangeBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(rangeBuf.String(), s, env))
	resultValues := formalMultiResultRefs(result, len(carriedTys))
	for i, name := range carried {
		env.bindValue(name, resultValues[i], carriedTys[i])
	}
	return buf.String()
}

func emitFormalRangeBindings(s *ast.RangeStmt, source string, sourceTy string, ivSSA string, env *formalEnv) string {
	var buf strings.Builder
	indexValue := ivSSA
	indexTy := "index"
	if keyName := rangeKeyName(s.Key); keyName != "" {
		keyTy := formalTargetIntType(env.module)
		if s.Tok == token.ASSIGN {
			keyTy = chooseFormalCommonType(env.typeOf(keyName), formalTargetIntType(env.module))
		}
		boundIndex := indexValue
		if keyTy != "index" {
			cast := env.temp("range_idx")
			buf.WriteString(emitFormalLinef(s, env, "    %s = arith.index_cast %s : index to %s", cast, ivSSA, keyTy))
			boundIndex = cast
			indexTy = keyTy
		}
		if s.Tok == token.DEFINE {
			env.defineOrAssign(keyName, keyTy)
		} else {
			env.assign(keyName, keyTy)
		}
		env.bindValue(keyName, boundIndex, keyTy)
	}
	if valueName := rangeKeyName(s.Value); valueName != "" {
		valueTy := inferFormalRangeValueType(valueName, sourceTy, s.Body, env)
		indexArg := indexValue
		if indexTy != "index" {
			indexArg = ivSSA
		}
		valueTmp, _, valuePrelude, ok := emitFormalIndexedReadValue(formalGoIndexSpec{
			source:     source,
			sourceTy:   sourceTy,
			index:      indexArg,
			indexTy:    "index",
			hintedTy:   valueTy,
			tempPrefix: "rangeval",
		}, env)
		if !ok {
			valueTmp, valuePrelude = emitFormalHelperCall(
				formalHelperCallSpec{
					base:       formalRuntimeIndexSymbol(sourceTy).String(),
					args:       []string{source, indexArg},
					argTys:     []string{sourceTy, "index"},
					resultTy:   valueTy,
					tempPrefix: "rangeval",
				},
				env,
			)
		}
		buf.WriteString(valuePrelude)
		if s.Tok == token.DEFINE {
			env.defineOrAssign(valueName, valueTy)
		} else {
			env.assign(valueName, valueTy)
		}
		env.bindValue(valueName, valueTmp, valueTy)
	}
	return buf.String()
}

func rangeKeyName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return ""
	}
	return ident.Name
}

func emitFormalIncDecStmt(s *ast.IncDecStmt, env *formalEnv) string {
	ident, ok := s.X.(*ast.Ident)
	if !ok {
		return emitFormalLinef(s, env, "    go.todo %q", shortNodeName(s))
	}
	name := env.use(ident.Name)
	ty := env.typeOf(ident.Name)
	if !isFormalIntegerType(ty) {
		return emitFormalLinef(s, env, "    go.todo %q", "incdec_non_integer")
	}
	step := env.temp("const")
	next := env.temp("inc")
	env.assign(ident.Name, ty)
	op := "arith.addi"
	if s.Tok == token.DEC {
		op = "arith.subi"
	}
	env.bindValue(ident.Name, next, ty)
	return emitFormalLinef(s, env, "    %s = arith.constant 1 : %s", step, ty) +
		emitFormalLinef(s, env, "    %s = %s %s, %s : %s", next, op, name, step, ty)
}

func formalMutatedOuterNames(base *formalEnv, thenEnv *formalEnv, elseEnv *formalEnv, hasElse bool) []string {
	names := make([]string, 0)
	for name, binding := range base.locals {
		thenBinding, ok := thenEnv.locals[name]
		if !ok {
			continue
		}
		elseBinding := binding
		if hasElse {
			var ok bool
			elseBinding, ok = elseEnv.locals[name]
			if !ok {
				continue
			}
		}
		if binding.current != thenBinding.current || binding.current != elseBinding.current || binding.ty != thenBinding.ty || binding.ty != elseBinding.ty {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func collectAssignedOuterNames(stmts []ast.Stmt, env *formalEnv, exclude string) []string {
	excludes := make(map[string]struct{})
	if exclude != "" {
		excludes[exclude] = struct{}{}
	}
	return collectAssignedOuterNamesWithExcludes(stmts, env, excludes)
}

func collectAssignedOuterNamesWithExcludes(stmts []ast.Stmt, env *formalEnv, excludes map[string]struct{}) []string {
	seen := make(map[string]struct{})
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				if _, skip := excludes[ident.Name]; skip {
					continue
				}
				if _, ok := env.locals[ident.Name]; ok {
					seen[ident.Name] = struct{}{}
				}
			}
		case *ast.IncDecStmt:
			ident, ok := s.X.(*ast.Ident)
			if !ok || ident.Name == "_" {
				continue
			}
			if _, skip := excludes[ident.Name]; skip {
				continue
			}
			if _, ok := env.locals[ident.Name]; ok {
				seen[ident.Name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func matchFormalCountedLoopCond(expr ast.Expr) (string, ast.Expr, bool) {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok || binary.Op != token.LSS {
		return "", nil, false
	}
	ident, ok := binary.X.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return "", nil, false
	}
	return ident.Name, binary.Y, true
}

func isFormalLoopIncrement(stmt ast.Stmt, ivName string) bool {
	switch s := stmt.(type) {
	case *ast.IncDecStmt:
		ident, ok := s.X.(*ast.Ident)
		return ok && ident.Name == ivName && s.Tok == token.INC
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 || s.Tok != token.ASSIGN {
			return false
		}
		lhs, ok := s.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != ivName {
			return false
		}
		binary, ok := s.Rhs[0].(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			return false
		}
		x, ok := binary.X.(*ast.Ident)
		if !ok || x.Name != ivName {
			return false
		}
		lit, ok := binary.Y.(*ast.BasicLit)
		return ok && lit.Kind == token.INT && lit.Value == "1"
	default:
		return false
	}
}

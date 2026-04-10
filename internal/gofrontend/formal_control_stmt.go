package gofrontend

import (
	"fmt"
	"go/ast"
	goconstant "go/constant"
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
				boolConst := ""
				if valueBool, ok := formalKnownBoolExpr(valueSpec.Values[i], env); ok {
					if valueBool {
						boolConst = "true"
					} else {
						boolConst = "false"
					}
				}
				env.bindValueWithBool(name.Name, value, valueTy, boolConst)
				continue
			}
			value, prelude := emitFormalZeroValue(ty, env)
			buf.WriteString(prelude)
			boolConst := ""
			if normalizeFormalType(ty) == "i1" {
				boolConst = "false"
			}
			env.bindValueWithBool(name.Name, value, ty, boolConst)
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

func emitFormalForStmt(s *ast.ForStmt, remaining []ast.Stmt, env *formalEnv) string {
	if text, ok := emitFormalForStmtAttempt(env, func(attemptEnv *formalEnv) (string, bool) {
		return emitFormalZeroTripForStmt(s, attemptEnv)
	}); ok {
		return text
	}
	if text, ok := emitFormalForStmtAttempt(env, func(attemptEnv *formalEnv) (string, bool) {
		return emitFormalCountedForStmt(s, remaining, attemptEnv)
	}); ok {
		return text
	}
	if text, ok := emitFormalForStmtAttempt(env, func(attemptEnv *formalEnv) (string, bool) {
		return emitFormalWhileForStmt(s, attemptEnv)
	}); ok {
		return text
	}
	return emitFormalLinef(s, env, "    go.todo %q", "ForStmt")
}

func emitFormalForStmtAttempt(env *formalEnv, lower func(*formalEnv) (string, bool)) (string, bool) {
	if env == nil || lower == nil {
		return "", false
	}
	attemptEnv := env.clone()
	text, ok := lower(attemptEnv)
	if !ok {
		return "", false
	}
	propagateFormalOuterBindings(env, attemptEnv)
	syncFormalTempID(env, attemptEnv)
	return text, true
}

func emitFormalCountedForStmt(s *ast.ForStmt, remaining []ast.Stmt, env *formalEnv) (string, bool) {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return buf.String(), true
		}
	}

	if s.Cond == nil {
		return "", false
	}

	loopSpec, ok := matchFormalCountedLoopCond(s.Cond, env.module)
	if !ok {
		return "", false
	}

	bodyStmts := s.Body.List
	if formalLoopContainsUnsupportedBranch(bodyStmts) {
		return "", false
	}
	if s.Post != nil {
		step, ok := matchFormalCountedLoopStep(s.Post, loopSpec.ivName, env.module)
		if !ok || step != loopSpec.step {
			return "", false
		}
	} else {
		if len(bodyStmts) == 0 {
			return "", false
		}
		last := bodyStmts[len(bodyStmts)-1]
		step, ok := matchFormalCountedLoopStep(last, loopSpec.ivName, env.module)
		if !ok || step != loopSpec.step {
			return "", false
		}
		bodyStmts = bodyStmts[:len(bodyStmts)-1]
	}

	ivInit := env.use(loopSpec.ivName)
	ivTy := env.typeOf(loopSpec.ivName)
	if !isFormalIntegerType(ivTy) {
		return "", false
	}

	bound, boundTy, boundPrelude := emitFormalExpr(loopSpec.boundExpr, ivTy, env)
	if boundTy != ivTy {
		return "", false
	}
	loopValues := formalCountedLoopValues{init: ivInit, bound: bound, ty: ivTy}

	lowering, tripPrelude, ok := emitFormalCountedLoopTripCount(s, loopValues, loopSpec, env)
	if !ok || lowering.initIndex == "" || lowering.tripCount == "" {
		return "", false
	}

	carried := collectAssignedOuterNames(bodyStmts, env, loopSpec.ivName)

	buf.WriteString(boundPrelude)
	buf.WriteString(tripPrelude)

	lowerValue := env.temp("idx")
	step := env.temp("const")
	iterSSA := env.temp(sanitizeName(loopSpec.ivName) + "_iter")
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 0 : index", lowerValue))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 1 : index", step))

	if len(carried) == 0 {
		bodyEnv := env.clone()
		bodyPrelude, ok := emitFormalCountedLoopBodyIV(s, iterSSA, lowering, loopSpec, bodyEnv)
		if !ok {
			return "", false
		}
		bodyText, _, bodyTerm := emitFormalLoopBody(bodyStmts, bodyEnv, "", "")
		syncFormalTempID(env, bodyEnv)
		if bodyTerm {
			return "", false
		}
		var forBuf strings.Builder
		forBuf.WriteString(fmt.Sprintf("    scf.for %s = %s to %s step %s {\n", iterSSA, lowerValue, lowering.tripCount, step))
		forBuf.WriteString(indentBlock(bodyPrelude, 2))
		forBuf.WriteString(indentBlock(bodyText, 2))
		forBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(forBuf.String(), s, env))
		if formalStmtListUsesIdent(remaining, loopSpec.ivName) {
			exitIV, exitPrelude := emitFormalCountedLoopExitIV(s, loopValues, loopSpec, env)
			buf.WriteString(exitPrelude)
			env.bindValue(loopSpec.ivName, exitIV, ivTy)
		}
		return buf.String(), true
	}

	carriedTys := make([]string, 0, len(carried))
	iterArgs := make([]string, 0, len(carried))
	result := env.temp("loop")
	bodyEnv := env.clone()
	bodyPrelude, ok := emitFormalCountedLoopBodyIV(s, iterSSA, lowering, loopSpec, bodyEnv)
	if !ok {
		return "", false
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
		return "", false
	}
	var forBuf strings.Builder
	forBuf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(carriedTys)), iterSSA, lowerValue, lowering.tripCount, step, strings.Join(iterArgs, ", "), strings.Join(carriedTys, ", ")))
	forBuf.WriteString(indentBlock(bodyPrelude, 2))
	forBuf.WriteString(indentBlock(bodyText, 2))
	forBuf.WriteString(emitFormalYieldLine(yieldValues, carriedTys, env))
	forBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(forBuf.String(), s, env))
	resultValues := formalMultiResultRefs(result, len(carriedTys))
	for i, name := range carried {
		env.bindValue(name, resultValues[i], carriedTys[i])
	}
	if formalStmtListUsesIdent(remaining, loopSpec.ivName) {
		exitIV, exitPrelude := emitFormalCountedLoopExitIV(s, loopValues, loopSpec, env)
		buf.WriteString(exitPrelude)
		env.bindValue(loopSpec.ivName, exitIV, ivTy)
	}
	return buf.String(), true
}

func emitFormalWhileForStmt(s *ast.ForStmt, env *formalEnv) (string, bool) {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return buf.String(), true
		}
	}
	if s.Cond == nil {
		return "", false
	}

	carriedScan := append([]ast.Stmt{}, s.Body.List...)
	if s.Post != nil {
		carriedScan = append(carriedScan, s.Post)
	}
	carried := collectAssignedOuterNamesDeep(carriedScan, env, "")
	carriedTys := make([]string, 0, len(carried))
	iterArgs := make([]string, 0, len(carried)+1)
	condValues := make([]string, 0, len(carried)+1)
	bodyArgs := make([]string, 0, len(carried)+1)
	iterSSAs := make([]string, 0, len(carried))
	bodySSAs := make([]string, 0, len(carried))
	stopInit := env.temp("loopstop")
	stopIter := env.temp("loop_stop_iter")
	stopBody := env.temp("loop_stop_body")
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant false", stopInit))
	iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", stopIter, stopInit))
	condValues = append(condValues, stopIter)
	bodyArgs = append(bodyArgs, fmt.Sprintf("%s: i1", stopBody))
	for _, name := range carried {
		ty := env.typeOf(name)
		carriedTys = append(carriedTys, ty)
		iterSSA := env.temp(sanitizeName(name) + "_iter")
		bodySSA := env.temp(sanitizeName(name) + "_body")
		iterSSAs = append(iterSSAs, iterSSA)
		bodySSAs = append(bodySSAs, bodySSA)
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", iterSSA, env.use(name)))
		condValues = append(condValues, iterSSA)
		bodyArgs = append(bodyArgs, fmt.Sprintf("%s: %s", bodySSA, ty))
	}
	condEnv := env.clone()
	bodyEnv := env.clone()
	condEnv.bindValue(stopIter, stopIter, "i1")
	for i, name := range carried {
		condEnv.bindValue(name, iterSSAs[i], carriedTys[i])
		bodyEnv.bindValue(name, bodySSAs[i], carriedTys[i])
	}

	cond, condPrelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		return "", false
	}
	syncFormalTempID(bodyEnv, condEnv)
	bodyText, yieldValues, bodyStop, bodyTerm := emitFormalLoopBodyWithBreak(s.Body.List, bodyEnv, carried, carriedTys)
	if bodyTerm {
		return "", false
	}
	postEnv := env.clone()
	syncFormalTempID(postEnv, condEnv, bodyEnv)
	for i, name := range carried {
		if i < len(yieldValues) {
			postEnv.bindValue(name, yieldValues[i], carriedTys[i])
		}
	}
	postText := ""
	postYield := append([]string(nil), yieldValues...)
	postYieldPrelude := ""
	if s.Post != nil {
		postStmtText, term := emitFormalStmt(s.Post, postEnv, nil)
		if term {
			return "", false
		}
		postText = postStmtText
		postYield, postYieldPrelude, ok = coerceFormalLoopCarriedValues(postEnv, carried, carriedTys)
		if !ok {
			return "", false
		}
	}
	continueFlag := env.temp("loop_continue_flag")
	continueCond := env.temp("loop_continue")
	loopCond := env.temp("loop_cond")

	var whileBuf strings.Builder
	resultTypes := append([]string{"i1"}, carriedTys...)
	result := env.temp("loop")
	whileBuf.WriteString(fmt.Sprintf("    %s = scf.while (%s) : (%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString(indentBlock(condPrelude, 1))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.constant true", continueFlag))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.xori %s, %s : i1", continueCond, stopIter, continueFlag))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.andi %s, %s : i1", loopCond, continueCond, cond))
	whileBuf.WriteString(emitFormalLinef(s, env, "      scf.condition(%s) %s : %s", loopCond, strings.Join(condValues, ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString("    } do {\n")
	whileBuf.WriteString(fmt.Sprintf("    ^bb0(%s):\n", strings.Join(bodyArgs, ", ")))
	whileBuf.WriteString(indentBlock(bodyText, 1))
	postResult := env.temp("loopstep")
	whileBuf.WriteString(fmt.Sprintf("      %s = scf.if %s -> (%s) {\n", formalIfResultBinding(postResult, len(resultTypes)), bodyStop, strings.Join(resultTypes, ", ")))
	whileBuf.WriteString(emitFormalYieldLine(append([]string{bodyStop}, yieldValues...), resultTypes, env))
	whileBuf.WriteString("      } else {\n")
	whileBuf.WriteString(indentBlock(postText, 2))
	whileBuf.WriteString(indentBlock(postYieldPrelude, 2))
	whileBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : %s", strings.Join(append([]string{stopInit}, postYield...), ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString("      }\n")
	postRefs := formalMultiResultRefs(postResult, len(resultTypes))
	whileBuf.WriteString(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(postRefs, ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(whileBuf.String(), s, env))
	resultValues := formalMultiResultRefs(result, len(resultTypes))
	for i, name := range carried {
		env.bindValue(name, resultValues[i+1], carriedTys[i])
	}
	syncFormalTempID(env, condEnv, bodyEnv, postEnv)
	return buf.String(), true
}

func coerceFormalLoopCarriedValues(env *formalEnv, carriedNames []string, carriedTys []string) ([]string, string, bool) {
	if len(carriedNames) != len(carriedTys) {
		return nil, "", false
	}
	values := make([]string, 0, len(carriedNames))
	var buf strings.Builder
	for i, name := range carriedNames {
		value := env.use(name)
		valueTy := env.typeOf(name)
		targetTy := carriedTys[i]
		if normalizeFormalType(valueTy) != normalizeFormalType(targetTy) {
			coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, valueTy, targetTy, env)
			if !ok || normalizeFormalType(coercedTy) != normalizeFormalType(targetTy) {
				return nil, "", false
			}
			buf.WriteString(coercedPrelude)
			value = coercedValue
		}
		values = append(values, value)
	}
	return values, buf.String(), true
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
	hint := formalAssignTargetType(s.X, env)
	currentValue, ty, prelude := emitFormalExpr(s.X, hint, env)
	if !isFormalIntegerType(ty) {
		return prelude + emitFormalLinef(s, env, "    go.todo %q", "incdec_non_integer")
	}
	step := env.temp("const")
	next := env.temp("inc")
	op := "arith.addi"
	if s.Tok == token.DEC {
		op = "arith.subi"
	}
	assignText, ok := emitFormalAssignTargetValue(s.X, next, ty, env)
	if !ok {
		return prelude +
			emitFormalLinef(s, env, "    %s = arith.constant 1 : %s", step, ty) +
			emitFormalLinef(s, env, "    %s = %s %s, %s : %s", next, op, currentValue, step, ty) +
			emitFormalLinef(s, env, "    go.todo %q", shortNodeName(s))
	}
	return prelude +
		emitFormalLinef(s, env, "    %s = arith.constant 1 : %s", step, ty) +
		emitFormalLinef(s, env, "    %s = %s %s, %s : %s", next, op, currentValue, step, ty) +
		assignText
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

func collectAssignedOuterNamesDeep(stmts []ast.Stmt, env *formalEnv, exclude string) []string {
	excludes := make(map[string]struct{})
	if exclude != "" {
		excludes[exclude] = struct{}{}
	}
	seen := make(map[string]struct{})
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch node := n.(type) {
			case nil:
				return false
			case *ast.FuncLit:
				return false
			case *ast.AssignStmt:
				for _, lhs := range node.Lhs {
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
				ident, ok := node.X.(*ast.Ident)
				if !ok || ident.Name == "_" {
					return true
				}
				if _, skip := excludes[ident.Name]; skip {
					return true
				}
				if _, ok := env.locals[ident.Name]; ok {
					seen[ident.Name] = struct{}{}
				}
			}
			return true
		})
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
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

type formalCountedLoopSpec struct {
	ivName    string
	boundExpr ast.Expr
	step      int
	inclusive bool
	unsigned  bool
}

type formalCountedLoopValues struct {
	init  string
	bound string
	ty    string
}

type formalCountedLoopLowering struct {
	values    formalCountedLoopValues
	initIndex string
	tripCount string
}

type formalForFirstIterationStatus int

const (
	formalForFirstIterationUnknown formalForFirstIterationStatus = iota
	formalForFirstIterationNever
	formalForFirstIterationAlways
)

func emitFormalZeroTripForStmt(s *ast.ForStmt, env *formalEnv) (string, bool) {
	if formalClassifyForFirstIteration(s, env.module) != formalForFirstIterationNever {
		return "", false
	}
	if s == nil {
		return "", false
	}
	if s.Init == nil {
		return "", false
	}
	text, term := emitFormalStmt(s.Init, env, nil)
	if term {
		return text, true
	}
	return text, true
}

func formalClassifyForFirstIteration(s *ast.ForStmt, module *formalModuleContext) formalForFirstIterationStatus {
	if s == nil || s.Cond == nil {
		return formalForFirstIterationUnknown
	}

	initExpr, ivExpr, ok := formalSimpleForInitExpr(s.Init)
	if !ok {
		return formalForFirstIterationUnknown
	}

	binary, ok := formalUnparenExpr(s.Cond).(*ast.BinaryExpr)
	if !ok {
		return formalForFirstIterationUnknown
	}

	ivComparable := formalCountedLoopComparableExpr(ivExpr, module)
	condX := formalCountedLoopComparableExpr(binary.X, module)
	condY := formalCountedLoopComparableExpr(binary.Y, module)
	compareOp := binary.Op
	boundExpr := condY
	switch {
	case formalExprStructuralEqual(condX, ivComparable):
	case formalExprStructuralEqual(condY, ivComparable):
		compareOp, ok = formalReverseComparisonOp(binary.Op)
		if !ok {
			return formalForFirstIterationUnknown
		}
		boundExpr = condX
	default:
		return formalForFirstIterationUnknown
	}

	initValue, _, ok := formalTypedConstValue(formalCountedLoopComparableExpr(initExpr, module), module)
	if !ok || initValue == nil {
		return formalForFirstIterationUnknown
	}
	boundValue, _, ok := formalTypedConstValue(formalCountedLoopComparableExpr(boundExpr, module), module)
	if !ok || boundValue == nil {
		return formalForFirstIterationUnknown
	}

	unsigned := formalExprUsesUnsignedIntegerSemantics(ivComparable, module) || formalExprUsesUnsignedIntegerSemantics(boundExpr, module)
	result, ok := formalCompareIntegerConsts(compareOp, initValue, boundValue, unsigned)
	if !ok {
		return formalForFirstIterationUnknown
	}
	if result {
		return formalForFirstIterationAlways
	}
	return formalForFirstIterationNever
}

func formalSimpleForInitExpr(stmt ast.Stmt) (ast.Expr, ast.Expr, bool) {
	assign, ok := stmt.(*ast.AssignStmt)
	if !ok || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
		return nil, nil, false
	}
	switch assign.Tok {
	case token.ASSIGN, token.DEFINE:
		if ident, ok := formalUnparenExpr(assign.Lhs[0]).(*ast.Ident); ok && ident.Name == "_" {
			return nil, nil, false
		}
		return assign.Rhs[0], assign.Lhs[0], true
	default:
		return nil, nil, false
	}
}

func formalReverseComparisonOp(op token.Token) (token.Token, bool) {
	switch op {
	case token.LSS:
		return token.GTR, true
	case token.LEQ:
		return token.GEQ, true
	case token.GTR:
		return token.LSS, true
	case token.GEQ:
		return token.LEQ, true
	case token.EQL, token.NEQ:
		return op, true
	default:
		return token.ILLEGAL, false
	}
}

func formalExprStructuralEqual(lhs ast.Expr, rhs ast.Expr) bool {
	lhs = formalUnparenExpr(lhs)
	rhs = formalUnparenExpr(rhs)
	switch l := lhs.(type) {
	case *ast.Ident:
		r, ok := rhs.(*ast.Ident)
		return ok && l.Name == r.Name
	case *ast.BasicLit:
		r, ok := rhs.(*ast.BasicLit)
		return ok && l.Kind == r.Kind && l.Value == r.Value
	case *ast.SelectorExpr:
		r, ok := rhs.(*ast.SelectorExpr)
		return ok && formalExprStructuralEqual(l.X, r.X) && l.Sel != nil && r.Sel != nil && l.Sel.Name == r.Sel.Name
	case *ast.IndexExpr:
		r, ok := rhs.(*ast.IndexExpr)
		return ok && formalExprStructuralEqual(l.X, r.X) && formalExprStructuralEqual(l.Index, r.Index)
	case *ast.StarExpr:
		r, ok := rhs.(*ast.StarExpr)
		return ok && formalExprStructuralEqual(l.X, r.X)
	default:
		return false
	}
}

func formalCompareIntegerConsts(op token.Token, lhs goconstant.Value, rhs goconstant.Value, unsigned bool) (bool, bool) {
	lhs = goconstant.ToInt(lhs)
	rhs = goconstant.ToInt(rhs)
	if lhs == nil || rhs == nil {
		return false, false
	}
	if unsigned {
		lhsU, ok := goconstant.Uint64Val(lhs)
		if !ok {
			return false, false
		}
		rhsU, ok := goconstant.Uint64Val(rhs)
		if !ok {
			return false, false
		}
		switch op {
		case token.LSS:
			return lhsU < rhsU, true
		case token.LEQ:
			return lhsU <= rhsU, true
		case token.GTR:
			return lhsU > rhsU, true
		case token.GEQ:
			return lhsU >= rhsU, true
		case token.EQL:
			return lhsU == rhsU, true
		case token.NEQ:
			return lhsU != rhsU, true
		default:
			return false, false
		}
	}
	lhsI, ok := goconstant.Int64Val(lhs)
	if !ok {
		return false, false
	}
	rhsI, ok := goconstant.Int64Val(rhs)
	if !ok {
		return false, false
	}
	switch op {
	case token.LSS:
		return lhsI < rhsI, true
	case token.LEQ:
		return lhsI <= rhsI, true
	case token.GTR:
		return lhsI > rhsI, true
	case token.GEQ:
		return lhsI >= rhsI, true
	case token.EQL:
		return lhsI == rhsI, true
	case token.NEQ:
		return lhsI != rhsI, true
	default:
		return false, false
	}
}

func matchFormalCountedLoopCond(expr ast.Expr, module *formalModuleContext) (formalCountedLoopSpec, bool) {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok {
		return formalCountedLoopSpec{}, false
	}
	ident, ok := formalCountedLoopCondIdent(binary.X, module)
	if !ok || ident.Name == "_" {
		return formalCountedLoopSpec{}, false
	}
	spec := formalCountedLoopSpec{
		ivName:    ident.Name,
		boundExpr: formalCountedLoopComparableExpr(binary.Y, module),
		unsigned:  formalExprUsesUnsignedIntegerSemantics(binary.X, module) || formalExprUsesUnsignedIntegerSemantics(binary.Y, module),
	}
	switch binary.Op {
	case token.LSS:
		spec.step = +1
	case token.LEQ:
		spec.step = +1
		spec.inclusive = true
	case token.GTR:
		spec.step = -1
	case token.GEQ:
		spec.step = -1
		spec.inclusive = true
	default:
		return formalCountedLoopSpec{}, false
	}
	return spec, true
}

func formalLoopContainsUnsupportedCurrentBranch(stmts []ast.Stmt, allowBreak bool) bool {
	for _, stmt := range stmts {
		found := false
		ast.Inspect(stmt, func(n ast.Node) bool {
			switch node := n.(type) {
			case nil:
				return false
			case *ast.FuncLit:
				return false
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				if n == stmt {
					return true
				}
				return false
			case *ast.BranchStmt:
				if node.Label != nil || (node.Tok == token.BREAK && !allowBreak) {
					found = true
					return false
				}
			}
			return true
		})
		if found {
			return true
		}
	}
	return false
}

func formalLoopContainsUnsupportedBranch(stmts []ast.Stmt) bool {
	return formalLoopContainsUnsupportedCurrentBranch(stmts, false)
}

func emitFormalCountedLoopExitIV(node ast.Node, values formalCountedLoopValues, spec formalCountedLoopSpec, env *formalEnv) (string, string) {
	cmp := env.temp("loop_exit_cmp")
	exit := env.temp("loop_exit")
	adjusted := values.bound
	pred := "slt"
	if spec.unsigned {
		pred = "ult"
	}
	var buf strings.Builder
	if spec.step > 0 && spec.inclusive {
		adjusted = env.temp("loop_exit_bound")
		pred = strings.Replace(pred, "lt", "le", 1)
		one := env.temp("const")
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.constant 1 : %s", one, values.ty))
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.addi %s, %s : %s", adjusted, values.bound, one, values.ty))
	} else if spec.step < 0 && spec.inclusive {
		adjusted = env.temp("loop_exit_bound")
		if spec.unsigned {
			pred = "uge"
		} else {
			pred = "sge"
		}
		one := env.temp("const")
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.constant 1 : %s", one, values.ty))
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.subi %s, %s : %s", adjusted, values.bound, one, values.ty))
	} else if spec.step < 0 {
		if spec.unsigned {
			pred = "ugt"
		} else {
			pred = "sgt"
		}
	} else if spec.unsigned {
		pred = "ult"
	} else {
		pred = "slt"
	}
	if spec.step > 0 && !spec.inclusive {
		if spec.unsigned {
			pred = "ult"
		} else {
			pred = "slt"
		}
	}
	buf.WriteString(emitFormalLinef(node, env, "    %s = arith.cmpi %s, %s, %s : %s", cmp, pred, values.init, values.bound, values.ty))
	buf.WriteString(emitFormalLinef(node, env, "    %s = arith.select %s, %s, %s : %s", exit, cmp, adjusted, values.init, values.ty))
	return exit, buf.String()
}

func matchFormalCountedLoopStep(stmt ast.Stmt, ivName string, module *formalModuleContext) (int, bool) {
	switch s := stmt.(type) {
	case *ast.IncDecStmt:
		ident, ok := s.X.(*ast.Ident)
		if !ok || ident.Name != ivName {
			return 0, false
		}
		switch s.Tok {
		case token.INC:
			return +1, true
		case token.DEC:
			return -1, true
		default:
			return 0, false
		}
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return 0, false
		}
		lhs, ok := s.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != ivName {
			return 0, false
		}
		switch s.Tok {
		case token.ADD_ASSIGN:
			if formalExprIsIntConstantOne(s.Rhs[0], module) {
				return +1, true
			}
			return 0, false
		case token.SUB_ASSIGN:
			if formalExprIsIntConstantOne(s.Rhs[0], module) {
				return -1, true
			}
			return 0, false
		case token.ASSIGN:
		default:
			return 0, false
		}
		binary, ok := s.Rhs[0].(*ast.BinaryExpr)
		if !ok {
			return 0, false
		}
		x, ok := binary.X.(*ast.Ident)
		if !ok || x.Name != ivName {
			if binary.Op == token.ADD {
				y, yok := binary.Y.(*ast.Ident)
				if yok && y.Name == ivName && formalExprIsIntConstantOne(binary.X, module) {
					return +1, true
				}
			}
			return 0, false
		}
		if !formalExprIsIntConstantOne(binary.Y, module) {
			return 0, false
		}
		switch binary.Op {
		case token.ADD:
			return +1, true
		case token.SUB:
			return -1, true
		default:
			return 0, false
		}
	default:
		return 0, false
	}
}

func formalExprIsIntConstantOne(expr ast.Expr, module *formalModuleContext) bool {
	if lit, ok := formalUnparenExpr(expr).(*ast.BasicLit); ok && lit.Kind == token.INT && lit.Value == "1" {
		return true
	}
	val, _, ok := formalTypedConstValue(expr, module)
	return ok && val != nil && val.String() == "1"
}

func formalCountedLoopCondIdent(expr ast.Expr, module *formalModuleContext) (*ast.Ident, bool) {
	if ident, ok := formalCountedLoopIdent(formalCountedLoopComparableExpr(expr, module)); ok {
		return ident, true
	}
	return nil, false
}

func formalCountedLoopComparableExpr(expr ast.Expr, module *formalModuleContext) ast.Expr {
	expr = formalUnparenExpr(expr)
	call, ok := formalUnparenExpr(expr).(*ast.CallExpr)
	if !ok || !isFormalTypeConversionCall(call, module) {
		return expr
	}
	convertedTy, ok := formalTypedExprType(call, module)
	if !ok || !isFormalIntegerType(convertedTy) {
		return expr
	}
	argTy, ok := formalTypedExprType(call.Args[0], module)
	if !ok || !isFormalIntegerType(argTy) {
		return expr
	}
	return formalCountedLoopComparableExpr(call.Args[0], module)
}

func formalCountedLoopIdent(expr ast.Expr) (*ast.Ident, bool) {
	ident, ok := formalUnparenExpr(expr).(*ast.Ident)
	if !ok || ident.Name == "_" {
		return nil, false
	}
	return ident, true
}

func emitFormalCountedLoopTripCount(node ast.Node, values formalCountedLoopValues, spec formalCountedLoopSpec, env *formalEnv) (formalCountedLoopLowering, string, bool) {
	initIndex := values.init
	boundIndex := values.bound
	var buf strings.Builder
	if values.ty != "index" {
		initIndex = env.temp("idx")
		boundIndex = env.temp("idx")
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.index_cast %s : %s to index", initIndex, values.init, values.ty))
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.index_cast %s : %s to index", boundIndex, values.bound, values.ty))
	}
	diff := env.temp("trip")
	if spec.step > 0 {
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.subi %s, %s : index", diff, boundIndex, initIndex))
	} else {
		buf.WriteString(emitFormalLinef(node, env, "    %s = arith.subi %s, %s : index", diff, initIndex, boundIndex))
	}
	lowering := formalCountedLoopLowering{
		values:    values,
		initIndex: initIndex,
		tripCount: diff,
	}
	if !spec.inclusive {
		return lowering, buf.String(), true
	}
	one := env.temp("const")
	trip := env.temp("trip")
	buf.WriteString(emitFormalLinef(node, env, "    %s = arith.constant 1 : index", one))
	buf.WriteString(emitFormalLinef(node, env, "    %s = arith.addi %s, %s : index", trip, diff, one))
	lowering.tripCount = trip
	return lowering, buf.String(), true
}

func emitFormalCountedLoopBodyIV(node ast.Node, iter string, lowering formalCountedLoopLowering, spec formalCountedLoopSpec, env *formalEnv) (string, bool) {
	actualIndex := iter
	var buf strings.Builder
	if lowering.initIndex != iter {
		actualIndex = env.temp(sanitizeName(spec.ivName) + "_idx")
		op := "arith.addi"
		if spec.step < 0 {
			op = "arith.subi"
		}
		buf.WriteString(emitFormalLinef(node, env, "    %s = %s %s, %s : index", actualIndex, op, lowering.initIndex, iter))
	}
	if lowering.values.ty == "index" {
		env.bindValue(spec.ivName, actualIndex, lowering.values.ty)
		return buf.String(), true
	}
	ivCast := env.temp(sanitizeName(spec.ivName) + "_body")
	buf.WriteString(emitFormalLinef(node, env, "    %s = arith.index_cast %s : index to %s", ivCast, actualIndex, lowering.values.ty))
	env.bindValue(spec.ivName, ivCast, lowering.values.ty)
	return buf.String(), true
}

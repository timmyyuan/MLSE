package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

func emitFormalForLoopReturnState(s *ast.ForStmt, suffix []ast.Stmt, env *formalEnv, initState formalLoopReturnState) (string, formalLoopReturnState, bool) {
	if formalLoopReturnHasComplexInitOrPost(s) {
		return "", formalLoopReturnState{}, false
	}
	if text, state, ok := emitFormalCountedForLoopReturnState(s, suffix, env, initState); ok {
		return text, state, true
	}
	return emitFormalWhileLoopReturnState(s, suffix, env, initState)
}

func formalLoopReturnHasComplexInitOrPost(s *ast.ForStmt) bool {
	if s == nil {
		return false
	}
	return formalLoopReturnHasComplexLValue(s.Init) || formalLoopReturnHasComplexLValue(s.Post)
}

func formalLoopReturnHasComplexLValue(stmt ast.Stmt) bool {
	switch s := stmt.(type) {
	case nil:
		return false
	case *ast.AssignStmt:
		for _, lhs := range s.Lhs {
			if _, ok := lhs.(*ast.Ident); !ok {
				return true
			}
		}
		return false
	case *ast.IncDecStmt:
		_, ok := s.X.(*ast.Ident)
		return !ok
	default:
		return false
	}
}

// emitFormalCountedForLoopReturnState lowers counted `for` loops with function-level early return into `scf.for iter_args`.
func emitFormalCountedForLoopReturnState(s *ast.ForStmt, suffix []ast.Stmt, env *formalEnv, initState formalLoopReturnState) (string, formalLoopReturnState, bool) {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return "", formalLoopReturnState{}, false
		}
	}
	if s.Cond == nil {
		return "", formalLoopReturnState{}, false
	}

	loopSpec, ok := matchFormalCountedLoopCond(s.Cond, env.module)
	if !ok || len(s.Body.List) == 0 {
		return "", formalLoopReturnState{}, false
	}

	bodyStmts := s.Body.List
	if s.Post != nil {
		step, ok := matchFormalCountedLoopStep(s.Post, loopSpec.ivName, env.module)
		if !ok || step != loopSpec.step {
			return "", formalLoopReturnState{}, false
		}
	} else {
		last := bodyStmts[len(bodyStmts)-1]
		step, ok := matchFormalCountedLoopStep(last, loopSpec.ivName, env.module)
		if !ok || step != loopSpec.step {
			return "", formalLoopReturnState{}, false
		}
		bodyStmts = bodyStmts[:len(bodyStmts)-1]
	}

	carried := collectAssignedOuterNames(bodyStmts, env, loopSpec.ivName)
	carriedTys := make([]string, 0, len(carried))
	carriedIters := make([]string, 0, len(carried))

	ivInit := env.use(loopSpec.ivName)
	ivTy := env.typeOf(loopSpec.ivName)
	if !isFormalIntegerType(ivTy) {
		return "", formalLoopReturnState{}, false
	}

	bound, boundTy, boundPrelude := emitFormalExpr(loopSpec.boundExpr, ivTy, env)
	if boundTy != ivTy {
		return "", formalLoopReturnState{}, false
	}
	buf.WriteString(boundPrelude)
	loopValues := formalCountedLoopValues{init: ivInit, bound: bound, ty: ivTy}

	lowering, tripPrelude, ok := emitFormalCountedLoopTripCount(s, loopValues, loopSpec, env)
	if !ok || lowering.initIndex == "" || lowering.tripCount == "" {
		return "", formalLoopReturnState{}, false
	}
	buf.WriteString(tripPrelude)

	lowerValue := env.temp("idx")
	step := env.temp("const")
	iterSSA := env.temp(sanitizeName(loopSpec.ivName) + "_iter")
	stopIter := env.temp("loopret_stop_iter")
	doneIter := env.temp("loopret_done_iter")
	retIters := make([]string, 0, len(initState.retTypes))
	for i := range initState.retTypes {
		retIters = append(retIters, env.temp(fmt.Sprintf("loopret%d_iter", i)))
	}
	iterArgs := []string{
		stopIter + " = " + initState.stop,
		doneIter + " = " + initState.done,
	}
	resultTypes := formalLoopReturnResultTypes(initState)
	for i, ty := range initState.retTypes {
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", retIters[i], initState.retValues[i]))
		_ = ty
	}
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 0 : index", lowerValue))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 1 : index", step))

	bodyEnv := env.clone()
	for _, name := range carried {
		ty := env.typeOf(name)
		carriedTys = append(carriedTys, ty)
		iterSSA := env.temp(sanitizeName(name) + "_iter")
		carriedIters = append(carriedIters, iterSSA)
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", iterSSA, env.use(name)))
		bodyEnv.bindValue(name, iterSSA, ty)
		resultTypes = append(resultTypes, ty)
	}
	result := env.temp("loopret")
	var forBuf strings.Builder
	forBuf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), iterSSA, lowerValue, lowering.tripCount, step, strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", ")))

	bodyPrelude, ok := emitFormalCountedLoopBodyIV(s, iterSSA, lowering, loopSpec, bodyEnv)
	if !ok {
		syncFormalTempID(env, bodyEnv)
		return "", formalLoopReturnState{}, false
	}

	iterState := formalLoopReturnState{
		stop:      stopIter,
		done:      doneIter,
		retValues: retIters,
		retTypes:  append([]string(nil), initState.retTypes...),
	}
	bodyText, bodyState, ok := emitFormalLoopReturnSequence(bodyStmts, bodyEnv, iterState)
	if !ok {
		syncFormalTempID(env, bodyEnv)
		return "", formalLoopReturnState{}, false
	}
	bodyCarryValues, bodyCarryPrelude, ok := coerceFormalLoopCarriedValues(bodyEnv, carried, carriedTys)
	if !ok {
		syncFormalTempID(env, bodyEnv)
		return "", formalLoopReturnState{}, false
	}
	stepResult := bodyEnv.temp("loopstep")
	forBuf.WriteString(fmt.Sprintf("        %s = scf.if %s -> (%s) {\n", formalIfResultBinding(stepResult, len(resultTypes)), stopIter, strings.Join(resultTypes, ", ")))
	forBuf.WriteString(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(append(formalLoopReturnResultValues(iterState), carriedIters...), ", "), strings.Join(resultTypes, ", ")))
	forBuf.WriteString("        } else {\n")
	forBuf.WriteString(indentBlock(bodyPrelude, 3))
	forBuf.WriteString(indentBlock(bodyText, 3))
	forBuf.WriteString(indentBlock(bodyCarryPrelude, 2))
	forBuf.WriteString(indentBlock(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(append(formalLoopReturnResultValues(bodyState), bodyCarryValues...), ", "), strings.Join(resultTypes, ", ")), 2))
	forBuf.WriteString("        }\n")
	stepRefs := formalMultiResultRefs(stepResult, len(resultTypes))
	forBuf.WriteString(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(stepRefs, ", "), strings.Join(resultTypes, ", ")))
	forBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(forBuf.String(), s, env))
	if formalStmtListUsesIdent(suffix, loopSpec.ivName) {
		exitIV, exitPrelude := emitFormalCountedLoopExitIV(s, loopValues, loopSpec, env)
		buf.WriteString(exitPrelude)
		env.bindValue(loopSpec.ivName, exitIV, ivTy)
	}
	resultRefs := formalMultiResultRefs(result, len(resultTypes))
	for i, name := range carried {
		env.bindValue(name, resultRefs[len(initState.retTypes)+2+i], carriedTys[i])
	}
	syncFormalTempID(env, bodyEnv)
	return buf.String(), formalLoopReturnLoopExitState(result, initState), true
}

func emitFormalWhileLoopReturnState(s *ast.ForStmt, suffix []ast.Stmt, env *formalEnv, initState formalLoopReturnState) (string, formalLoopReturnState, bool) {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return "", formalLoopReturnState{}, false
		}
	}
	_, syntheticRestart := formalSyntheticGotoRestartFlagName(s)
	if s.Cond == nil || (formalLoopContainsUnsupportedCurrentBranch(s.Body.List, true) && !syntheticRestart) {
		return "", formalLoopReturnState{}, false
	}

	carriedScan := append([]ast.Stmt{}, s.Body.List...)
	if s.Post != nil {
		carriedScan = append(carriedScan, s.Post)
	}
	carried := collectAssignedOuterNamesDeep(carriedScan, env, "")
	carriedTys := make([]string, 0, len(carried))

	stopIter := env.temp("loopret_stop_iter")
	doneIter := env.temp("loopret_done_iter")
	stopBody := env.temp("loopret_stop_body")
	doneBody := env.temp("loopret_done_body")
	retIters := make([]string, 0, len(initState.retTypes))
	retBodies := make([]string, 0, len(initState.retTypes))
	for i := range initState.retTypes {
		retIters = append(retIters, env.temp(fmt.Sprintf("loopret%d_iter", i)))
		retBodies = append(retBodies, env.temp(fmt.Sprintf("loopret%d_body", i)))
	}
	iterArgs := []string{
		stopIter + " = " + initState.stop,
		doneIter + " = " + initState.done,
	}
	condValues := []string{stopIter, doneIter}
	bodyArgs := []string{
		fmt.Sprintf("%s: i1", stopBody),
		fmt.Sprintf("%s: i1", doneBody),
	}
	resultTypes := formalLoopReturnResultTypes(initState)
	for i := range initState.retTypes {
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", retIters[i], initState.retValues[i]))
		condValues = append(condValues, retIters[i])
		bodyArgs = append(bodyArgs, fmt.Sprintf("%s: %s", retBodies[i], initState.retTypes[i]))
	}
	carriedIter := make([]string, 0, len(carried))
	carriedBody := make([]string, 0, len(carried))
	for _, name := range carried {
		ty := env.typeOf(name)
		carriedTys = append(carriedTys, ty)
		iterSSA := env.temp(sanitizeName(name) + "_iter")
		bodySSA := env.temp(sanitizeName(name) + "_body")
		carriedIter = append(carriedIter, iterSSA)
		carriedBody = append(carriedBody, bodySSA)
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", iterSSA, env.use(name)))
		condValues = append(condValues, iterSSA)
		bodyArgs = append(bodyArgs, fmt.Sprintf("%s: %s", bodySSA, ty))
		resultTypes = append(resultTypes, ty)
	}

	condEnv := env.clone()
	condEnv.bindValue(stopIter, stopIter, "i1")
	condEnv.bindValue(doneIter, doneIter, "i1")
	for i := range initState.retTypes {
		condEnv.bindValue(retIters[i], retIters[i], initState.retTypes[i])
	}
	for i, name := range carried {
		condEnv.bindValue(name, carriedIter[i], carriedTys[i])
	}
	cond, condPrelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		return "", formalLoopReturnState{}, false
	}
	continueFlag := env.temp("loopret_continue_flag")
	continueCond := env.temp("loopret_continue")
	loopCond := env.temp("loopret_cond")

	bodyEnv := env.clone()
	syncFormalTempID(bodyEnv, condEnv)
	bodyEnv.bindValue(stopBody, stopBody, "i1")
	bodyEnv.bindValue(doneBody, doneBody, "i1")
	for i := range initState.retTypes {
		bodyEnv.bindValue(retBodies[i], retBodies[i], initState.retTypes[i])
	}
	for i, name := range carried {
		bodyEnv.bindValue(name, carriedBody[i], carriedTys[i])
	}
	iterState := formalLoopReturnState{
		stop:      stopBody,
		done:      doneBody,
		retValues: retBodies,
		retTypes:  append([]string(nil), initState.retTypes...),
	}
	bodyText, bodyState, ok := emitFormalLoopReturnSequence(s.Body.List, bodyEnv, iterState)
	if !ok {
		syncFormalTempID(env, condEnv, bodyEnv)
		return "", formalLoopReturnState{}, false
	}
	bodyCarryValues, bodyCarryPrelude, ok := coerceFormalLoopCarriedValues(bodyEnv, carried, carriedTys)
	if !ok {
		syncFormalTempID(env, condEnv, bodyEnv)
		return "", formalLoopReturnState{}, false
	}

	postEnv := bodyEnv.clone()
	postText := ""
	if s.Post != nil {
		postStmtText, term := emitFormalStmt(s.Post, postEnv, nil)
		if term {
			syncFormalTempID(env, condEnv, bodyEnv, postEnv)
			return "", formalLoopReturnState{}, false
		}
		postText = postStmtText
	}
	postCarryValues, postCarryPrelude, ok := coerceFormalLoopCarriedValues(postEnv, carried, carriedTys)
	if !ok {
		syncFormalTempID(env, condEnv, bodyEnv, postEnv)
		return "", formalLoopReturnState{}, false
	}

	postResult := postEnv.temp("loopret_post")
	result := env.temp("loopret")
	var whileBuf strings.Builder
	whileBuf.WriteString(fmt.Sprintf("    %s = scf.while (%s) : (%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString(indentBlock(condPrelude, 1))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.constant true", continueFlag))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.xori %s, %s : i1", continueCond, stopIter, continueFlag))
	whileBuf.WriteString(emitFormalLinef(s, env, "      %s = arith.andi %s, %s : i1", loopCond, continueCond, cond))
	whileBuf.WriteString(emitFormalLinef(s, env, "      scf.condition(%s) %s : %s", loopCond, strings.Join(condValues, ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString("    } do {\n")
	whileBuf.WriteString(fmt.Sprintf("    ^bb0(%s):\n", strings.Join(bodyArgs, ", ")))
	whileBuf.WriteString(indentBlock(bodyText, 1))
	whileBuf.WriteString(fmt.Sprintf("      %s = scf.if %s -> (%s) {\n", formalIfResultBinding(postResult, len(resultTypes)), bodyState.stop, strings.Join(resultTypes, ", ")))
	whileBuf.WriteString(indentBlock(bodyCarryPrelude, 2))
	whileBuf.WriteString(indentBlock(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(append(formalLoopReturnResultValues(bodyState), bodyCarryValues...), ", "), strings.Join(resultTypes, ", ")), 0))
	whileBuf.WriteString("      } else {\n")
	whileBuf.WriteString(indentBlock(postText, 2))
	whileBuf.WriteString(indentBlock(postCarryPrelude, 2))
	whileBuf.WriteString(indentBlock(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(append(formalLoopReturnResultValues(bodyState), postCarryValues...), ", "), strings.Join(resultTypes, ", ")), 0))
	whileBuf.WriteString("      }\n")
	whileBuf.WriteString(emitFormalLinef(s, env, "      scf.yield %s : %s", strings.Join(formalMultiResultRefs(postResult, len(resultTypes)), ", "), strings.Join(resultTypes, ", ")))
	whileBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(whileBuf.String(), s, env))
	syncFormalTempID(env, condEnv, bodyEnv, postEnv)
	resultRefs := formalMultiResultRefs(result, len(resultTypes))
	for i, name := range carried {
		env.bindValue(name, resultRefs[len(initState.retTypes)+2+i], carriedTys[i])
	}
	return buf.String(), formalLoopReturnLoopExitState(result, initState), true
}

// emitFormalRangeLoopReturnState lowers `range` loops with function-level early return into `scf.for iter_args`.
func emitFormalRangeLoopReturnState(s *ast.RangeStmt, env *formalEnv, initState formalLoopReturnState) (string, formalLoopReturnState, bool) {
	if s.Tok != token.DEFINE && s.Tok != token.ASSIGN {
		return "", formalLoopReturnState{}, false
	}
	excludes := make(map[string]struct{})
	if keyName := rangeKeyName(s.Key); keyName != "" {
		excludes[keyName] = struct{}{}
	}
	if valueName := rangeKeyName(s.Value); valueName != "" {
		excludes[valueName] = struct{}{}
	}
	if len(collectAssignedOuterNamesWithExcludes(s.Body.List, env, excludes)) != 0 {
		return "", formalLoopReturnState{}, false
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
	stopIter := env.temp("loopret_stop_iter")
	doneIter := env.temp("loopret_done_iter")
	retIters := make([]string, 0, len(initState.retTypes))
	for i := range initState.retTypes {
		retIters = append(retIters, env.temp(fmt.Sprintf("loopret%d_iter", i)))
	}
	iterArgs := []string{
		stopIter + " = " + initState.stop,
		doneIter + " = " + initState.done,
	}
	for i := range initState.retTypes {
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", retIters[i], initState.retValues[i]))
	}

	var buf strings.Builder
	buf.WriteString(sourcePrelude)
	buf.WriteString(lengthPrelude)
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 0 : index", lower))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.index_cast %s : %s to index", upper, lengthTmp, formalTargetIntType(env.module)))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant 1 : index", step))

	result := env.temp("range")
	resultTypes := formalLoopReturnResultTypes(initState)
	var rangeBuf strings.Builder
	rangeBuf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), ivSSA, lower, upper, step, strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", ")))

	bodyEnv := env.clone()
	bodyPrelude := emitFormalRangeBindings(s, source, sourceTy, ivSSA, bodyEnv)
	iterState := formalLoopReturnState{
		stop:      stopIter,
		done:      doneIter,
		retValues: retIters,
		retTypes:  append([]string(nil), initState.retTypes...),
	}
	bodyText, bodyState, ok := emitFormalLoopReturnSequence(s.Body.List, bodyEnv, iterState)
	if !ok {
		syncFormalTempID(env, bodyEnv)
		return "", formalLoopReturnState{}, false
	}
	stepResult := bodyEnv.temp("rangestep")
	rangeBuf.WriteString(fmt.Sprintf("        %s = scf.if %s -> (%s) {\n", formalIfResultBinding(stepResult, len(resultTypes)), stopIter, strings.Join(resultTypes, ", ")))
	rangeBuf.WriteString(emitFormalLoopReturnYield(iterState, env))
	rangeBuf.WriteString("        } else {\n")
	rangeBuf.WriteString(indentBlock(bodyPrelude, 3))
	rangeBuf.WriteString(indentBlock(bodyText, 3))
	rangeBuf.WriteString(indentBlock(emitFormalLoopReturnYield(bodyState, env), 2))
	rangeBuf.WriteString("        }\n")
	stepRefs := formalLoopReturnRefs(stepResult, initState)
	rangeBuf.WriteString(emitFormalLoopReturnYield(stepRefs, env))
	rangeBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(rangeBuf.String(), s, env))
	syncFormalTempID(env, bodyEnv)
	return buf.String(), formalLoopReturnLoopExitState(result, initState), true
}

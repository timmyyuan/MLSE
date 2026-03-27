package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// emitFormalForLoopReturnState lowers counted `for` loops with function-level early return into `scf.for iter_args`.
func emitFormalForLoopReturnState(s *ast.ForStmt, suffix []ast.Stmt, env *formalEnv, initState formalLoopReturnState) (string, formalLoopReturnState, bool) {
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

	ivName, upperExpr, ok := matchFormalCountedLoopCond(s.Cond)
	if !ok || len(s.Body.List) == 0 {
		return "", formalLoopReturnState{}, false
	}

	bodyStmts := s.Body.List
	if s.Post != nil {
		if !isFormalLoopIncrement(s.Post, ivName) {
			return "", formalLoopReturnState{}, false
		}
	} else {
		last := bodyStmts[len(bodyStmts)-1]
		if !isFormalLoopIncrement(last, ivName) {
			return "", formalLoopReturnState{}, false
		}
		bodyStmts = bodyStmts[:len(bodyStmts)-1]
	}

	carried := collectAssignedOuterNames(bodyStmts, env, ivName)
	if len(carried) != 0 {
		return "", formalLoopReturnState{}, false
	}

	ivInit := env.use(ivName)
	ivTy := env.typeOf(ivName)
	if !isFormalIntegerType(ivTy) {
		return "", formalLoopReturnState{}, false
	}

	upper, upperTy, upperPrelude := emitFormalExpr(upperExpr, ivTy, env)
	buf.WriteString(upperPrelude)
	if upperTy != ivTy {
		return "", formalLoopReturnState{}, false
	}

	lowerValue := ivInit
	upperValue := upper
	loopBoundTy := ivTy
	if ivTy != "index" {
		lowerIndex := env.temp("idx")
		upperIndex := env.temp("idx")
		buf.WriteString(fmt.Sprintf("    %s = arith.index_cast %s : %s to index\n", lowerIndex, ivInit, ivTy))
		buf.WriteString(fmt.Sprintf("    %s = arith.index_cast %s : %s to index\n", upperIndex, upper, ivTy))
		lowerValue = lowerIndex
		upperValue = upperIndex
		loopBoundTy = "index"
	}

	step := env.temp("const")
	ivSSA := env.temp(sanitizeName(ivName) + "_iv")
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
	for i, ty := range initState.retTypes {
		iterArgs = append(iterArgs, fmt.Sprintf("%s = %s", retIters[i], initState.retValues[i]))
		_ = ty
	}

	buf.WriteString(fmt.Sprintf("    %s = arith.constant 1 : %s\n", step, loopBoundTy))
	result := env.temp("loopret")
	resultTypes := formalLoopReturnResultTypes(initState)
	buf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), ivSSA, lowerValue, upperValue, step, strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", ")))

	bodyEnv := env.clone()
	bodyPrelude := ""
	if ivTy == "index" {
		bodyEnv.bindValue(ivName, ivSSA, ivTy)
	} else {
		ivCast := bodyEnv.temp(sanitizeName(ivName) + "_body")
		bodyPrelude = fmt.Sprintf("    %s = arith.index_cast %s : index to %s\n", ivCast, ivSSA, ivTy)
		bodyEnv.bindValue(ivName, ivCast, ivTy)
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
	stepResult := bodyEnv.temp("loopstep")
	buf.WriteString(fmt.Sprintf("        %s = scf.if %s -> (%s) {\n", formalIfResultBinding(stepResult, len(resultTypes)), stopIter, strings.Join(resultTypes, ", ")))
	buf.WriteString(emitFormalLoopReturnYield(iterState))
	buf.WriteString("        } else {\n")
	buf.WriteString(indentBlock(bodyPrelude, 3))
	buf.WriteString(indentBlock(bodyText, 3))
	buf.WriteString(indentBlock(emitFormalLoopReturnYield(bodyState), 2))
	buf.WriteString("        }\n")
	stepRefs := formalLoopReturnRefs(stepResult, initState)
	buf.WriteString(emitFormalLoopReturnYield(stepRefs))
	buf.WriteString("    }\n")
	if formalStmtListUsesIdent(suffix, ivName) {
		exitIV, _, exitPrelude := emitFormalTodoValue("loop_iv_exit", ivTy, env)
		buf.WriteString(exitPrelude)
		env.bindValue(ivName, exitIV, ivTy)
	}
	syncFormalTempID(env, bodyEnv)
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
	lengthTmp, lengthPrelude, ok := emitFormalGoLenValue(source, sourceTy, "i32", "rangelen", env)
	if !ok {
		lengthTmp, lengthPrelude = emitFormalHelperCall(
			formalHelperCallSpec{
				base:       "__mlse_range_len_" + sanitizeName(sourceTy),
				args:       []string{source},
				argTys:     []string{sourceTy},
				resultTy:   "i32",
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
	buf.WriteString(fmt.Sprintf("    %s = arith.constant 0 : index\n", lower))
	buf.WriteString(fmt.Sprintf("    %s = arith.index_cast %s : i32 to index\n", upper, lengthTmp))
	buf.WriteString(fmt.Sprintf("    %s = arith.constant 1 : index\n", step))

	result := env.temp("range")
	resultTypes := formalLoopReturnResultTypes(initState)
	buf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s) -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), ivSSA, lower, upper, step, strings.Join(iterArgs, ", "), strings.Join(resultTypes, ", ")))

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
	buf.WriteString(fmt.Sprintf("        %s = scf.if %s -> (%s) {\n", formalIfResultBinding(stepResult, len(resultTypes)), stopIter, strings.Join(resultTypes, ", ")))
	buf.WriteString(emitFormalLoopReturnYield(iterState))
	buf.WriteString("        } else {\n")
	buf.WriteString(indentBlock(bodyPrelude, 3))
	buf.WriteString(indentBlock(bodyText, 3))
	buf.WriteString(indentBlock(emitFormalLoopReturnYield(bodyState), 2))
	buf.WriteString("        }\n")
	stepRefs := formalLoopReturnRefs(stepResult, initState)
	buf.WriteString(emitFormalLoopReturnYield(stepRefs))
	buf.WriteString("    }\n")
	syncFormalTempID(env, bodyEnv)
	return buf.String(), formalLoopReturnLoopExitState(result, initState), true
}

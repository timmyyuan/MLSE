package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type formalLoopReturnState struct {
	stop      string
	done      string
	retValues []string
	retTypes  []string
}

func formalStmtListContainsReturn(stmts []ast.Stmt) bool {
	for _, stmt := range stmts {
		if formalStmtContainsReturn(stmt) {
			return true
		}
	}
	return false
}

func formalStmtContainsReturn(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found || n == nil {
			return !found
		}
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
			return false
		default:
			return true
		}
	})
	return found
}

func formalStmtContainsBreak(stmt ast.Stmt) bool {
	found := false
	ast.Inspect(stmt, func(n ast.Node) bool {
		if found || n == nil {
			return !found
		}
		switch node := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return false
		case *ast.BranchStmt:
			if node.Tok == token.BREAK && node.Label == nil {
				found = true
				return false
			}
			return true
		default:
			return true
		}
	})
	return found
}

func formalStmtListUsesIdent(stmts []ast.Stmt, name string) bool {
	if name == "" {
		return false
	}
	used := false
	for _, stmt := range stmts {
		ast.Inspect(stmt, func(n ast.Node) bool {
			if used || n == nil {
				return !used
			}
			switch node := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.Ident:
				if node.Name == name {
					used = true
					return false
				}
				return true
			default:
				return true
			}
		})
		if used {
			return true
		}
	}
	return false
}

func emitFormalLoopReturnInit(resultTypes []string, env *formalEnv) (string, formalLoopReturnState) {
	var buf strings.Builder
	stop := env.temp("stop")
	done := env.temp("done")
	buf.WriteString(emitFormalLinef(nil, env, "    %s = arith.constant false", stop))
	buf.WriteString(emitFormalLinef(nil, env, "    %s = arith.constant false", done))
	values := make([]string, 0, len(resultTypes))
	for _, ty := range resultTypes {
		value, prelude := emitFormalZeroValue(ty, env)
		buf.WriteString(prelude)
		values = append(values, value)
	}
	return buf.String(), formalLoopReturnState{
		stop:      stop,
		done:      done,
		retValues: values,
		retTypes:  append([]string(nil), resultTypes...),
	}
}

// emitFormalReturningLoopStmt recognizes loop statements whose body may early-return the enclosing function.
func emitFormalReturningLoopStmt(stmt ast.Stmt, remaining []ast.Stmt, env *formalEnv, resultTypes []string) (string, int, bool, bool) {
	body, values, types, consumed, ok := emitFormalReturningLoopRegion(stmt, remaining, env, resultTypes)
	if !ok {
		return "", 0, false, false
	}
	return body + emitFormalReturnLine(values, types, env), consumed, true, true
}

func formalLoopReturnResultTypes(state formalLoopReturnState) []string {
	types := make([]string, 0, len(state.retTypes)+2)
	types = append(types, "i1")
	types = append(types, "i1")
	types = append(types, state.retTypes...)
	return types
}

func formalLoopReturnResultValues(state formalLoopReturnState) []string {
	values := make([]string, 0, len(state.retValues)+2)
	values = append(values, state.stop)
	values = append(values, state.done)
	values = append(values, state.retValues...)
	return values
}

func emitFormalLoopReturnYield(state formalLoopReturnState, env *formalEnv) string {
	return emitFormalYieldLine(formalLoopReturnResultValues(state), formalLoopReturnResultTypes(state), env)
}

func formalLoopReturnRefs(base string, state formalLoopReturnState) formalLoopReturnState {
	refs := formalMultiResultRefs(base, len(state.retTypes)+2)
	return formalLoopReturnState{
		stop:      refs[0],
		done:      refs[1],
		retValues: refs[2:],
		retTypes:  append([]string(nil), state.retTypes...),
	}
}

func formalLoopReturnLoopExitState(base string, state formalLoopReturnState) formalLoopReturnState {
	refs := formalMultiResultRefs(base, len(state.retTypes)+2)
	return formalLoopReturnState{
		stop:      refs[1],
		done:      refs[1],
		retValues: refs[2:],
		retTypes:  append([]string(nil), state.retTypes...),
	}
}

func emitFormalLoopReturnStmt(s *ast.ReturnStmt, env *formalEnv, state formalLoopReturnState) (string, formalLoopReturnState, bool) {
	var (
		values []string
		buf    strings.Builder
	)
	if len(s.Results) == 0 {
		for _, ty := range state.retTypes {
			value, prelude := emitFormalZeroValue(ty, env)
			buf.WriteString(prelude)
			values = append(values, value)
		}
	} else {
		operands, _, prelude, ok := emitFormalReturnExprOperands(s.Results, state.retTypes, env)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		buf.WriteString(prelude)
		values = operands
	}
	stop := env.temp("stop")
	done := env.temp("done")
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant true", stop))
	buf.WriteString(emitFormalLinef(s, env, "    %s = arith.constant true", done))
	return buf.String(), formalLoopReturnState{
		stop:      stop,
		done:      done,
		retValues: values,
		retTypes:  append([]string(nil), state.retTypes...),
	}, true
}

func emitFormalLoopBreakState(env *formalEnv, state formalLoopReturnState) (string, formalLoopReturnState, bool) {
	stop := env.temp("stop")
	return emitFormalLinef(nil, env, "    %s = arith.constant true", stop), formalLoopReturnState{
		stop:      stop,
		done:      state.done,
		retValues: append([]string(nil), state.retValues...),
		retTypes:  append([]string(nil), state.retTypes...),
	}, true
}

func emitFormalLoopReturnIfStmt(s *ast.IfStmt, suffix []ast.Stmt, env *formalEnv, state formalLoopReturnState) (string, formalLoopReturnState, bool) {
	if s == nil {
		return "", formalLoopReturnState{}, false
	}
	if s.Init != nil {
		scopedEnv := env.clone()
		initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
		if term {
			syncFormalTempID(env, scopedEnv)
			return "", formalLoopReturnState{}, false
		}
		clone := *s
		clone.Init = nil
		bodyText, bodyState, ok := emitFormalLoopReturnIfStmt(&clone, suffix, scopedEnv, state)
		if !ok {
			syncFormalTempID(env, scopedEnv)
			return "", formalLoopReturnState{}, false
		}
		syncFormalTempID(env, scopedEnv)
		return initText + bodyText, bodyState, true
	}

	condEnv := env.clone()
	cond, prelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		syncFormalTempID(env, condEnv)
		return "", formalLoopReturnState{}, false
	}

	thenEnv := condEnv.clone()
	thenStmts := append(append([]ast.Stmt(nil), s.Body.List...), suffix...)
	thenText, thenState, ok := emitFormalLoopReturnSequence(thenStmts, thenEnv, state)
	if !ok {
		syncFormalTempID(env, condEnv, thenEnv)
		return "", formalLoopReturnState{}, false
	}

	elseEnv := condEnv.clone()
	elseStmts := append([]ast.Stmt(nil), suffix...)
	if s.Else != nil {
		elseBlock, ok := s.Else.(*ast.BlockStmt)
		if !ok {
			syncFormalTempID(env, condEnv, thenEnv, elseEnv)
			return "", formalLoopReturnState{}, false
		}
		elseStmts = append(append([]ast.Stmt(nil), elseBlock.List...), suffix...)
	}
	elseText, elseState, ok := emitFormalLoopReturnSequence(elseStmts, elseEnv, state)
	if !ok {
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return "", formalLoopReturnState{}, false
	}

	syncFormalTempID(env, condEnv, thenEnv, elseEnv)
	result := env.temp("loopif")
	var buf strings.Builder
	buf.WriteString(prelude)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(state.retTypes)+2), cond, strings.Join(formalLoopReturnResultTypes(state), ", ")))
	ifBuf.WriteString(indentBlock(thenText, 2))
	ifBuf.WriteString(emitFormalLoopReturnYield(thenState, env))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(elseText, 2))
	ifBuf.WriteString(emitFormalLoopReturnYield(elseState, env))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
	return buf.String(), formalLoopReturnRefs(result, state), true
}

func emitFormalLoopBreakIfStmt(s *ast.IfStmt, suffix []ast.Stmt, env *formalEnv, state formalLoopReturnState) (string, formalLoopReturnState, bool) {
	if s == nil {
		return "", formalLoopReturnState{}, false
	}
	if s.Init != nil {
		scopedEnv := env.clone()
		initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
		if term {
			syncFormalTempID(env, scopedEnv)
			return "", formalLoopReturnState{}, false
		}
		clone := *s
		clone.Init = nil
		bodyText, bodyState, ok := emitFormalLoopBreakIfStmt(&clone, suffix, scopedEnv, state)
		if !ok {
			syncFormalTempID(env, scopedEnv)
			return "", formalLoopReturnState{}, false
		}
		syncFormalTempID(env, scopedEnv)
		return initText + bodyText, bodyState, true
	}

	breakPrefix, ok := matchFormalBreakIf(s)
	if !ok {
		return "", formalLoopReturnState{}, false
	}
	condEnv := env.clone()
	cond, prelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		syncFormalTempID(env, condEnv)
		return "", formalLoopReturnState{}, false
	}

	thenEnv := condEnv.clone()
	thenPrefix, _, term := emitFormalLoopBody(breakPrefix, thenEnv, "", "")
	if term {
		syncFormalTempID(env, condEnv, thenEnv)
		return "", formalLoopReturnState{}, false
	}
	thenBreak, thenState, ok := emitFormalLoopBreakState(thenEnv, state)
	if !ok {
		syncFormalTempID(env, condEnv, thenEnv)
		return "", formalLoopReturnState{}, false
	}

	elseEnv := condEnv.clone()
	elseText, elseState, ok := emitFormalLoopReturnSequence(suffix, elseEnv, state)
	if !ok {
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return "", formalLoopReturnState{}, false
	}

	syncFormalTempID(env, condEnv, thenEnv, elseEnv)
	result := env.temp("loopif")
	var buf strings.Builder
	buf.WriteString(prelude)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(state.retTypes)+2), cond, strings.Join(formalLoopReturnResultTypes(state), ", ")))
	ifBuf.WriteString(indentBlock(thenPrefix, 2))
	ifBuf.WriteString(indentBlock(thenBreak, 2))
	ifBuf.WriteString(emitFormalLoopReturnYield(thenState, env))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(elseText, 2))
	ifBuf.WriteString(emitFormalLoopReturnYield(elseState, env))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
	return buf.String(), formalLoopReturnRefs(result, state), true
}

func emitFormalLoopReturnSequence(stmts []ast.Stmt, env *formalEnv, state formalLoopReturnState) (string, formalLoopReturnState, bool) {
	firstReturn := -1
	firstBreak := -1
	for i, stmt := range stmts {
		if formalStmtContainsReturn(stmt) {
			firstReturn = i
			break
		}
		if firstBreak == -1 && formalStmtContainsBreak(stmt) {
			firstBreak = i
		}
	}
	firstControl := firstReturn
	controlKind := "return"
	if firstControl == -1 || (firstBreak != -1 && firstBreak < firstControl) {
		firstControl = firstBreak
		controlKind = "break"
	}
	if firstControl == -1 {
		bodyText, _, term := emitFormalLoopBody(stmts, env, "", "")
		if term {
			return "", formalLoopReturnState{}, false
		}
		return bodyText, state, true
	}

	prefix := stmts[:firstControl]
	target := stmts[firstControl]
	suffix := stmts[firstControl+1:]
	prefixText, _, term := emitFormalLoopBody(prefix, env, "", "")
	if term {
		return "", formalLoopReturnState{}, false
	}

	switch s := target.(type) {
	case *ast.ReturnStmt:
		if controlKind != "return" {
			return "", formalLoopReturnState{}, false
		}
		bodyText, nextState, ok := emitFormalLoopReturnStmt(s, env, state)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		return prefixText + bodyText, nextState, true
	case *ast.IfStmt:
		if controlKind == "break" && !formalStmtContainsReturn(s) {
			bodyText, nextState, ok := emitFormalLoopBreakIfStmt(s, suffix, env, state)
			if !ok {
				return "", formalLoopReturnState{}, false
			}
			return prefixText + bodyText, nextState, true
		}
		bodyText, nextState, ok := emitFormalLoopReturnIfStmt(s, suffix, env, state)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		return prefixText + bodyText, nextState, true
	case *ast.BranchStmt:
		if controlKind != "break" || s.Tok != token.BREAK || s.Label != nil {
			return "", formalLoopReturnState{}, false
		}
		bodyText, nextState, ok := emitFormalLoopBreakState(env, state)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		return prefixText + bodyText, nextState, true
	case *ast.ForStmt:
		bodyText, nextState, ok := emitFormalForLoopReturnState(s, suffix, env, state)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		return emitFormalLoopReturnSuffix(prefixText+bodyText, nextState, suffix, env)
	case *ast.RangeStmt:
		bodyText, nextState, ok := emitFormalRangeLoopReturnState(s, env, state)
		if !ok {
			return "", formalLoopReturnState{}, false
		}
		return emitFormalLoopReturnSuffix(prefixText+bodyText, nextState, suffix, env)
	default:
		return "", formalLoopReturnState{}, false
	}
}

func emitFormalLoopReturnSuffix(prefix string, state formalLoopReturnState, suffix []ast.Stmt, env *formalEnv) (string, formalLoopReturnState, bool) {
	if len(suffix) == 0 {
		return prefix, state, true
	}

	restEnv := env.clone()
	restText, restState, ok := emitFormalLoopReturnSequence(suffix, restEnv, state)
	if !ok {
		syncFormalTempID(env, restEnv)
		return "", formalLoopReturnState{}, false
	}

	result := env.temp("loopcont")
	var buf strings.Builder
	buf.WriteString(prefix)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(state.retTypes)+2), state.stop, strings.Join(formalLoopReturnResultTypes(state), ", ")))
	ifBuf.WriteString(emitFormalLoopReturnYield(state, env))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(restText, 2))
	ifBuf.WriteString(emitFormalLoopReturnYield(restState, env))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), nil, env))
	syncFormalTempID(env, restEnv)
	return buf.String(), formalLoopReturnRefs(result, state), true
}

func emitFormalReturningLoopRegion(stmt ast.Stmt, remaining []ast.Stmt, env *formalEnv, resultTypes []string) (string, []string, []string, int, bool) {
	if len(resultTypes) == 0 {
		return "", nil, nil, 0, false
	}
	if s, ok := stmt.(*ast.ForStmt); ok {
		if body, values, types, consumed, ok := emitFormalGuaranteedReturningForRegion(s, env, resultTypes); ok {
			return body, values, types, consumed, true
		}
	}
	workEnv := env.clone()
	initPrelude, initState := emitFormalLoopReturnInit(resultTypes, workEnv)

	var (
		loopText  string
		loopState formalLoopReturnState
		ok        bool
	)
	switch s := stmt.(type) {
	case *ast.ForStmt:
		if !formalStmtListContainsReturn(s.Body.List) {
			return "", nil, nil, 0, false
		}
		loopText, loopState, ok = emitFormalForLoopReturnState(s, remaining, workEnv, initState)
	case *ast.RangeStmt:
		if !formalStmtListContainsReturn(s.Body.List) {
			return "", nil, nil, 0, false
		}
		loopText, loopState, ok = emitFormalRangeLoopReturnState(s, workEnv, initState)
	default:
		return "", nil, nil, 0, false
	}
	if !ok {
		return "", nil, nil, 0, false
	}

	restEnv := workEnv.clone()
	restBody, restValues, restTypes, ok := emitFormalReturningRegion(remaining, restEnv, resultTypes)
	if !ok {
		return "", nil, nil, 0, false
	}

	syncFormalTempID(workEnv, restEnv)
	result := workEnv.temp("loopret")
	var buf strings.Builder
	buf.WriteString(initPrelude)
	buf.WriteString(loopText)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), loopState.done, strings.Join(resultTypes, ", ")))
	ifBuf.WriteString(emitFormalYieldLine(loopState.retValues, loopState.retTypes, workEnv))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(restBody, 2))
	ifBuf.WriteString(emitFormalYieldLine(restValues, restTypes, workEnv))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), stmt, workEnv))
	return buf.String(), formalMultiResultRefs(result, len(resultTypes)), append([]string(nil), resultTypes...), len(remaining) + 1, true
}

func emitFormalGuaranteedReturningForRegion(s *ast.ForStmt, env *formalEnv, resultTypes []string) (string, []string, []string, int, bool) {
	if s == nil || formalClassifyForFirstIteration(s, env.module) != formalForFirstIterationAlways {
		return "", nil, nil, 0, false
	}
	workEnv := env.clone()
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, workEnv, nil)
		buf.WriteString(initText)
		if term {
			syncFormalTempID(env, workEnv)
			return "", nil, nil, 0, false
		}
	}
	body, values, types, ok := emitFormalReturningRegion(s.Body.List, workEnv, resultTypes)
	if !ok {
		syncFormalTempID(env, workEnv)
		return "", nil, nil, 0, false
	}
	if strings.Contains(body, "go.todo") || strings.Contains(body, "go.todo_value") {
		syncFormalTempID(env, workEnv)
		return "", nil, nil, 0, false
	}
	syncFormalTempID(env, workEnv)
	return buf.String() + body, values, types, 1, true
}

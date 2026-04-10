package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// emitFormalLoopBody lowers loop bodies, including `continue`-shaped `if` regions with carried values.
func emitFormalLoopBody(stmts []ast.Stmt, env *formalEnv, carriedName string, carriedTy string) (string, string, bool) {
	carriedNames := []string(nil)
	carriedTys := []string(nil)
	if carriedName != "" {
		carriedNames = []string{carriedName}
		carriedTys = []string{carriedTy}
	}
	body, values, term := emitFormalLoopBodyWithCarried(stmts, env, carriedNames, carriedTys)
	if len(values) == 0 {
		return body, "", term
	}
	return body, values[0], term
}

func emitFormalBareContinueWithCarried(stmt ast.Stmt, env *formalEnv, carriedNames []string) (string, []string, bool) {
	branch, ok := stmt.(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || branch.Label != nil {
		return "", nil, false
	}
	if len(carriedNames) != 0 {
		return "", formalLoopCarriedValues(env, carriedNames), true
	}
	return "", nil, true
}

func emitFormalBareContinueWithBreak(stmt ast.Stmt, env *formalEnv, carriedNames []string) (string, []string, string, bool) {
	branch, ok := stmt.(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || branch.Label != nil {
		return "", nil, "", false
	}
	stop := env.temp("loopstop")
	text := emitFormalLinef(branch, env, "    %s = arith.constant false", stop)
	if len(carriedNames) != 0 {
		return text, formalLoopCarriedValues(env, carriedNames), stop, true
	}
	return text, nil, stop, true
}

func emitFormalLoopReturningStmtWithCarried(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, bool, bool) {
	text, consumed, term, ok := emitFormalReturningLoopStmt(stmts[0], stmts[1:], env, env.resultTypes)
	if !ok {
		return "", nil, false, false
	}
	if term {
		return text, nil, true, true
	}
	restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], env, carriedNames, carriedTys)
	if len(carriedNames) != 0 && len(restYield) == 0 {
		restYield = formalLoopCarriedValues(env, carriedNames)
	}
	return text + restBody, restYield, bodyTerm, true
}

func emitFormalLoopBodyWithCarried(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, bool) {
	if len(stmts) == 0 {
		if len(carriedNames) != 0 {
			return "", formalLoopCarriedValues(env, carriedNames), false
		}
		return "", nil, false
	}

	if text, yieldValues, ok := emitFormalBareContinueWithCarried(stmts[0], env, carriedNames); ok {
		return text, yieldValues, false
	}

	ifStmt, continuePrefix, ok := matchFormalContinueIf(stmts[0])
	if ok {
		branchEnv := env
		var buf strings.Builder
		if ifStmt.Init != nil {
			branchEnv = env.clone()
			initText, term := emitFormalStmt(ifStmt.Init, branchEnv, nil)
			syncFormalTempID(env, branchEnv)
			if term {
				return initText, nil, true
			}
			buf.WriteString(initText)
			clone := *ifStmt
			clone.Init = nil
			ifStmt = &clone
		}
		condEnv := branchEnv.clone()
		cond, prelude, ok := emitFormalCondition(ifStmt.Cond, condEnv)
		if !ok {
			propagateFormalOuterBindings(branchEnv, condEnv)
			syncFormalTempID(env, branchEnv, condEnv)
			return prelude + emitFormalLinef(ifStmt, branchEnv, "    go.todo %q", "IfStmt_condition"), nil, false
		}
		thenEnv := condEnv.clone()
		thenBody, thenTerm := emitFormalRegionBlock(continuePrefix, thenEnv)
		syncFormalTempID(env, branchEnv, condEnv, thenEnv)
		if thenTerm {
			return "", nil, true
		}
		if condValue, known := formalKnownBoolExpr(ifStmt.Cond, condEnv); known {
			if condValue {
				if len(carriedNames) == 0 {
					return prelude + thenBody, nil, false
				}
				return prelude + thenBody, formalLoopCarriedValues(thenEnv, carriedNames), false
			}
			elseEnv := condEnv.clone()
			elseBody, elseYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[1:], elseEnv, carriedNames, carriedTys)
			syncFormalTempID(env, branchEnv, condEnv, thenEnv, elseEnv)
			if bodyTerm {
				return "", nil, true
			}
			if len(carriedNames) != 0 && len(elseYield) == 0 {
				elseYield = formalLoopCarriedValues(elseEnv, carriedNames)
			}
			return prelude + elseBody, elseYield, false
		}
		elseEnv := condEnv.clone()
		elseBody, elseYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[1:], elseEnv, carriedNames, carriedTys)
		syncFormalTempID(env, branchEnv, condEnv, thenEnv, elseEnv)
		if bodyTerm {
			return "", nil, true
		}

		buf.WriteString(prelude)
		if len(carriedNames) == 0 {
			var ifBuf strings.Builder
			ifBuf.WriteString(fmt.Sprintf("    scf.if %s {\n", cond))
			ifBuf.WriteString(indentBlock(thenBody, 2))
			ifBuf.WriteString("    } else {\n")
			ifBuf.WriteString(indentBlock(elseBody, 2))
			ifBuf.WriteString("    }\n")
			buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
			return buf.String(), nil, false
		}

		result := env.temp("loopcont")
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(carriedTys)), cond, strings.Join(carriedTys, ", ")))
		ifBuf.WriteString(indentBlock(thenBody, 2))
		ifBuf.WriteString(emitFormalYieldLine(formalLoopCarriedValues(thenEnv, carriedNames), carriedTys, branchEnv))
		ifBuf.WriteString("    } else {\n")
		ifBuf.WriteString(indentBlock(elseBody, 2))
		ifBuf.WriteString(emitFormalYieldLine(elseYield, carriedTys, branchEnv))
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
		return buf.String(), formalMultiResultRefs(result, len(carriedTys)), false
	}

	if ifStmt, ok := stmts[0].(*ast.IfStmt); ok {
		attemptEnv := env.clone()
		var next ast.Stmt
		if len(stmts) > 1 {
			next = stmts[1]
		}
		if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[1:], attemptEnv); ok {
			if term {
				syncFormalTempID(env, attemptEnv)
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
			}
			syncFormalTempID(env, attemptEnv)
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, attemptEnv, attemptEnv.resultTypes); ok {
			if term {
				syncFormalTempID(env, attemptEnv)
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
			}
			syncFormalTempID(env, attemptEnv)
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[1:], attemptEnv, attemptEnv.resultTypes); ok {
			if term {
				syncFormalTempID(env, attemptEnv)
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
			}
			syncFormalTempID(env, attemptEnv)
			return text + restBody, restYield, bodyTerm
		}
		syncFormalTempID(env, attemptEnv)
	}
	if text, yieldValues, term, ok := emitFormalLoopReturningStmtWithCarried(stmts, env, carriedNames, carriedTys); ok {
		return text, yieldValues, term
	}

	text, term := emitFormalStmt(stmts[0], env, nil)
	if term {
		return text, nil, true
	}
	restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[1:], env, carriedNames, carriedTys)
	if len(carriedNames) != 0 && len(restYield) == 0 {
		restYield = formalLoopCarriedValues(env, carriedNames)
	}
	return text + restBody, restYield, bodyTerm
}

func emitFormalLoopReturningStmtWithBreak(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, string, bool, bool) {
	text, consumed, term, ok := emitFormalReturningLoopStmt(stmts[0], stmts[1:], env, env.resultTypes)
	if !ok {
		return "", nil, "", false, false
	}
	if term {
		return text, nil, "", true, true
	}
	restBody, restYield, restStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[consumed:], env, carriedNames, carriedTys)
	if len(carriedNames) != 0 && len(restYield) == 0 {
		restYield = formalLoopCarriedValues(env, carriedNames)
	}
	return text + restBody, restYield, restStop, bodyTerm, true
}

func emitFormalLoopBodyWithBreak(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, string, bool) {
	if len(stmts) == 0 {
		stop := env.temp("loopstop")
		text := emitFormalLinef(nil, env, "    %s = arith.constant false", stop)
		if len(carriedNames) != 0 {
			return text, formalLoopCarriedValues(env, carriedNames), stop, false
		}
		return text, nil, stop, false
	}

	if text, yieldValues, stop, ok := emitFormalBareContinueWithBreak(stmts[0], env, carriedNames); ok {
		return text, yieldValues, stop, false
	}

	if ifStmt, continuePrefix, ok := matchFormalContinueIf(stmts[0]); ok {
		branchEnv := env
		var buf strings.Builder
		if ifStmt.Init != nil {
			branchEnv = env.clone()
			initText, term := emitFormalStmt(ifStmt.Init, branchEnv, nil)
			syncFormalTempID(env, branchEnv)
			if term {
				return initText, nil, "", true
			}
			buf.WriteString(initText)
			clone := *ifStmt
			clone.Init = nil
			ifStmt = &clone
		}
		condEnv := branchEnv.clone()
		cond, prelude, ok := emitFormalCondition(ifStmt.Cond, condEnv)
		if !ok {
			propagateFormalOuterBindings(branchEnv, condEnv)
			syncFormalTempID(env, branchEnv, condEnv)
			return prelude + emitFormalLinef(ifStmt, branchEnv, "    go.todo %q", "IfStmt_condition"), nil, "", false
		}
		thenEnv := condEnv.clone()
		thenBody, thenTerm := emitFormalRegionBlock(continuePrefix, thenEnv)
		syncFormalTempID(env, branchEnv, condEnv, thenEnv)
		if thenTerm {
			return "", nil, "", true
		}
		thenStop := thenEnv.temp("loopstop")
		thenText := thenBody + emitFormalLinef(ifStmt, thenEnv, "    %s = arith.constant false", thenStop)
		if condValue, known := formalKnownBoolExpr(ifStmt.Cond, condEnv); known {
			if condValue {
				if len(carriedNames) == 0 {
					return prelude + thenText, nil, thenStop, false
				}
				return prelude + thenText, formalLoopCarriedValues(thenEnv, carriedNames), thenStop, false
			}
			elseEnv := condEnv.clone()
			elseBody, elseYield, elseStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[1:], elseEnv, carriedNames, carriedTys)
			syncFormalTempID(env, branchEnv, condEnv, thenEnv, elseEnv)
			if bodyTerm {
				return "", nil, "", true
			}
			elseBody, elseStop = ensureFormalLoopStopValue(elseBody, elseStop, elseEnv, ifStmt)
			if len(carriedNames) != 0 && len(elseYield) == 0 {
				elseYield = formalLoopCarriedValues(elseEnv, carriedNames)
			}
			return prelude + elseBody, elseYield, elseStop, false
		}
		elseEnv := condEnv.clone()
		elseBody, elseYield, elseStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[1:], elseEnv, carriedNames, carriedTys)
		syncFormalTempID(env, branchEnv, condEnv, thenEnv, elseEnv)
		if bodyTerm {
			return "", nil, "", true
		}
		elseBody, elseStop = ensureFormalLoopStopValue(elseBody, elseStop, elseEnv, ifStmt)

		buf.WriteString(prelude)
		result := env.temp("loopcont")
		if len(carriedNames) == 0 {
			var ifBuf strings.Builder
			ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (i1) {\n", result, cond))
			ifBuf.WriteString(indentBlock(thenText, 2))
			ifBuf.WriteString(emitFormalLinef(ifStmt, branchEnv, "        scf.yield %s : i1", thenStop))
			ifBuf.WriteString("    } else {\n")
			ifBuf.WriteString(indentBlock(elseBody, 2))
			ifBuf.WriteString(emitFormalLinef(ifStmt, branchEnv, "        scf.yield %s : i1", elseStop))
			ifBuf.WriteString("    }\n")
			buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
			return buf.String(), nil, result, false
		}

		resultTypes := append([]string{"i1"}, carriedTys...)
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
		ifBuf.WriteString(indentBlock(thenText, 2))
		ifBuf.WriteString(emitFormalYieldLine(append([]string{thenStop}, formalLoopCarriedValues(thenEnv, carriedNames)...), resultTypes, branchEnv))
		ifBuf.WriteString("    } else {\n")
		ifBuf.WriteString(indentBlock(elseBody, 2))
		ifBuf.WriteString(emitFormalYieldLine(append([]string{elseStop}, elseYield...), resultTypes, branchEnv))
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
		refs := formalMultiResultRefs(result, len(resultTypes))
		return buf.String(), refs[1:], refs[0], false
	}

	if ifStmt, ok := stmts[0].(*ast.IfStmt); ok {
		if breakPrefix, ok := matchFormalBreakIf(ifStmt); ok {
			branchEnv := env
			var buf strings.Builder
			if ifStmt.Init != nil {
				branchEnv = env.clone()
				initText, term := emitFormalStmt(ifStmt.Init, branchEnv, nil)
				syncFormalTempID(env, branchEnv)
				if term {
					return initText, nil, "", true
				}
				buf.WriteString(initText)
				clone := *ifStmt
				clone.Init = nil
				ifStmt = &clone
			}
			condEnv := branchEnv.clone()
			cond, prelude, ok := emitFormalCondition(ifStmt.Cond, condEnv)
			if !ok {
				propagateFormalOuterBindings(branchEnv, condEnv)
				syncFormalTempID(env, branchEnv, condEnv)
				return prelude + emitFormalLinef(ifStmt, branchEnv, "    go.todo %q", "IfStmt_condition"), nil, "", false
			}
			thenEnv := condEnv.clone()
			thenBody, thenTerm := emitFormalRegionBlock(breakPrefix, thenEnv)
			syncFormalTempID(env, branchEnv, condEnv, thenEnv)
			if thenTerm {
				return "", nil, "", true
			}
			thenStop := thenEnv.temp("loopstop")
			thenText := thenBody + emitFormalLinef(ifStmt, thenEnv, "    %s = arith.constant true", thenStop)
			elseEnv := condEnv.clone()
			elseBody, elseYield, elseStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[1:], elseEnv, carriedNames, carriedTys)
			syncFormalTempID(env, branchEnv, condEnv, thenEnv, elseEnv)
			if bodyTerm {
				return "", nil, "", true
			}
			elseBody, elseStop = ensureFormalLoopStopValue(elseBody, elseStop, elseEnv, ifStmt)

			buf.WriteString(prelude)
			result := env.temp("loopbreak")
			if len(carriedNames) == 0 {
				var ifBuf strings.Builder
				ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (i1) {\n", result, cond))
				ifBuf.WriteString(indentBlock(thenText, 2))
				ifBuf.WriteString(emitFormalLinef(ifStmt, branchEnv, "        scf.yield %s : i1", thenStop))
				ifBuf.WriteString("    } else {\n")
				ifBuf.WriteString(indentBlock(elseBody, 2))
				ifBuf.WriteString(emitFormalLinef(ifStmt, branchEnv, "        scf.yield %s : i1", elseStop))
				ifBuf.WriteString("    }\n")
				buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
				return buf.String(), nil, result, false
			}

			resultTypes := append([]string{"i1"}, carriedTys...)
			var ifBuf strings.Builder
			ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
			ifBuf.WriteString(indentBlock(thenText, 2))
			ifBuf.WriteString(emitFormalYieldLine(append([]string{thenStop}, formalLoopCarriedValues(thenEnv, carriedNames)...), resultTypes, branchEnv))
			ifBuf.WriteString("    } else {\n")
			ifBuf.WriteString(indentBlock(elseBody, 2))
			ifBuf.WriteString(emitFormalYieldLine(append([]string{elseStop}, elseYield...), resultTypes, branchEnv))
			ifBuf.WriteString("    }\n")
			buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), ifStmt, branchEnv))
			refs := formalMultiResultRefs(result, len(resultTypes))
			return buf.String(), refs[1:], refs[0], false
		}
	}

	if branch, ok := stmts[0].(*ast.BranchStmt); ok && branch.Tok == token.BREAK && branch.Label == nil {
		stop := env.temp("loopstop")
		text := emitFormalLinef(branch, env, "    %s = arith.constant true", stop)
		if len(carriedNames) != 0 {
			return text, formalLoopCarriedValues(env, carriedNames), stop, false
		}
		return text, nil, stop, false
	}

	if text, yieldValues, stop, term, ok := emitFormalLeadingIfWithBreak(stmts, env, carriedNames, carriedTys); ok {
		return text, yieldValues, stop, term
	}
	if text, yieldValues, stop, term, ok := emitFormalLoopReturningStmtWithBreak(stmts, env, carriedNames, carriedTys); ok {
		return text, yieldValues, stop, term
	}

	text, term := emitFormalStmt(stmts[0], env, nil)
	if term {
		return text, nil, "", true
	}
	restBody, restYield, restStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[1:], env, carriedNames, carriedTys)
	if len(carriedNames) != 0 && len(restYield) == 0 {
		restYield = formalLoopCarriedValues(env, carriedNames)
	}
	restBody, restStop = ensureFormalLoopStopValue(restBody, restStop, env, stmts[0])
	return text + restBody, restYield, restStop, bodyTerm
}

func emitFormalLeadingIfWithBreak(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, string, bool, bool) {
	if len(stmts) == 0 {
		return "", nil, "", false, false
	}
	ifStmt, ok := stmts[0].(*ast.IfStmt)
	if !ok {
		return "", nil, "", false, false
	}

	attemptEnv := env.clone()
	var next ast.Stmt
	if len(stmts) > 1 {
		next = stmts[1]
	}
	if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[1:], attemptEnv); ok {
		if term {
			syncFormalTempID(env, attemptEnv)
			return text, nil, "", true, true
		}
		restBody, restYield, restStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
		if len(carriedNames) != 0 && len(restYield) == 0 {
			restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
		}
		syncFormalTempID(env, attemptEnv)
		return text + restBody, restYield, restStop, bodyTerm, true
	}
	if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, attemptEnv, attemptEnv.resultTypes); ok {
		if term {
			syncFormalTempID(env, attemptEnv)
			return text, nil, "", true, true
		}
		restBody, restYield, restStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
		if len(carriedNames) != 0 && len(restYield) == 0 {
			restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
		}
		syncFormalTempID(env, attemptEnv)
		return text + restBody, restYield, restStop, bodyTerm, true
	}
	if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[1:], attemptEnv, attemptEnv.resultTypes); ok {
		if term {
			syncFormalTempID(env, attemptEnv)
			return text, nil, "", true, true
		}
		restBody, restYield, restStop, bodyTerm := emitFormalLoopBodyWithBreak(stmts[consumed:], attemptEnv, carriedNames, carriedTys)
		if len(carriedNames) != 0 && len(restYield) == 0 {
			restYield = formalLoopCarriedValues(attemptEnv, carriedNames)
		}
		syncFormalTempID(env, attemptEnv)
		return text + restBody, restYield, restStop, bodyTerm, true
	}
	if text, yieldValues, stop, ok := emitFormalLoopIfWithBreak(ifStmt, stmts[1:], attemptEnv, carriedNames, carriedTys); ok {
		syncFormalTempID(env, attemptEnv)
		return text, yieldValues, stop, false, true
	}
	syncFormalTempID(env, attemptEnv)
	return "", nil, "", false, false
}

func emitFormalLoopIfWithBreak(s *ast.IfStmt, rest []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, string, bool) {
	if s == nil {
		return "", nil, "", false
	}
	if s.Init != nil {
		scopedEnv := env.clone()
		initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
		if term {
			syncFormalTempID(env, scopedEnv)
			return "", nil, "", false
		}
		clone := *s
		clone.Init = nil
		body, yieldValues, stop, ok := emitFormalLoopIfWithBreak(&clone, rest, scopedEnv, carriedNames, carriedTys)
		syncFormalTempID(env, scopedEnv)
		if !ok {
			return "", nil, "", false
		}
		return initText + body, yieldValues, stop, false
	}

	condEnv := env.clone()
	cond, prelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		propagateFormalOuterBindings(env, condEnv)
		syncFormalTempID(env, condEnv)
		return "", nil, "", false
	}

	thenEnv := condEnv.clone()
	thenStmts := append(append([]ast.Stmt(nil), s.Body.List...), rest...)
	thenBody, thenYield, thenStop, thenTerm := emitFormalLoopBodyWithBreak(thenStmts, thenEnv, carriedNames, carriedTys)
	if thenTerm {
		syncFormalTempID(env, condEnv, thenEnv)
		return "", nil, "", false
	}
	thenBody, thenStop = ensureFormalLoopStopValue(thenBody, thenStop, thenEnv, s)

	elseEnv := condEnv.clone()
	elseStmts := append([]ast.Stmt(nil), rest...)
	switch elseNode := s.Else.(type) {
	case nil:
	case *ast.BlockStmt:
		elseStmts = append(append([]ast.Stmt(nil), elseNode.List...), rest...)
	case *ast.IfStmt:
		elseStmts = append([]ast.Stmt{elseNode}, rest...)
	default:
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return "", nil, "", false
	}
	elseBody, elseYield, elseStop, elseTerm := emitFormalLoopBodyWithBreak(elseStmts, elseEnv, carriedNames, carriedTys)
	if elseTerm {
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return "", nil, "", false
	}
	elseBody, elseStop = ensureFormalLoopStopValue(elseBody, elseStop, elseEnv, s)

	var buf strings.Builder
	buf.WriteString(prelude)
	result := env.temp("loopif")
	if len(carriedNames) == 0 {
		var ifBuf strings.Builder
		ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (i1) {\n", result, cond))
		ifBuf.WriteString(indentBlock(thenBody, 2))
		ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : i1", thenStop))
		ifBuf.WriteString("    } else {\n")
		ifBuf.WriteString(indentBlock(elseBody, 2))
		ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : i1", elseStop))
		ifBuf.WriteString("    }\n")
		buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return buf.String(), nil, result, true
	}

	resultTypes := append([]string{"i1"}, carriedTys...)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
	ifBuf.WriteString(indentBlock(thenBody, 2))
	ifBuf.WriteString(emitFormalYieldLine(append([]string{thenStop}, thenYield...), resultTypes, env))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(elseBody, 2))
	ifBuf.WriteString(emitFormalYieldLine(append([]string{elseStop}, elseYield...), resultTypes, env))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
	refs := formalMultiResultRefs(result, len(resultTypes))
	syncFormalTempID(env, condEnv, thenEnv, elseEnv)
	return buf.String(), refs[1:], refs[0], true
}

func ensureFormalLoopStopValue(body string, stop string, env *formalEnv, node ast.Node) (string, string) {
	if stop != "" {
		return body, stop
	}
	stop = env.temp("loopstop")
	return body + emitFormalLinef(node, env, "    %s = arith.constant false", stop), stop
}

func formalLoopCarriedValues(env *formalEnv, carriedNames []string) []string {
	values := make([]string, 0, len(carriedNames))
	for _, name := range carriedNames {
		values = append(values, env.use(name))
	}
	return values
}

func matchFormalContinueIf(stmt ast.Stmt) (*ast.IfStmt, []ast.Stmt, bool) {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || len(ifStmt.Body.List) == 0 {
		return nil, nil, false
	}
	last := ifStmt.Body.List[len(ifStmt.Body.List)-1]
	branch, ok := last.(*ast.BranchStmt)
	if !ok || branch.Tok != token.CONTINUE || branch.Label != nil {
		return nil, nil, false
	}
	return ifStmt, ifStmt.Body.List[:len(ifStmt.Body.List)-1], true
}

func matchFormalBreakIf(stmt ast.Stmt) ([]ast.Stmt, bool) {
	ifStmt, ok := stmt.(*ast.IfStmt)
	if !ok || ifStmt.Else != nil || len(ifStmt.Body.List) == 0 {
		return nil, false
	}
	last := ifStmt.Body.List[len(ifStmt.Body.List)-1]
	branch, ok := last.(*ast.BranchStmt)
	if !ok || branch.Tok != token.BREAK || branch.Label != nil {
		return nil, false
	}
	return ifStmt.Body.List[:len(ifStmt.Body.List)-1], true
}

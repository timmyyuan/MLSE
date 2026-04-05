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

func emitFormalLoopBodyWithCarried(stmts []ast.Stmt, env *formalEnv, carriedNames []string, carriedTys []string) (string, []string, bool) {
	if len(stmts) == 0 {
		if len(carriedNames) != 0 {
			return "", formalLoopCarriedValues(env, carriedNames), false
		}
		return "", nil, false
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
		cond, prelude, ok := emitFormalCondition(ifStmt.Cond, branchEnv)
		if !ok {
			return prelude + emitFormalLinef(ifStmt, branchEnv, "    go.todo %q", "IfStmt_condition"), nil, false
		}
		thenEnv := branchEnv.clone()
		thenBody, thenTerm := emitFormalRegionBlock(continuePrefix, thenEnv)
		syncFormalTempID(env, branchEnv, thenEnv)
		if thenTerm {
			return "", nil, true
		}
		elseEnv := branchEnv.clone()
		elseBody, elseYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[1:], elseEnv, carriedNames, carriedTys)
		syncFormalTempID(env, branchEnv, thenEnv, elseEnv)
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
		var next ast.Stmt
		if len(stmts) > 1 {
			next = stmts[1]
		}
		if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[1:], env); ok {
			if term {
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], env, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(env, carriedNames)
			}
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, env, env.resultTypes); ok {
			if term {
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], env, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(env, carriedNames)
			}
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[1:], env, env.resultTypes); ok {
			if term {
				return text, nil, true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBodyWithCarried(stmts[consumed:], env, carriedNames, carriedTys)
			if len(carriedNames) != 0 && len(restYield) == 0 {
				restYield = formalLoopCarriedValues(env, carriedNames)
			}
			return text + restBody, restYield, bodyTerm
		}
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

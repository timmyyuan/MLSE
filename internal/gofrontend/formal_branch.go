package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// emitFormalLoopBody lowers loop bodies, including `continue`-shaped `if` regions with carried values.
func emitFormalLoopBody(stmts []ast.Stmt, env *formalEnv, carriedName string, carriedTy string) (string, string, bool) {
	if len(stmts) == 0 {
		if carriedName != "" {
			return "", env.use(carriedName), false
		}
		return "", "", false
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
				return initText, "", true
			}
			buf.WriteString(initText)
			clone := *ifStmt
			clone.Init = nil
			ifStmt = &clone
		}
		cond, prelude, ok := emitFormalCondition(ifStmt.Cond, branchEnv)
		if !ok {
			return prelude + "    go.todo \"IfStmt_condition\"\n", "", false
		}
		thenEnv := branchEnv.clone()
		thenBody, thenTerm := emitFormalRegionBlock(continuePrefix, thenEnv)
		syncFormalTempID(env, branchEnv, thenEnv)
		if thenTerm {
			return "", "", true
		}
		elseEnv := branchEnv.clone()
		elseBody, elseYield, bodyTerm := emitFormalLoopBody(stmts[1:], elseEnv, carriedName, carriedTy)
		syncFormalTempID(env, branchEnv, thenEnv, elseEnv)
		if bodyTerm {
			return "", "", true
		}

		buf.WriteString(prelude)
		if carriedName == "" {
			buf.WriteString(fmt.Sprintf("    scf.if %s {\n", cond))
			buf.WriteString(indentBlock(thenBody, 2))
			buf.WriteString("    } else {\n")
			buf.WriteString(indentBlock(elseBody, 2))
			buf.WriteString("    }\n")
			return buf.String(), "", false
		}

		result := env.temp("loopcont")
		buf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", result, cond, carriedTy))
		buf.WriteString(indentBlock(thenBody, 2))
		buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", thenEnv.use(carriedName), carriedTy))
		buf.WriteString("    } else {\n")
		buf.WriteString(indentBlock(elseBody, 2))
		buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", elseYield, carriedTy))
		buf.WriteString("    }\n")
		return buf.String(), result, false
	}

	if ifStmt, ok := stmts[0].(*ast.IfStmt); ok {
		var next ast.Stmt
		if len(stmts) > 1 {
			next = stmts[1]
		}
		if text, consumed, term, ok := emitFormalVoidReturningIfStmt(ifStmt, stmts[1:], env); ok {
			if term {
				return text, "", true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBody(stmts[consumed:], env, carriedName, carriedTy)
			if carriedName != "" && restYield == "" {
				restYield = env.use(carriedName)
			}
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, env, env.resultTypes); ok {
			if term {
				return text, "", true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBody(stmts[consumed:], env, carriedName, carriedTy)
			if carriedName != "" && restYield == "" {
				restYield = env.use(carriedName)
			}
			return text + restBody, restYield, bodyTerm
		}
		if text, consumed, term, ok := emitFormalReturningIfStmt(ifStmt, stmts[1:], env, env.resultTypes); ok {
			if term {
				return text, "", true
			}
			restBody, restYield, bodyTerm := emitFormalLoopBody(stmts[consumed:], env, carriedName, carriedTy)
			if carriedName != "" && restYield == "" {
				restYield = env.use(carriedName)
			}
			return text + restBody, restYield, bodyTerm
		}
	}

	text, term := emitFormalStmt(stmts[0], env, nil)
	if term {
		return text, "", true
	}
	restBody, restYield, bodyTerm := emitFormalLoopBody(stmts[1:], env, carriedName, carriedTy)
	if carriedName != "" && restYield == "" {
		restYield = env.use(carriedName)
	}
	return text + restBody, restYield, bodyTerm
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

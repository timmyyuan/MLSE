package gofrontend

import (
	"go/ast"
	"strings"
)

// emitFormalIfStmtWithInit lowers `if init; cond { ... }` by scoping the init before the structured if.
func emitFormalIfStmtWithInit(s *ast.IfStmt, env *formalEnv) string {
	if s == nil || s.Init == nil {
		return ""
	}

	scopedEnv := env.clone()
	initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
	if term {
		syncFormalTempID(env, scopedEnv)
		return initText
	}

	clone := *s
	clone.Init = nil
	bodyText := emitFormalIfStmt(&clone, scopedEnv)
	propagateFormalOuterBindings(env, scopedEnv)
	syncFormalTempID(env, scopedEnv)
	return initText + bodyText
}

func propagateFormalOuterBindings(target *formalEnv, source *formalEnv) {
	if target == nil || source == nil {
		return
	}
	for name, binding := range target.locals {
		sourceBinding, ok := source.locals[name]
		if !ok {
			continue
		}
		binding.current = sourceBinding.current
		binding.ty = sourceBinding.ty
		if sourceBinding.funcSig != nil {
			binding.funcSig = cloneFormalFuncSig(*sourceBinding.funcSig)
		} else {
			binding.funcSig = nil
		}
	}
}

// emitFormalVoidReturningIfStmt recognizes `if` regions that end the surrounding void function.
func emitFormalVoidReturningIfStmt(s *ast.IfStmt, remaining []ast.Stmt, env *formalEnv) (string, int, bool, bool) {
	if s == nil {
		return "", 0, false, false
	}
	if s.Init != nil {
		scopedEnv := env.clone()
		initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
		if term {
			syncFormalTempID(env, scopedEnv)
			return initText, 1, true, true
		}
		clone := *s
		clone.Init = nil
		bodyText, consumed, bodyTerm, ok := emitFormalVoidReturningIfStmt(&clone, remaining, scopedEnv)
		if !ok {
			syncFormalTempID(env, scopedEnv)
			return "", 0, false, false
		}
		syncFormalTempID(env, scopedEnv)
		return initText + bodyText, consumed, bodyTerm, true
	}

	elseStmts := remaining
	consumed := len(remaining) + 1
	if s.Else != nil {
		elseBlock, ok := s.Else.(*ast.BlockStmt)
		if !ok {
			return "", 0, false, false
		}
		elseStmts = elseBlock.List
		consumed = 1
	}
	cond, prelude, ok := emitFormalCondition(s.Cond, env)
	if !ok {
		return "", 0, false, false
	}

	thenEnv := env.clone()
	thenBody, ok := emitFormalVoidReturningRegion(s.Body.List, thenEnv)
	if !ok {
		syncFormalTempID(env, thenEnv)
		return "", 0, false, false
	}

	elseEnv := env.clone()
	elseBody, ok := emitFormalVoidBranchRegion(elseStmts, elseEnv)
	if !ok {
		syncFormalTempID(env, thenEnv, elseEnv)
		return "", 0, false, false
	}

	var buf strings.Builder
	buf.WriteString(prelude)
	buf.WriteString("    scf.if " + cond + " {\n")
	buf.WriteString(indentBlock(thenBody, 2))
	if len(elseStmts) != 0 || s.Else != nil {
		buf.WriteString("    } else {\n")
		buf.WriteString(indentBlock(elseBody, 2))
		buf.WriteString("    }\n")
	} else {
		buf.WriteString("    }\n")
	}
	buf.WriteString("    return\n")
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String(), consumed, true, true
}

func emitFormalVoidReturningRegion(stmts []ast.Stmt, env *formalEnv) (string, bool) {
	if len(stmts) == 0 {
		return "", false
	}
	ret, ok := stmts[len(stmts)-1].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 0 {
		return "", false
	}
	body, terminated := emitFormalRegionBlock(stmts[:len(stmts)-1], env)
	if terminated {
		return "", false
	}
	return body, true
}

func emitFormalVoidBranchRegion(stmts []ast.Stmt, env *formalEnv) (string, bool) {
	if len(stmts) == 0 {
		return "", true
	}
	if body, ok := emitFormalVoidReturningRegion(stmts, env); ok {
		return body, true
	}
	body, terminated := emitFormalRegionBlock(stmts, env)
	if terminated {
		return "", false
	}
	return body, true
}

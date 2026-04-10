package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

type formalReturningRegionCandidate struct {
	stmts         []ast.Stmt
	usesRemaining bool
}

func emitFormalTerminatingIfStmt(s *ast.IfStmt, next ast.Stmt, env *formalEnv, resultTypes []string) (string, int, bool, bool) {
	if s.Init != nil || len(resultTypes) != 1 {
		return "", 0, false, false
	}

	thenExpr, ok := extractSingleReturnExpr(s.Body.List)
	if !ok {
		return "", 0, false, false
	}

	elseExpr := ast.Expr(nil)
	consumed := 1
	switch elseNode := s.Else.(type) {
	case nil:
		ret, ok := next.(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			return "", 0, false, false
		}
		elseExpr = ret.Results[0]
		consumed = 2
	case *ast.BlockStmt:
		elseExpr, ok = extractSingleReturnExpr(elseNode.List)
		if !ok {
			return "", 0, false, false
		}
	default:
		return "", 0, false, false
	}

	condEnv := env.clone()
	cond, prelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		syncFormalTempID(env, condEnv)
		return "", 0, false, false
	}

	resultTy := resultTypes[0]
	thenEnv := condEnv.clone()
	thenValue, thenTy, thenPrelude := emitFormalExpr(thenExpr, resultTy, thenEnv)
	elseEnv := condEnv.clone()
	elseValue, elseTy, elsePrelude := emitFormalExpr(elseExpr, resultTy, elseEnv)
	if normalizeFormalType(thenTy) != normalizeFormalType(resultTy) || normalizeFormalType(elseTy) != normalizeFormalType(resultTy) {
		syncFormalTempID(env, condEnv, thenEnv, elseEnv)
		return "", 0, false, false
	}
	syncFormalTempID(env, condEnv, thenEnv, elseEnv)
	result := env.temp("ifret")
	var buf strings.Builder
	buf.WriteString(prelude)
	var ifBuf strings.Builder
	ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", result, cond, resultTy))
	ifBuf.WriteString(indentBlock(thenPrelude, 2))
	ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : %s", thenValue, resultTy))
	ifBuf.WriteString("    } else {\n")
	ifBuf.WriteString(indentBlock(elsePrelude, 2))
	ifBuf.WriteString(emitFormalLinef(s, env, "        scf.yield %s : %s", elseValue, resultTy))
	ifBuf.WriteString("    }\n")
	buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
	buf.WriteString(emitFormalLinef(s, env, "    return %s : %s", result, resultTy))
	return buf.String(), consumed, true, true
}

func emitFormalReturningIfStmt(s *ast.IfStmt, remaining []ast.Stmt, env *formalEnv, resultTypes []string) (string, int, bool, bool) {
	if len(resultTypes) == 0 {
		return "", 0, false, false
	}
	body, values, types, consumed, ok := emitFormalReturningIfRegion(s, remaining, env, resultTypes)
	if !ok {
		return "", 0, false, false
	}
	return body + emitFormalReturnLine(values, types, env), consumed, true, true
}

func emitFormalReturningRegion(stmts []ast.Stmt, env *formalEnv, resultTypes []string) (string, []string, []string, bool) {
	for i, stmt := range stmts {
		attemptEnv := env.clone()
		prefix, terminated := emitFormalRegionBlock(stmts[:i], attemptEnv)
		if terminated {
			syncFormalTempID(env, attemptEnv)
			return "", nil, nil, false
		}
		if ifStmt, ok := stmt.(*ast.IfStmt); ok {
			body, values, types, _, ok := emitFormalReturningIfRegion(ifStmt, stmts[i+1:], attemptEnv, resultTypes)
			if ok {
				syncFormalTempID(env, attemptEnv)
				return prefix + body, values, types, true
			}
		}
		body, values, types, _, ok := emitFormalReturningLoopRegion(stmt, stmts[i+1:], attemptEnv, resultTypes)
		if ok {
			syncFormalTempID(env, attemptEnv)
			return prefix + body, values, types, true
		}
		syncFormalTempID(env, attemptEnv)
	}
	prefix, exprs, ok := extractTrailingReturnExprs(stmts)
	if !ok {
		if len(stmts) == 0 {
			return "", nil, nil, false
		}
		prefix := stmts[:len(stmts)-1]
		body, terminated := emitFormalRegionBlock(prefix, env)
		if terminated {
			return "", nil, nil, false
		}
		termText, ok := emitFormalTerminatingStmt(stmts[len(stmts)-1], env)
		if !ok {
			return "", nil, nil, false
		}
		var (
			values []string
			types  []string
			buf    strings.Builder
		)
		for _, ty := range resultTypes {
			value, prelude := emitFormalZeroValue(ty, env)
			buf.WriteString(prelude)
			values = append(values, value)
			types = append(types, ty)
		}
		return body + termText + buf.String(), values, types, true
	}
	body, terminated := emitFormalRegionBlock(prefix, env)
	if terminated {
		return "", nil, nil, false
	}
	values, types, prelude, ok := emitFormalReturnExprOperands(exprs, resultTypes, env)
	if !ok {
		return "", nil, nil, false
	}
	return body + prelude, values, types, true
}

func emitFormalReturningIfRegion(s *ast.IfStmt, remaining []ast.Stmt, env *formalEnv, resultTypes []string) (string, []string, []string, int, bool) {
	if s == nil {
		return "", nil, nil, 0, false
	}
	if s.Init != nil {
		scopedEnv := env.clone()
		initText, term := emitFormalStmt(s.Init, scopedEnv, nil)
		if term {
			syncFormalTempID(env, scopedEnv)
			return "", nil, nil, 0, false
		}
		clone := *s
		clone.Init = nil
		body, values, types, consumed, ok := emitFormalReturningIfRegion(&clone, remaining, scopedEnv, resultTypes)
		if !ok {
			syncFormalTempID(env, scopedEnv)
			return "", nil, nil, 0, false
		}
		syncFormalTempID(env, scopedEnv)
		return initText + body, values, types, consumed, true
	}

	condEnv := env.clone()
	cond, prelude, ok := emitFormalCondition(s.Cond, condEnv)
	if !ok {
		syncFormalTempID(env, condEnv)
		return "", nil, nil, 0, false
	}

	thenCandidates := formalReturningRegionCandidates(s.Body.List, remaining)
	elseCandidates, ok := formalReturningElseCandidates(s, remaining)
	if !ok {
		syncFormalTempID(env, condEnv)
		return "", nil, nil, 0, false
	}
	for _, thenCandidate := range thenCandidates {
		thenEnv := condEnv.clone()
		thenBody, thenValues, thenTypes, ok := emitFormalReturningRegion(thenCandidate.stmts, thenEnv, resultTypes)
		if !ok {
			syncFormalTempID(env, condEnv, thenEnv)
			continue
		}
		for _, elseCandidate := range elseCandidates {
			elseEnv := condEnv.clone()
			elseBody, elseValues, elseTypes, ok := emitFormalReturningRegion(elseCandidate.stmts, elseEnv, resultTypes)
			if !ok {
				syncFormalTempID(env, condEnv, thenEnv, elseEnv)
				continue
			}
			syncFormalTempID(env, condEnv, thenEnv, elseEnv)
			result := env.temp("ifret")
			var buf strings.Builder
			buf.WriteString(prelude)
			var ifBuf strings.Builder
			ifBuf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
			ifBuf.WriteString(indentBlock(thenBody, 2))
			ifBuf.WriteString(emitFormalYieldLine(thenValues, thenTypes, env))
			ifBuf.WriteString("    } else {\n")
			ifBuf.WriteString(indentBlock(elseBody, 2))
			ifBuf.WriteString(emitFormalYieldLine(elseValues, elseTypes, env))
			ifBuf.WriteString("    }\n")
			buf.WriteString(annotateFormalStructuredOp(ifBuf.String(), s, env))
			consumed := 1
			if thenCandidate.usesRemaining || elseCandidate.usesRemaining {
				consumed = len(remaining) + 1
			}
			return buf.String(), formalMultiResultRefs(result, len(resultTypes)), append([]string(nil), resultTypes...), consumed, true
		}
	}
	syncFormalTempID(env, condEnv)
	return "", nil, nil, 0, false
}

func formalReturningRegionCandidates(base []ast.Stmt, remaining []ast.Stmt) []formalReturningRegionCandidate {
	candidates := []formalReturningRegionCandidate{{
		stmts: append([]ast.Stmt(nil), base...),
	}}
	if len(remaining) != 0 {
		candidates = append(candidates, formalReturningRegionCandidate{
			stmts:         append(append([]ast.Stmt(nil), base...), remaining...),
			usesRemaining: true,
		})
	}
	return candidates
}

func formalReturningElseCandidates(s *ast.IfStmt, remaining []ast.Stmt) ([]formalReturningRegionCandidate, bool) {
	if s == nil {
		return nil, false
	}
	if s.Else == nil {
		if len(remaining) == 0 {
			return nil, false
		}
		return []formalReturningRegionCandidate{{
			stmts:         append([]ast.Stmt(nil), remaining...),
			usesRemaining: true,
		}}, true
	}
	elseBlock, ok := s.Else.(*ast.BlockStmt)
	if !ok {
		return nil, false
	}
	return formalReturningRegionCandidates(elseBlock.List, remaining), true
}

func emitFormalFallbackReturn(resultTypes []string, env *formalEnv) string {
	return emitFormalReturnValues(resultTypes, env)
}

func emitFormalReturnValues(resultTypes []string, env *formalEnv) string {
	if len(resultTypes) == 0 {
		return emitFormalLinef(nil, env, "    return")
	}
	var (
		values []string
		buf    strings.Builder
	)
	for _, ty := range resultTypes {
		value, prelude := emitFormalZeroValue(ty, env)
		buf.WriteString(prelude)
		values = append(values, value)
	}
	buf.WriteString(emitFormalReturnLine(values, resultTypes, env))
	return buf.String()
}

func emitFormalReturnLine(values []string, types []string, env *formalEnv) string {
	return emitFormalLinef(nil, env, "    return %s : %s", strings.Join(values, ", "), strings.Join(types, ", "))
}

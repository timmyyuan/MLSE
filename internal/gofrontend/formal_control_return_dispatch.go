package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

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

	cond, prelude, ok := emitFormalCondition(s.Cond, env)
	if !ok {
		return "", 0, false, false
	}

	resultTy := resultTypes[0]
	thenEnv := env.clone()
	thenValue, thenTy, thenPrelude := emitFormalExpr(thenExpr, resultTy, thenEnv)
	elseEnv := env.clone()
	elseValue, elseTy, elsePrelude := emitFormalExpr(elseExpr, resultTy, elseEnv)
	if normalizeFormalType(thenTy) != normalizeFormalType(resultTy) || normalizeFormalType(elseTy) != normalizeFormalType(resultTy) {
		syncFormalTempID(env, thenEnv, elseEnv)
		return "", 0, false, false
	}
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
	syncFormalTempID(env, thenEnv, elseEnv)
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
		return "", nil, nil, false
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

	elseStmts := remaining
	consumed := len(remaining) + 1
	if s.Else != nil {
		elseBlock, ok := s.Else.(*ast.BlockStmt)
		if !ok {
			return "", nil, nil, 0, false
		}
		elseStmts = elseBlock.List
		consumed = 1
	}
	if len(elseStmts) == 0 {
		return "", nil, nil, 0, false
	}

	cond, prelude, ok := emitFormalCondition(s.Cond, env)
	if !ok {
		return "", nil, nil, 0, false
	}

	thenEnv := env.clone()
	thenBody, thenValues, thenTypes, ok := emitFormalReturningRegion(s.Body.List, thenEnv, resultTypes)
	if !ok {
		syncFormalTempID(env, thenEnv)
		return "", nil, nil, 0, false
	}

	elseEnv := env.clone()
	elseBody, elseValues, elseTypes, ok := emitFormalReturningRegion(elseStmts, elseEnv, resultTypes)
	if !ok {
		syncFormalTempID(env, thenEnv, elseEnv)
		return "", nil, nil, 0, false
	}

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
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String(), formalMultiResultRefs(result, len(resultTypes)), append([]string(nil), resultTypes...), consumed, true
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

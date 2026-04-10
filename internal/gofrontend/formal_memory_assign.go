package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type formalMultiResultCallSpec struct {
	base      string
	callee    string
	args      []string
	argTys    []string
	resultTys []string
	indirect  bool
}

// emitFormalAssignStmt lowers Go assignments into SSA rebinding or helper-based updates.
func emitFormalAssignStmt(s *ast.AssignStmt, env *formalEnv) string {
	if isFormalCompoundAssignToken(s.Tok) {
		return emitFormalCompoundAssignStmt(s, env)
	}
	if len(s.Rhs) == 1 && len(s.Lhs) > 1 {
		return emitFormalExpandedAssignStmt(s, env)
	}
	if len(s.Lhs) != len(s.Rhs) {
		return emitFormalLinef(s, env, "    go.todo %q", "assign_arity_mismatch")
	}

	var buf strings.Builder
	for i := range s.Lhs {
		if ident, ok := s.Lhs[i].(*ast.Ident); ok {
			if ident.Name == "_" {
				_, _, prelude := emitFormalExpr(s.Rhs[i], "", env)
				buf.WriteString(prelude)
				continue
			}

			hint := env.typeOf(ident.Name)
			if s.Tok == token.DEFINE && hint == formalOpaqueType("value") {
				hint = inferFormalExprType(s.Rhs[i], env)
			}
			value, ty, prelude := emitFormalExpr(s.Rhs[i], hint, env)
			buf.WriteString(prelude)
			boolConst := ""
			if valueBool, ok := formalKnownBoolExpr(s.Rhs[i], env); ok {
				if valueBool {
					boolConst = "true"
				} else {
					boolConst = "false"
				}
			}
			switch s.Tok {
			case token.DEFINE:
				env.defineOrAssign(ident.Name, ty)
			case token.ASSIGN:
				env.assign(ident.Name, ty)
			default:
				env.assign(ident.Name, ty)
			}
			env.bindValueWithBool(ident.Name, value, ty, boolConst)
			continue
		}

		hint := formalAssignTargetType(s.Lhs[i], env)
		if isFormalOpaquePlaceholderType(hint) {
			hint = inferFormalExprType(s.Rhs[i], env)
		}
		value, ty, prelude := emitFormalExpr(s.Rhs[i], hint, env)
		buf.WriteString(prelude)
		assignText, ok := emitFormalAssignTargetValue(s.Lhs[i], value, ty, env)
		if !ok {
			buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "assign_target"))
			continue
		}
		buf.WriteString(assignText)
	}
	return buf.String()
}

func emitFormalCompoundAssignStmt(s *ast.AssignStmt, env *formalEnv) string {
	if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
		return emitFormalLinef(s, env, "    go.todo %q", "compound_assign_arity_mismatch")
	}
	op, ok := formalCompoundAssignBinaryToken(s.Tok)
	if !ok {
		return emitFormalLinef(s, env, "    go.todo %q", "compound_assign")
	}

	hint := formalAssignTargetType(s.Lhs[0], env)
	currentValue, currentTy, currentPrelude := emitFormalExpr(s.Lhs[0], hint, env)
	rhsHint := currentTy
	if isFormalOpaquePlaceholderType(rhsHint) {
		rhsHint = inferFormalExprType(s.Rhs[0], env)
	}
	rhsValue, rhsTy, rhsPrelude := emitFormalExpr(s.Rhs[0], rhsHint, env)
	nextValue, nextTy, nextPrelude, ok := emitFormalBinaryOpValues(op, formalBinaryOperands{
		lhs:   currentValue,
		lhsTy: currentTy,
		rhs:   rhsValue,
		rhsTy: rhsTy,
	}, s, env)
	if !ok {
		return currentPrelude + rhsPrelude + emitFormalLinef(s, env, "    go.todo %q", "compound_assign")
	}
	assignText, ok := emitFormalAssignTargetValue(s.Lhs[0], nextValue, nextTy, env)
	if !ok {
		return currentPrelude + rhsPrelude + nextPrelude + emitFormalLinef(s, env, "    go.todo %q", "assign_target")
	}
	return currentPrelude + rhsPrelude + nextPrelude + assignText
}

func isFormalCompoundAssignToken(tok token.Token) bool {
	switch tok {
	case token.ADD_ASSIGN, token.SUB_ASSIGN:
		return true
	default:
		return false
	}
}

func formalCompoundAssignBinaryToken(tok token.Token) (token.Token, bool) {
	switch tok {
	case token.ADD_ASSIGN:
		return token.ADD, true
	case token.SUB_ASSIGN:
		return token.SUB, true
	default:
		return token.ILLEGAL, false
	}
}

func emitFormalExpandedAssignStmt(s *ast.AssignStmt, env *formalEnv) string {
	var buf strings.Builder

	if values, types, prelude, ok := emitFormalExpandedAssignExpr(s.Rhs[0], env); ok && len(values) == len(s.Lhs) && len(types) == len(s.Lhs) {
		buf.WriteString(prelude)
		for i, lhs := range s.Lhs {
			ident, ok := lhs.(*ast.Ident)
			if ok {
				if ident.Name == "_" {
					continue
				}
				switch s.Tok {
				case token.DEFINE:
					env.defineOrAssign(ident.Name, types[i])
				case token.ASSIGN:
					env.assign(ident.Name, types[i])
				default:
					env.assign(ident.Name, types[i])
				}
				env.bindValue(ident.Name, values[i], types[i])
				continue
			}
			assignText, ok := emitFormalAssignTargetValue(lhs, values[i], types[i], env)
			if !ok {
				buf.WriteString(emitFormalLinef(s, env, "    go.todo %q", "assign_target"))
				continue
			}
			buf.WriteString(assignText)
		}
		return buf.String()
	}

	hint := formalOpaqueType("value")
	for _, lhs := range s.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		hint = env.typeOf(ident.Name)
		if s.Tok == token.DEFINE && hint == formalOpaqueType("value") {
			hint = inferFormalExprType(s.Rhs[0], env)
		}
		break
	}

	value, ty, prelude := emitFormalExpr(s.Rhs[0], hint, env)
	buf.WriteString(prelude)

	for i, lhs := range s.Lhs {
		ident, ok := lhs.(*ast.Ident)
		if !ok || ident.Name == "_" {
			continue
		}
		assignedValue := value
		assignedTy := ty
		if i > 0 {
			assignedTy = syntheticFormalAssignType(ident.Name, env)
			zeroValue, zeroPrelude := emitFormalZeroValue(assignedTy, env)
			buf.WriteString(zeroPrelude)
			assignedValue = zeroValue
		}
		switch s.Tok {
		case token.DEFINE:
			env.defineOrAssign(ident.Name, assignedTy)
		case token.ASSIGN:
			env.assign(ident.Name, assignedTy)
		default:
			env.assign(ident.Name, assignedTy)
		}
		env.bindValue(ident.Name, assignedValue, assignedTy)
	}
	return buf.String()
}

func emitFormalExpandedAssignExpr(expr ast.Expr, env *formalEnv) ([]string, []string, string, bool) {
	call, ok := expr.(*ast.CallExpr)
	if !ok {
		return nil, nil, "", false
	}
	return emitFormalExpandedCallExpr(call, env)
}

func emitFormalExpandedCallExpr(call *ast.CallExpr, env *formalEnv) ([]string, []string, string, bool) {
	if isMakeBuiltin(call) {
		return nil, nil, "", false
	}
	if _, _, _, ok := emitFormalBuiltinCall(call, "", env); ok {
		return nil, nil, "", false
	}
	if isFormalTypeConversionCall(call, env.module) {
		return nil, nil, "", false
	}
	if values, types, prelude, ok := emitFormalImmediateCapturedFuncLitCall(call, env); ok && len(types) > 1 {
		return values, types, prelude, true
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok || len(sig.results) <= 1 {
		return nil, nil, "", false
	}

	argHints := []string(nil)
	if len(sig.params) == len(call.Args) {
		argHints = sig.params
	}

	selector, isMethod := call.Fun.(*ast.SelectorExpr)
	if isMethod && !isFormalPackageSelector(selector, env) {
		recv, recvTy, recvPrelude := emitFormalExpr(selector.X, "", env)
		args, argTys, argPrelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
		symbol := formalMethodSymbol(selector, append([]string{recvTy}, argTys...), sig.results, env.module)
		base := env.temp("call")
		var buf strings.Builder
		buf.WriteString(recvPrelude)
		buf.WriteString(argPrelude)
		buf.WriteString(formatFormalMultiResultCall(formalMultiResultCallSpec{
			base:      base,
			callee:    symbol,
			args:      append([]string{recv}, args...),
			argTys:    append([]string{recvTy}, argTys...),
			resultTys: sig.results,
		}))
		return formalCallMultiResultRefs(base, sig.results), append([]string(nil), sig.results...), buf.String(), true
	}

	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, sig.results, env); ok {
		base := env.temp("call")
		return formalCallMultiResultRefs(base, sig.results), append([]string(nil), sig.results...), prelude + formatFormalMultiResultCall(formalMultiResultCallSpec{
			base:      base,
			callee:    symbol,
			args:      args,
			argTys:    argTys,
			resultTys: sig.results,
		}), true
	}

	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	base := env.temp("call")
	var buf strings.Builder
	buf.WriteString(prelude)
	buf.WriteString(calleePrelude)
	buf.WriteString(formatFormalMultiResultCall(formalMultiResultCallSpec{
		base:      base,
		callee:    calleeValue,
		args:      args,
		argTys:    argTys,
		resultTys: sig.results,
		indirect:  true,
	}))
	return formalCallMultiResultRefs(base, sig.results), append([]string(nil), sig.results...), buf.String(), true
}

func formatFormalMultiResultCall(spec formalMultiResultCallSpec) string {
	op := "func.call"
	calleeText := "@" + spec.callee
	if spec.indirect {
		op = "func.call_indirect"
		calleeText = spec.callee
	}
	return fmt.Sprintf(
		"    %s:%d = %s %s(%s) : (%s) -> (%s)\n",
		spec.base,
		len(spec.resultTys),
		op,
		calleeText,
		strings.Join(spec.args, ", "),
		strings.Join(spec.argTys, ", "),
		strings.Join(spec.resultTys, ", "),
	)
}

func formalCallMultiResultRefs(base string, resultTys []string) []string {
	values := make([]string, len(resultTys))
	for i := range resultTys {
		values[i] = fmt.Sprintf("%s#%d", base, i)
	}
	return values
}

func formalAssignTargetType(lhs ast.Expr, env *formalEnv) string {
	lhs = formalUnparenExpr(lhs)
	switch target := lhs.(type) {
	case *ast.Ident:
		if target.Name == "_" {
			return formalOpaqueType("value")
		}
		if env != nil && env.module != nil {
			if typedTy, ok := formalTypedExprType(target, env.module); ok && isFormalTypedInfoUsableType(typedTy) {
				return typedTy
			}
		}
		return env.typeOf(target.Name)
	case *ast.IndexExpr:
		if env != nil && env.module != nil {
			if typedTy, ok := formalTypedExprType(target, env.module); ok && isFormalTypedInfoUsableType(typedTy) {
				return typedTy
			}
		}
		return formalIndexResultType(inferFormalExprType(target.X, env))
	case *ast.StarExpr:
		return formalDerefType(inferFormalExprType(target.X, env))
	default:
		return inferFormalExprType(lhs, env)
	}
}

func emitFormalAssignTargetValueRebind(lhs ast.Expr, value string, valueTy string, env *formalEnv) (string, bool) {
	if env == nil {
		return "", false
	}
	rebindEnv := env.clone()
	text, ok := emitFormalAssignTargetValue(lhs, value, valueTy, rebindEnv)
	syncFormalTempID(env, rebindEnv)
	if !ok {
		return "", false
	}
	env.locals = rebindEnv.locals
	return text, true
}

// emitFormalAssignTargetValue updates non-identifier lvalues and rebinds the root when needed.
func emitFormalAssignTargetValue(lhs ast.Expr, value string, valueTy string, env *formalEnv) (string, bool) {
	lhs = formalUnparenExpr(lhs)
	switch target := lhs.(type) {
	case *ast.Ident:
		if target.Name == "_" {
			return "", true
		}
		env.assign(target.Name, valueTy)
		env.bindValue(target.Name, value, valueTy)
		return "", true
	case *ast.SelectorExpr:
		if isFormalPackageSelector(target, env) {
			return "", false
		}
		base, baseTy, basePrelude := emitFormalExpr(target.X, "", env)
		fieldTy := formalAssignTargetType(lhs, env)
		fieldOffset, hasFieldOffset := formalSelectorFieldOffset(target, env.module)
		fieldAddr, fieldAddrTy, fieldAddrPrelude, ok := emitFormalFieldAddr(formalFieldAddrSpec{
			base:      base,
			baseTy:    baseTy,
			field:     target.Sel.Name,
			fieldTy:   fieldTy,
			offset:    fieldOffset,
			hasOffset: hasFieldOffset,
		}, env)
		if ok {
			storePrelude, storeOK := emitFormalStore(value, valueTy, fieldAddr, fieldAddrTy, env)
			if storeOK {
				return basePrelude + fieldAddrPrelude + storePrelude, true
			}
		}
		updatedBase, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeStoreSelectorSymbol(target.Sel.Name).String(),
				args:       []string{base, value},
				argTys:     []string{baseTy, valueTy},
				resultTy:   baseTy,
				tempPrefix: "store",
			},
			env,
		)
		rebindPrelude, ok := emitFormalAssignTargetValueRebind(target.X, updatedBase, baseTy, env)
		if !ok {
			return "", false
		}
		return basePrelude + helperPrelude + rebindPrelude, true
	case *ast.IndexExpr:
		source, sourceTy, sourcePrelude := emitFormalExpr(target.X, "", env)
		indexHint := formalIndexOperandHint(sourceTy)
		index, indexTy, indexPrelude := emitFormalExpr(target.Index, indexHint, env)
		elementTy := formalIndexResultType(sourceTy)
		if isFormalSliceType(sourceTy) && normalizeFormalType(valueTy) == normalizeFormalType(elementTy) {
			elemAddr, elemAddrTy, elemAddrPrelude, ok := emitFormalElemAddr(formalElemAddrSpec{
				base:    source,
				baseTy:  sourceTy,
				index:   index,
				indexTy: indexTy,
				elemTy:  elementTy,
			}, env)
			if ok {
				storePrelude, storeOK := emitFormalStore(value, valueTy, elemAddr, elemAddrTy, env)
				if storeOK {
					return sourcePrelude + indexPrelude + elemAddrPrelude + storePrelude, true
				}
			}
		}
		updatedSource, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeStoreIndexSymbol(sourceTy).String(),
				args:       []string{source, index, value},
				argTys:     []string{sourceTy, indexTy, valueTy},
				resultTy:   sourceTy,
				tempPrefix: "store",
			},
			env,
		)
		rebindPrelude, ok := emitFormalAssignTargetValueRebind(target.X, updatedSource, sourceTy, env)
		if !ok {
			return "", false
		}
		return sourcePrelude + indexPrelude + helperPrelude + rebindPrelude, true
	case *ast.StarExpr:
		ptr, ptrTy, ptrPrelude := emitFormalExpr(target.X, "", env)
		storePrelude, ok := emitFormalStore(value, valueTy, ptr, ptrTy, env)
		if ok {
			return ptrPrelude + storePrelude, true
		}
		updatedPtr, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeStoreDerefSymbol(ptrTy).String(),
				args:       []string{ptr, value},
				argTys:     []string{ptrTy, valueTy},
				resultTy:   ptrTy,
				tempPrefix: "store",
			},
			env,
		)
		rebindPrelude, ok := emitFormalAssignTargetValueRebind(target.X, updatedPtr, ptrTy, env)
		if !ok {
			return "", false
		}
		return ptrPrelude + helperPrelude + rebindPrelude, true
	default:
		return "", false
	}
}

func formalDerefType(ptrTy string) string {
	ptrTy = normalizeFormalType(ptrTy)
	if strings.HasPrefix(ptrTy, "!go.ptr<") && strings.HasSuffix(ptrTy, ">") {
		return strings.TrimSuffix(strings.TrimPrefix(ptrTy, "!go.ptr<"), ">")
	}
	return formalOpaqueType("value")
}

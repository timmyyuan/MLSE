// This file contains the core Go AST -> formal MLIR dispatcher.
// See docs/go-frontend-lowering.md#core-dispatch.
package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

type formalGoCompareSpec struct {
	op         token.Token
	lhs        string
	rhs        string
	operandTy  string
	lhsPrelude string
	rhsPrelude string
}

// emitFormalFunc lowers one parsed Go function or method into module text.
func emitFormalFunc(fn *ast.FuncDecl, module *formalModuleContext) string {
	return emitFormalFuncBody(formalFuncBodySpec{
		name:   formalFuncSymbol(fn, module),
		recv:   fn.Recv,
		fnType: fn.Type,
		body:   fn.Body,
	}, module)
}

func emitFormalFuncBody(spec formalFuncBodySpec, module *formalModuleContext) string {
	env := newFormalEnv(module)
	env.currentFunc = sanitizeName(spec.name)
	params := emitFormalParams(formalJoinFieldLists(spec.recv, spec.fnType.Params), env)
	results := emitFormalResultTypes(spec.fnType.Results, module)
	env.resultTypes = append([]string(nil), results...)

	var buf strings.Builder
	if spec.private {
		buf.WriteString(formatPrivateFuncHeader(spec.name, params, results))
	} else {
		buf.WriteString(formatFuncHeader(spec.name, params, results))
	}

	terminated := false
	if spec.body == nil {
		buf.WriteString("    go.todo \"missing_body\"\n")
	} else {
		bodyText, term := emitFormalFuncBlock(spec.body.List, env, results)
		buf.WriteString(bodyText)
		terminated = term
	}
	if !terminated {
		if len(results) > 0 {
			buf.WriteString("    go.todo \"implicit_return_placeholder\"\n")
		}
		buf.WriteString(emitFormalFallbackReturn(results, env))
	}
	buf.WriteString("  }\n")
	return buf.String()
}

func emitFormalParams(fields *ast.FieldList, env *formalEnv) []string {
	return emitBoundParams(
		fields,
		func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, env.module) },
		func() string { return env.temp("arg") },
		func(name string, ty string) string { return env.define(name, ty) },
	)
}

func formalJoinFieldLists(lists ...*ast.FieldList) *ast.FieldList {
	var combined []*ast.Field
	for _, list := range lists {
		if list == nil {
			continue
		}
		combined = append(combined, list.List...)
	}
	if len(combined) == 0 {
		return nil
	}
	return &ast.FieldList{List: combined}
}

func emitFormalResultTypes(fields *ast.FieldList, module *formalModuleContext) []string {
	return emitFieldTypes(fields, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) })
}

// emitFormalStmt dispatches statement nodes to the file-specific lowerers.
func emitFormalStmt(stmt ast.Stmt, env *formalEnv, resultTypes []string) (string, bool) {
	if resultTypes == nil && env != nil {
		resultTypes = env.resultTypes
	}
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return emitFormalAssignStmt(s, env), false
	case *ast.ReturnStmt:
		return emitFormalReturnStmt(s, env, resultTypes), true
	case *ast.ExprStmt:
		return emitFormalExprStmt(s, env), false
	case *ast.DeclStmt:
		return emitFormalDeclStmt(s, env), false
	case *ast.IfStmt:
		return emitFormalIfStmt(s, env), false
	case *ast.ForStmt:
		return emitFormalForStmt(s, env), false
	case *ast.RangeStmt:
		return emitFormalRangeStmt(s, env), false
	case *ast.IncDecStmt:
		return emitFormalIncDecStmt(s, env), false
	case *ast.EmptyStmt:
		return "", false
	default:
		return fmt.Sprintf("    go.todo %q\n", shortNodeName(stmt)), false
	}
}

func emitFormalReturnStmt(s *ast.ReturnStmt, env *formalEnv, resultTypes []string) string {
	if len(s.Results) == 0 {
		return emitFormalReturnValues(resultTypes, env)
	}
	if len(resultTypes) == 0 {
		return "    go.todo \"unexpected_return_value\"\n    return\n"
	}
	if len(s.Results) != len(resultTypes) {
		return "    go.todo \"return_arity_mismatch\"\n" + emitFormalReturnValues(resultTypes, env)
	}
	if len(resultTypes) > 1 && len(s.Results) == 1 {
		return "    go.todo \"multi_result_return_value\"\n" + emitFormalReturnValues(resultTypes, env)
	}

	var (
		values []string
		types  []string
		buf    strings.Builder
	)
	for i, result := range s.Results {
		hint := ""
		if i < len(resultTypes) {
			hint = resultTypes[i]
		}
		value, ty, prelude := emitFormalExpr(result, hint, env)
		buf.WriteString(prelude)
		if hint != "" && normalizeFormalType(ty) != normalizeFormalType(hint) {
			if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, ty, hint, env); ok {
				buf.WriteString(coercedPrelude)
				value = coercedValue
				ty = coercedTy
			} else {
				todoValue, todoTy, todoPrelude := emitFormalTodoValue("return_type_mismatch", hint, env)
				buf.WriteString(todoPrelude)
				value = todoValue
				ty = todoTy
			}
		}
		values = append(values, value)
		types = append(types, ty)
	}
	buf.WriteString(emitFormalReturnLine(values, types))
	return buf.String()
}

func emitFormalExprStmt(s *ast.ExprStmt, env *formalEnv) string {
	if call, ok := s.X.(*ast.CallExpr); ok {
		text, ok := emitFormalCallStmt(call, env)
		if ok {
			return text
		}
	}
	_, _, prelude := emitFormalExpr(s.X, "", env)
	if prelude == "" {
		return fmt.Sprintf("    go.todo %q\n", "expr_stmt")
	}
	return prelude + fmt.Sprintf("    go.todo %q\n", "expr_stmt")
}

func emitFormalDeclStmt(s *ast.DeclStmt, env *formalEnv) string {
	gen, ok := s.Decl.(*ast.GenDecl)
	if !ok {
		return fmt.Sprintf("    go.todo %q\n", shortNodeName(s))
	}

	var buf strings.Builder
	for _, spec := range gen.Specs {
		valueSpec, ok := spec.(*ast.ValueSpec)
		if !ok {
			buf.WriteString(fmt.Sprintf("    go.todo %q\n", shortNodeName(spec)))
			continue
		}
		for i, name := range valueSpec.Names {
			if name.Name == "_" {
				continue
			}
			ty := formalOpaqueType("value")
			if valueSpec.Type != nil {
				ty = formalTypeExprToMLIR(valueSpec.Type, env.module)
			}
			if i < len(valueSpec.Values) {
				if valueSpec.Type == nil {
					ty = inferFormalExprType(valueSpec.Values[i], env)
				}
				value, valueTy, prelude := emitFormalExpr(valueSpec.Values[i], ty, env)
				buf.WriteString(prelude)
				env.bindValue(name.Name, value, valueTy)
				continue
			}
			value, prelude := emitFormalZeroValue(ty, env)
			buf.WriteString(prelude)
			env.bindValue(name.Name, value, ty)
		}
	}
	return buf.String()
}

func emitFormalIfStmt(s *ast.IfStmt, env *formalEnv) string {
	if s.Init != nil {
		return emitFormalIfStmtWithInit(s, env)
	}

	cond, prelude, ok := emitFormalCondition(s.Cond, env)
	if !ok {
		return prelude + "    go.todo \"IfStmt_condition\"\n"
	}

	thenEnv := env.clone()
	thenText, thenTerm := emitFormalRegionBlock(s.Body.List, thenEnv)
	if thenTerm {
		syncFormalTempID(env, thenEnv)
		return prelude + "    go.todo \"IfStmt_returning_region\"\n"
	}

	elseEnv := env.clone()
	elseText := ""
	hasElse := false
	if s.Else != nil {
		elseBlock, ok := s.Else.(*ast.BlockStmt)
		if !ok {
			return prelude + "    go.todo \"IfStmt_else\"\n"
		}
		hasElse = true
		var elseTerm bool
		elseText, elseTerm = emitFormalRegionBlock(elseBlock.List, elseEnv)
		if elseTerm {
			syncFormalTempID(env, thenEnv, elseEnv)
			return prelude + "    go.todo \"IfStmt_returning_region\"\n"
		}
	}

	mutated := formalMutatedOuterNames(env, thenEnv, elseEnv, hasElse)
	var buf strings.Builder
	buf.WriteString(prelude)
	switch len(mutated) {
	case 0:
		buf.WriteString(fmt.Sprintf("    scf.if %s {\n", cond))
		buf.WriteString(indentBlock(thenText, 2))
		if hasElse {
			buf.WriteString("    } else {\n")
			buf.WriteString(indentBlock(elseText, 2))
		}
		buf.WriteString("    }\n")
	case 1:
		name := mutated[0]
		thenValue := thenEnv.use(name)
		elseValue := env.use(name)
		ty := thenEnv.typeOf(name)
		if hasElse {
			elseValue = elseEnv.use(name)
			if ty == formalOpaqueType("value") {
				ty = elseEnv.typeOf(name)
			}
		}
		if ty == formalOpaqueType("value") {
			ty = env.typeOf(name)
		}
		if hasElse && normalizeFormalType(elseEnv.typeOf(name)) != normalizeFormalType(ty) {
			syncFormalTempID(env, thenEnv, elseEnv)
			return prelude + "    go.todo \"IfStmt_type_mismatch\"\n"
		}
		if !hasElse {
			elseText = ""
		}
		result := env.temp("if")
		buf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", result, cond, ty))
		buf.WriteString(indentBlock(thenText, 2))
		buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", thenValue, ty))
		buf.WriteString("    } else {\n")
		buf.WriteString(indentBlock(elseText, 2))
		buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", elseValue, ty))
		buf.WriteString("    }\n")
		env.bindValue(name, result, ty)
	default:
		resultTypes := make([]string, 0, len(mutated))
		thenValues := make([]string, 0, len(mutated))
		elseValues := make([]string, 0, len(mutated))
		for _, name := range mutated {
			ty := thenEnv.typeOf(name)
			if hasElse {
				if ty == formalOpaqueType("value") {
					ty = elseEnv.typeOf(name)
				}
				if normalizeFormalType(elseEnv.typeOf(name)) != normalizeFormalType(ty) {
					syncFormalTempID(env, thenEnv, elseEnv)
					return prelude + "    go.todo \"IfStmt_type_mismatch\"\n"
				}
			}
			if ty == formalOpaqueType("value") {
				ty = env.typeOf(name)
			}
			resultTypes = append(resultTypes, ty)
			thenValues = append(thenValues, thenEnv.use(name))
			if hasElse {
				elseValues = append(elseValues, elseEnv.use(name))
			} else {
				elseValues = append(elseValues, env.use(name))
			}
		}
		result := env.temp("if")
		buf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
		buf.WriteString(indentBlock(thenText, 2))
		buf.WriteString(emitFormalYieldLine(thenValues, resultTypes))
		buf.WriteString("    } else {\n")
		buf.WriteString(indentBlock(elseText, 2))
		buf.WriteString(emitFormalYieldLine(elseValues, resultTypes))
		buf.WriteString("    }\n")
		resultValues := formalMultiResultRefs(result, len(resultTypes))
		for i, name := range mutated {
			env.bindValue(name, resultValues[i], resultTypes[i])
		}
	}
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String()
}

func emitFormalForStmt(s *ast.ForStmt, env *formalEnv) string {
	var buf strings.Builder
	if s.Init != nil {
		initText, term := emitFormalStmt(s.Init, env, nil)
		buf.WriteString(initText)
		if term {
			return buf.String()
		}
	}

	if s.Cond == nil {
		buf.WriteString("    go.todo \"ForStmt\"\n")
		return buf.String()
	}

	ivName, upperExpr, ok := matchFormalCountedLoopCond(s.Cond)
	if !ok || len(s.Body.List) == 0 {
		buf.WriteString("    go.todo \"ForStmt\"\n")
		return buf.String()
	}

	bodyStmts := s.Body.List
	if s.Post != nil {
		if !isFormalLoopIncrement(s.Post, ivName) {
			buf.WriteString("    go.todo \"ForStmt\"\n")
			return buf.String()
		}
	} else {
		last := bodyStmts[len(bodyStmts)-1]
		if !isFormalLoopIncrement(last, ivName) {
			buf.WriteString("    go.todo \"ForStmt\"\n")
			return buf.String()
		}
		bodyStmts = bodyStmts[:len(bodyStmts)-1]
	}

	ivInit := env.use(ivName)
	ivTy := env.typeOf(ivName)
	if !isFormalIntegerType(ivTy) {
		buf.WriteString("    go.todo \"ForStmt_iv_type\"\n")
		return buf.String()
	}

	upper, upperTy, upperPrelude := emitFormalExpr(upperExpr, ivTy, env)
	if upperTy != ivTy {
		buf.WriteString(upperPrelude)
		buf.WriteString("    go.todo \"ForStmt_bound_type\"\n")
		return buf.String()
	}

	carried := collectAssignedOuterNames(bodyStmts, env, ivName)
	if len(carried) > 1 {
		buf.WriteString("    go.todo \"ForStmt_multi_iter_args\"\n")
		return buf.String()
	}

	lowerValue := ivInit
	upperValue := upper
	loopBoundTy := ivTy
	if ivTy != "index" {
		lowerIndex := env.temp("idx")
		upperIndex := env.temp("idx")
		upperPrelude += fmt.Sprintf("    %s = arith.index_cast %s : %s to index\n", lowerIndex, ivInit, ivTy)
		upperPrelude += fmt.Sprintf("    %s = arith.index_cast %s : %s to index\n", upperIndex, upper, ivTy)
		lowerValue = lowerIndex
		upperValue = upperIndex
		loopBoundTy = "index"
	}
	buf.WriteString(upperPrelude)

	step := env.temp("const")
	ivSSA := env.temp(sanitizeName(ivName) + "_iv")
	buf.WriteString(fmt.Sprintf("    %s = arith.constant 1 : %s\n", step, loopBoundTy))

	if len(carried) == 0 {
		bodyEnv := env.clone()
		bodyPrelude := ""
		if ivTy == "index" {
			bodyEnv.bindValue(ivName, ivSSA, ivTy)
		} else {
			ivCast := bodyEnv.temp(sanitizeName(ivName) + "_body")
			bodyPrelude = fmt.Sprintf("    %s = arith.index_cast %s : index to %s\n", ivCast, ivSSA, ivTy)
			bodyEnv.bindValue(ivName, ivCast, ivTy)
		}
		bodyText, _, bodyTerm := emitFormalLoopBody(bodyStmts, bodyEnv, "", "")
		syncFormalTempID(env, bodyEnv)
		if bodyTerm {
			return buf.String() + "    go.todo \"ForStmt_returning_body\"\n"
		}
		buf.WriteString(fmt.Sprintf("    scf.for %s = %s to %s step %s {\n", ivSSA, lowerValue, upperValue, step))
		buf.WriteString(indentBlock(bodyPrelude, 2))
		buf.WriteString(indentBlock(bodyText, 2))
		buf.WriteString("    }\n")
		exitIV, _, exitPrelude := emitFormalTodoValue("loop_iv_exit", ivTy, env)
		buf.WriteString(exitPrelude)
		env.bindValue(ivName, exitIV, ivTy)
		return buf.String()
	}

	carriedName := carried[0]
	carriedTy := env.typeOf(carriedName)
	iterSSA := fmt.Sprintf("%%%s_iter", sanitizeName(carriedName))
	result := env.temp("loop")
	bodyEnv := env.clone()
	bodyPrelude := ""
	if ivTy == "index" {
		bodyEnv.bindValue(ivName, ivSSA, ivTy)
	} else {
		ivCast := bodyEnv.temp(sanitizeName(ivName) + "_body")
		bodyPrelude = fmt.Sprintf("    %s = arith.index_cast %s : index to %s\n", ivCast, ivSSA, ivTy)
		bodyEnv.bindValue(ivName, ivCast, ivTy)
	}
	bodyEnv.bindValue(carriedName, iterSSA, carriedTy)
	bodyText, yieldValue, bodyTerm := emitFormalLoopBody(bodyStmts, bodyEnv, carriedName, carriedTy)
	syncFormalTempID(env, bodyEnv)
	if bodyTerm {
		return buf.String() + "    go.todo \"ForStmt_returning_body\"\n"
	}
	buf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s = %s) -> (%s) {\n", result, ivSSA, lowerValue, upperValue, step, iterSSA, env.use(carriedName), carriedTy))
	buf.WriteString(indentBlock(bodyPrelude, 2))
	buf.WriteString(indentBlock(bodyText, 2))
	buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", yieldValue, carriedTy))
	buf.WriteString("    }\n")
	env.bindValue(carriedName, result, carriedTy)
	exitIV, _, exitPrelude := emitFormalTodoValue("loop_iv_exit", ivTy, env)
	buf.WriteString(exitPrelude)
	env.bindValue(ivName, exitIV, ivTy)
	return buf.String()
}

func emitFormalRangeStmt(s *ast.RangeStmt, env *formalEnv) string {
	if s.Tok != token.DEFINE && s.Tok != token.ASSIGN {
		return "    go.todo \"RangeStmt\"\n"
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
	var buf strings.Builder
	buf.WriteString(sourcePrelude)
	buf.WriteString(lengthPrelude)
	buf.WriteString(fmt.Sprintf("    %s = arith.constant 0 : index\n", lower))
	buf.WriteString(fmt.Sprintf("    %s = arith.index_cast %s : i32 to index\n", upper, lengthTmp))
	buf.WriteString(fmt.Sprintf("    %s = arith.constant 1 : index\n", step))

	keyName := rangeKeyName(s.Key)
	valueName := rangeKeyName(s.Value)
	excludes := make(map[string]struct{})
	for _, name := range []string{keyName, valueName} {
		if name != "" {
			excludes[name] = struct{}{}
		}
	}
	carried := collectAssignedOuterNamesWithExcludes(s.Body.List, env, excludes)
	if len(carried) > 1 {
		return buf.String() + "    go.todo \"RangeStmt_multi_iter_args\"\n"
	}

	if len(carried) == 0 {
		bodyEnv := env.clone()
		bodyPrelude := emitFormalRangeBindings(s, source, sourceTy, ivSSA, bodyEnv)
		bodyText, _, bodyTerm := emitFormalLoopBody(s.Body.List, bodyEnv, "", "")
		syncFormalTempID(env, bodyEnv)
		if bodyTerm {
			return buf.String() + "    go.todo \"RangeStmt_returning_body\"\n"
		}
		buf.WriteString(fmt.Sprintf("    scf.for %s = %s to %s step %s {\n", ivSSA, lower, upper, step))
		buf.WriteString(indentBlock(bodyPrelude, 2))
		buf.WriteString(indentBlock(bodyText, 2))
		buf.WriteString("    }\n")
		return buf.String()
	}

	carriedName := carried[0]
	carriedTy := env.typeOf(carriedName)
	iterSSA := fmt.Sprintf("%%%s_iter", sanitizeName(carriedName))
	result := env.temp("range")
	bodyEnv := env.clone()
	bodyEnv.bindValue(carriedName, iterSSA, carriedTy)
	bodyPrelude := emitFormalRangeBindings(s, source, sourceTy, ivSSA, bodyEnv)
	bodyText, yieldValue, bodyTerm := emitFormalLoopBody(s.Body.List, bodyEnv, carriedName, carriedTy)
	syncFormalTempID(env, bodyEnv)
	if bodyTerm {
		return buf.String() + "    go.todo \"RangeStmt_returning_body\"\n"
	}
	buf.WriteString(fmt.Sprintf("    %s = scf.for %s = %s to %s step %s iter_args(%s = %s) -> (%s) {\n", result, ivSSA, lower, upper, step, iterSSA, env.use(carriedName), carriedTy))
	buf.WriteString(indentBlock(bodyPrelude, 2))
	buf.WriteString(indentBlock(bodyText, 2))
	buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", yieldValue, carriedTy))
	buf.WriteString("    }\n")
	env.bindValue(carriedName, result, carriedTy)
	return buf.String()
}

func emitFormalRangeBindings(s *ast.RangeStmt, source string, sourceTy string, ivSSA string, env *formalEnv) string {
	var buf strings.Builder
	indexValue := ivSSA
	indexTy := "index"
	if keyName := rangeKeyName(s.Key); keyName != "" {
		keyTy := "i32"
		if s.Tok == token.ASSIGN {
			keyTy = chooseFormalCommonType(env.typeOf(keyName), "i32")
		}
		boundIndex := indexValue
		if keyTy != "index" {
			cast := env.temp("range_idx")
			buf.WriteString(fmt.Sprintf("    %s = arith.index_cast %s : index to %s\n", cast, ivSSA, keyTy))
			boundIndex = cast
			indexTy = keyTy
		}
		if s.Tok == token.DEFINE {
			env.defineOrAssign(keyName, keyTy)
		} else {
			env.assign(keyName, keyTy)
		}
		env.bindValue(keyName, boundIndex, keyTy)
	}
	if valueName := rangeKeyName(s.Value); valueName != "" {
		valueTy := inferFormalRangeValueType(valueName, sourceTy, s.Body, env)
		indexArg := indexValue
		if indexTy != "index" {
			indexArg = ivSSA
		}
		valueTmp, _, valuePrelude, ok := emitFormalIndexedReadValue(formalGoIndexSpec{
			source:     source,
			sourceTy:   sourceTy,
			index:      indexArg,
			indexTy:    "index",
			hintedTy:   valueTy,
			tempPrefix: "rangeval",
		}, env)
		if !ok {
			valueTmp, valuePrelude = emitFormalHelperCall(
				formalHelperCallSpec{
					base:       "__mlse_index_" + sanitizeName(sourceTy),
					args:       []string{source, indexArg},
					argTys:     []string{sourceTy, "index"},
					resultTy:   valueTy,
					tempPrefix: "rangeval",
				},
				env,
			)
		}
		buf.WriteString(valuePrelude)
		if s.Tok == token.DEFINE {
			env.defineOrAssign(valueName, valueTy)
		} else {
			env.assign(valueName, valueTy)
		}
		env.bindValue(valueName, valueTmp, valueTy)
	}
	return buf.String()
}

func rangeKeyName(expr ast.Expr) string {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return ""
	}
	return ident.Name
}

func emitFormalIncDecStmt(s *ast.IncDecStmt, env *formalEnv) string {
	ident, ok := s.X.(*ast.Ident)
	if !ok {
		return fmt.Sprintf("    go.todo %q\n", shortNodeName(s))
	}
	name := env.use(ident.Name)
	ty := env.typeOf(ident.Name)
	if !isFormalIntegerType(ty) {
		return fmt.Sprintf("    go.todo %q\n", "incdec_non_integer")
	}
	step := env.temp("const")
	next := env.temp("inc")
	env.assign(ident.Name, ty)
	op := "arith.addi"
	if s.Tok == token.DEC {
		op = "arith.subi"
	}
	env.bindValue(ident.Name, next, ty)
	return fmt.Sprintf("    %s = arith.constant 1 : %s\n    %s = %s %s, %s : %s\n", step, ty, next, op, name, step, ty)
}

func emitFormalHelperCall(spec formalHelperCallSpec, env *formalEnv) (string, string) {
	resultTy := normalizeFormalType(spec.resultTy)
	symbol := env.module.registerExtern(spec.base, spec.argTys, []string{resultTy})
	tmp := env.temp(spec.tempPrefix)
	return tmp, fmt.Sprintf("    %s = func.call @%s(%s) : (%s) -> %s\n", tmp, symbol, strings.Join(spec.args, ", "), strings.Join(spec.argTys, ", "), resultTy)
}

func isFormalOpaquePlaceholderType(ty string) bool {
	ty = normalizeFormalType(ty)
	return ty == formalOpaqueType("value") || ty == formalOpaqueType("result")
}

func chooseFormalCommonType(lhsTy string, rhsTy string) string {
	lhsTy = normalizeFormalType(lhsTy)
	rhsTy = normalizeFormalType(rhsTy)
	switch {
	case isFormalOpaquePlaceholderType(lhsTy) && !isFormalOpaquePlaceholderType(rhsTy):
		return rhsTy
	case isFormalOpaquePlaceholderType(rhsTy) && !isFormalOpaquePlaceholderType(lhsTy):
		return lhsTy
	case lhsTy == rhsTy:
		return lhsTy
	case lhsTy == formalOpaqueType("value"):
		return rhsTy
	case rhsTy == formalOpaqueType("value"):
		return lhsTy
	default:
		return lhsTy
	}
}

func formalIndexResultType(containerTy string) string {
	containerTy = normalizeFormalType(containerTy)
	if strings.HasPrefix(containerTy, "!go.slice<") && strings.HasSuffix(containerTy, ">") {
		return strings.TrimSuffix(strings.TrimPrefix(containerTy, "!go.slice<"), ">")
	}
	if containerTy == "!go.string" {
		return "i8"
	}
	return formalOpaqueType("value")
}

func formalIndexOperandHint(containerTy string) string {
	if isFormalIndexLikeType(containerTy) {
		return "i32"
	}
	return ""
}

func formalFuncSigFromType(fnType *ast.FuncType, module *formalModuleContext) formalFuncSig {
	if fnType == nil {
		return formalFuncSig{}
	}
	return formalFuncSig{
		params:  emitFieldTypes(fnType.Params, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
		results: emitFieldTypes(fnType.Results, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
	}
}

func formalFuncSigFromDecl(fn *ast.FuncDecl, module *formalModuleContext) formalFuncSig {
	if fn == nil {
		return formalFuncSig{}
	}
	return formalFuncSig{
		params:  emitFieldTypes(formalJoinFieldLists(fn.Recv, fn.Type.Params), func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
		results: emitFieldTypes(fn.Type.Results, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) }),
	}
}

func emitFormalFuncLitExpr(lit *ast.FuncLit, hintedTy string, env *formalEnv) (string, string, string) {
	funcTy := formalTypeExprToMLIR(lit.Type, env.module)
	if funcTy == formalOpaqueType("func") {
		funcTy = normalizeFormalType(hintedTy)
	}
	sig, ok := parseFormalFuncType(funcTy)
	if !ok {
		return emitFormalTodoValue("FuncLit_type", normalizeFormalType(hintedTy), env)
	}

	captures := formalFuncLitCaptures(lit, env)
	if len(captures) != 0 {
		return emitFormalTodoValue("FuncLit_capture", funcTy, env)
	}

	symbol := env.module.reserveFuncLitSymbol(sig, env.currentFunc)
	env.module.addGeneratedFunc(emitFormalFuncBody(formalFuncBodySpec{
		name:    symbol,
		fnType:  lit.Type,
		body:    lit.Body,
		private: true,
	}, env.module))

	tmp := env.temp("funclit")
	return tmp, funcTy, fmt.Sprintf("    %s = func.constant @%s : %s\n", tmp, symbol, funcTy)
}

func formalFuncLitCaptures(lit *ast.FuncLit, env *formalEnv) []string {
	if lit == nil || env == nil {
		return nil
	}

	localNames := collectFormalFuncLocalNames(lit.Type, lit.Body)
	seen := make(map[string]struct{})
	var stack []ast.Node
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		if n == nil {
			stack = stack[:len(stack)-1]
			return false
		}
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		parent := ast.Node(nil)
		if len(stack) != 0 {
			parent = stack[len(stack)-1]
		}
		stack = append(stack, n)

		ident, ok := n.(*ast.Ident)
		if !ok || ident.Name == "_" {
			return true
		}
		if isFormalDefinitionIdent(parent, ident) {
			return true
		}
		if _, ok := localNames[ident.Name]; ok {
			return true
		}
		if _, ok := env.locals[ident.Name]; ok {
			seen[ident.Name] = struct{}{}
		}
		return true
	})

	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func collectFormalFuncLocalNames(fnType *ast.FuncType, body *ast.BlockStmt) map[string]struct{} {
	names := make(map[string]struct{})
	collectFormalFieldNames(names, fnType.Params)
	collectFormalFieldNames(names, fnType.Results)
	if body == nil {
		return names
	}

	ast.Inspect(body, func(n ast.Node) bool {
		switch node := n.(type) {
		case nil:
			return false
		case *ast.FuncLit:
			return false
		case *ast.AssignStmt:
			if node.Tok == token.DEFINE {
				for _, lhs := range node.Lhs {
					ident, ok := lhs.(*ast.Ident)
					if ok && ident.Name != "_" {
						names[ident.Name] = struct{}{}
					}
				}
			}
		case *ast.RangeStmt:
			if node.Tok == token.DEFINE {
				if ident, ok := node.Key.(*ast.Ident); ok && ident.Name != "_" {
					names[ident.Name] = struct{}{}
				}
				if ident, ok := node.Value.(*ast.Ident); ok && ident.Name != "_" {
					names[ident.Name] = struct{}{}
				}
			}
		case *ast.DeclStmt:
			gen, ok := node.Decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.VAR {
				return true
			}
			for _, spec := range gen.Specs {
				valueSpec, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for _, name := range valueSpec.Names {
					if name.Name != "_" {
						names[name.Name] = struct{}{}
					}
				}
			}
		}
		return true
	})
	return names
}

func collectFormalFieldNames(out map[string]struct{}, fields *ast.FieldList) {
	if fields == nil {
		return
	}
	for _, field := range fields.List {
		for _, name := range field.Names {
			if name.Name != "_" {
				out[name.Name] = struct{}{}
			}
		}
	}
}

func isFormalDefinitionIdent(parent ast.Node, ident *ast.Ident) bool {
	switch node := parent.(type) {
	case *ast.AssignStmt:
		if node.Tok != token.DEFINE {
			return false
		}
		for _, lhs := range node.Lhs {
			if lhs == ident {
				return true
			}
		}
	case *ast.Field:
		for _, name := range node.Names {
			if name == ident {
				return true
			}
		}
	case *ast.ValueSpec:
		for _, name := range node.Names {
			if name == ident {
				return true
			}
		}
	case *ast.RangeStmt:
		return (node.Key == ident || node.Value == ident) && node.Tok == token.DEFINE
	case *ast.SelectorExpr:
		return node.Sel == ident
	}
	return false
}

// emitFormalExpr dispatches expression nodes while threading type hints through lowering.
func emitFormalExpr(expr ast.Expr, hintedTy string, env *formalEnv) (string, string, string) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			litTy := "i32"
			if isFormalIntegerType(hintedTy) {
				litTy = normalizeFormalType(hintedTy)
			}
			tmp := env.temp("const")
			return tmp, litTy, fmt.Sprintf("    %s = arith.constant %s : %s\n", tmp, e.Value, litTy)
		case token.STRING:
			tmp := env.temp("str")
			quoted := strconv.Quote(strings.Trim(e.Value, "\"`"))
			return tmp, "!go.string", fmt.Sprintf("    %s = go.string_constant %s : !go.string\n", tmp, quoted)
		default:
			return emitFormalTodoValue("literal_"+sanitizeName(e.Kind.String()), normalizeFormalType(hintedTy), env)
		}
	case *ast.Ident:
		switch e.Name {
		case "nil":
			ty := normalizeFormalType(hintedTy)
			if hintedTy == "" {
				ty = "!go.error"
			}
			if !isFormalNilableType(ty) {
				value, prelude := emitFormalZeroValue(ty, env)
				return value, ty, prelude
			}
			tmp := env.temp("nil")
			return tmp, ty, fmt.Sprintf("    %s = go.nil : %s\n", tmp, ty)
		case "true", "false":
			tmp := env.temp("const")
			return tmp, "i1", fmt.Sprintf("    %s = arith.constant %s\n", tmp, e.Name)
		default:
			if _, ok := env.locals[e.Name]; ok {
				return env.use(e.Name), env.typeOf(e.Name), ""
			}
			if env.module != nil {
				symbol := env.module.topLevelSymbol(e.Name)
				if sig, ok := env.module.definedFuncs[symbol]; ok {
					funcTy := formatFormalFuncType(sig.params, sig.results)
					tmp := env.temp("fn")
					return tmp, funcTy, fmt.Sprintf("    %s = func.constant @%s : %s\n", tmp, symbol, funcTy)
				}
			}
			ty := env.typeOf(e.Name)
			if env.module != nil {
				if typedTy, ok := formalTypedExprType(e, env.module); ok && isFormalTypedInfoUsableType(typedTy) {
					ty = typedTy
				}
			}
			tmp, prelude := emitFormalHelperCall(formalHelperCallSpec{
				base:       e.Name,
				resultTy:   ty,
				tempPrefix: "global",
			}, env)
			return tmp, ty, prelude
		}
	case *ast.BinaryExpr:
		return emitFormalBinaryExpr(e, hintedTy, env)
	case *ast.CallExpr:
		return emitFormalCallExpr(e, hintedTy, env)
	case *ast.CompositeLit:
		return emitFormalCompositeLitExpr(e, hintedTy, env)
	case *ast.FuncLit:
		return emitFormalFuncLitExpr(e, hintedTy, env)
	case *ast.IndexExpr:
		return emitFormalIndexExpr(e, hintedTy, env)
	case *ast.ParenExpr:
		return emitFormalExpr(e.X, hintedTy, env)
	case *ast.SelectorExpr:
		return emitFormalSelectorExpr(e, hintedTy, env)
	case *ast.StarExpr:
		return emitFormalStarExpr(e, hintedTy, env)
	case *ast.TypeAssertExpr:
		return emitFormalTypeAssertExpr(e, hintedTy, env)
	case *ast.UnaryExpr:
		return emitFormalUnaryExpr(e, hintedTy, env)
	default:
		return emitFormalTodoValue(shortNodeName(expr), normalizeFormalType(hintedTy), env)
	}
}

func emitFormalIndexExpr(expr *ast.IndexExpr, hintedTy string, env *formalEnv) (string, string, string) {
	source, sourceTy, sourcePrelude := emitFormalExpr(expr.X, "", env)
	indexHint := formalIndexOperandHint(sourceTy)
	index, indexTy, indexPrelude := emitFormalExpr(expr.Index, indexHint, env)
	if tmp, elementTy, opText, ok := emitFormalIndexedReadValue(formalGoIndexSpec{
		source:     source,
		sourceTy:   sourceTy,
		index:      index,
		indexTy:    indexTy,
		hintedTy:   hintedTy,
		tempPrefix: "index",
	}, env); ok {
		return tmp, elementTy, sourcePrelude + indexPrelude + opText
	}
	elementTy := normalizeFormalType(hintedTy)
	if isFormalOpaquePlaceholderType(elementTy) {
		elementTy = formalIndexResultType(sourceTy)
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       "__mlse_index_" + sanitizeName(sourceTy),
			args:       []string{source, index},
			argTys:     []string{sourceTy, indexTy},
			resultTy:   elementTy,
			tempPrefix: "index",
		},
		env,
	)
	return tmp, elementTy, sourcePrelude + indexPrelude + helperPrelude
}

func emitFormalSelectorExpr(expr *ast.SelectorExpr, hintedTy string, env *formalEnv) (string, string, string) {
	ty := formalSelectorResultType(expr, hintedTy, env)
	if sig, ok := parseFormalFuncType(ty); ok {
		symbol := formalCallSymbol(expr, sig.params, sig.results, env.module)
		if symbol == "" {
			return emitFormalTodoValue("SelectorExpr", ty, env)
		}
		tmp := env.temp("sel")
		return tmp, ty, fmt.Sprintf("    %s = func.constant @%s : %s\n", tmp, symbol, ty)
	}

	if isFormalPackageSelector(expr, env) {
		tmp, prelude := emitFormalHelperCall(formalHelperCallSpec{
			base:       formalPackageSelectorSymbol(expr, env.module),
			resultTy:   ty,
			tempPrefix: "sel",
		}, env)
		return tmp, ty, prelude
	}

	base, baseTy, basePrelude := emitFormalExpr(expr.X, "", env)
	fieldAddr, fieldAddrTy, fieldAddrPrelude, ok := emitFormalFieldAddr(base, baseTy, expr.Sel.Name, ty, env)
	if ok {
		tmp, loadedTy, loadPrelude, loadOK := emitFormalLoad(fieldAddr, fieldAddrTy, ty, env)
		if loadOK {
			return tmp, loadedTy, basePrelude + fieldAddrPrelude + loadPrelude
		}
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       "__mlse_selector_" + sanitizeName(expr.Sel.Name),
			args:       []string{base},
			argTys:     []string{baseTy},
			resultTy:   ty,
			tempPrefix: "sel",
		},
		env,
	)
	return tmp, ty, basePrelude + helperPrelude
}

func emitFormalBinaryExpr(expr *ast.BinaryExpr, hintedTy string, env *formalEnv) (string, string, string) {
	operandHint := formalBinaryOperandHint(expr, env)
	lhs, lhsTy, lhsPrelude := emitFormalExpr(expr.X, operandHint, env)
	rhsHint := operandHint
	if isFormalOpaquePlaceholderType(rhsHint) && !isFormalOpaquePlaceholderType(lhsTy) {
		rhsHint = lhsTy
	}
	rhs, rhsTy, rhsPrelude := emitFormalExpr(expr.Y, rhsHint, env)
	operandTy := lhsTy
	if operandTy == "" || operandTy == formalOpaqueType("value") {
		operandTy = rhsTy
	}
	resultTy := operandTy
	op := ""
	switch expr.Op {
	case token.ADD:
		op = "arith.addi"
	case token.SUB:
		op = "arith.subi"
	case token.MUL:
		op = "arith.muli"
	case token.QUO:
		op = "arith.divsi"
	case token.EQL:
		op = "arith.cmpi eq,"
		resultTy = "i1"
	case token.NEQ:
		op = "arith.cmpi ne,"
		resultTy = "i1"
	case token.GTR:
		op = "arith.cmpi sgt,"
		resultTy = "i1"
	case token.LSS:
		op = "arith.cmpi slt,"
		resultTy = "i1"
	case token.GEQ:
		op = "arith.cmpi sge,"
		resultTy = "i1"
	case token.LEQ:
		op = "arith.cmpi sle,"
		resultTy = "i1"
	case token.LAND:
		op = "arith.andi"
		resultTy = "i1"
		operandTy = "i1"
	case token.LOR:
		op = "arith.ori"
		resultTy = "i1"
		operandTy = "i1"
	default:
		return emitFormalTodoValue("binary_"+sanitizeName(expr.Op.String()), normalizeFormalType(hintedTy), env)
	}

	operandTy = chooseFormalCommonType(lhsTy, rhsTy)
	if isFormalNilExpr(expr.X) {
		operandTy = normalizeFormalType(rhsTy)
	}
	if isFormalNilExpr(expr.Y) {
		operandTy = normalizeFormalType(lhsTy)
	}
	if !isFormalIntegerType(operandTy) && operandTy != "i1" {
		if expr.Op == token.ADD && operandTy == "!go.string" {
			tmp, helperPrelude := emitFormalHelperCall(
				formalHelperCallSpec{
					base:       "__mlse_add__go.string",
					args:       []string{lhs, rhs},
					argTys:     []string{operandTy, operandTy},
					resultTy:   operandTy,
					tempPrefix: "add",
				},
				env,
			)
			return tmp, operandTy, lhsPrelude + rhsPrelude + helperPrelude
		}
		if expr.Op == token.EQL || expr.Op == token.NEQ {
			if supportsFormalGoCompareOp(expr, operandTy) {
				return emitFormalGoCompare(
					formalGoCompareSpec{
						op:         expr.Op,
						lhs:        lhs,
						rhs:        rhs,
						operandTy:  operandTy,
						lhsPrelude: lhsPrelude,
						rhsPrelude: rhsPrelude,
					},
					env,
				)
			}
			helperBase := "__mlse_eq_" + sanitizeName(operandTy)
			if expr.Op == token.NEQ {
				helperBase = "__mlse_neq_" + sanitizeName(operandTy)
			}
			tmp, helperPrelude := emitFormalHelperCall(
				formalHelperCallSpec{
					base:       helperBase,
					args:       []string{lhs, rhs},
					argTys:     []string{operandTy, operandTy},
					resultTy:   "i1",
					tempPrefix: "cmp",
				},
				env,
			)
			return tmp, "i1", lhsPrelude + rhsPrelude + helperPrelude
		}
		if coercedLHS, _, coercedPrelude, ok := emitFormalCoerceValue(lhs, lhsTy, operandTy, env); ok {
			lhs = coercedLHS
			lhsPrelude += coercedPrelude
		}
		if coercedRHS, _, coercedPrelude, ok := emitFormalCoerceValue(rhs, rhsTy, operandTy, env); ok {
			rhs = coercedRHS
			rhsPrelude += coercedPrelude
		}
		tmp, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       "__mlse_bin_" + sanitizeName(expr.Op.String()) + "__" + sanitizeName(operandTy),
				args:       []string{lhs, rhs},
				argTys:     []string{operandTy, operandTy},
				resultTy:   normalizeFormalType(resultTy),
				tempPrefix: "bin",
			},
			env,
		)
		return tmp, normalizeFormalType(resultTy), lhsPrelude + rhsPrelude + helperPrelude
	}

	if coercedLHS, _, coercedPrelude, ok := emitFormalCoerceValue(lhs, lhsTy, operandTy, env); ok {
		lhs = coercedLHS
		lhsPrelude += coercedPrelude
	}
	if coercedRHS, _, coercedPrelude, ok := emitFormalCoerceValue(rhs, rhsTy, operandTy, env); ok {
		rhs = coercedRHS
		rhsPrelude += coercedPrelude
	}

	tmp := env.temp("bin")
	var buf strings.Builder
	buf.WriteString(lhsPrelude)
	buf.WriteString(rhsPrelude)
	if strings.HasPrefix(op, "arith.cmpi ") {
		buf.WriteString(fmt.Sprintf("    %s = %s %s, %s : %s\n", tmp, op, lhs, rhs, operandTy))
	} else {
		buf.WriteString(fmt.Sprintf("    %s = %s %s, %s : %s\n", tmp, op, lhs, rhs, operandTy))
	}
	return tmp, resultTy, buf.String()
}

func emitFormalGoCompare(spec formalGoCompareSpec, env *formalEnv) (string, string, string) {
	tmp := env.temp("cmp")
	mnemonic := "go.eq"
	if spec.op == token.NEQ {
		mnemonic = "go.neq"
	}
	var buf strings.Builder
	buf.WriteString(spec.lhsPrelude)
	buf.WriteString(spec.rhsPrelude)
	buf.WriteString(fmt.Sprintf(
		"    %s = %s %s, %s : (%s, %s) -> i1\n",
		tmp, mnemonic, spec.lhs, spec.rhs, spec.operandTy, spec.operandTy,
	))
	return tmp, "i1", buf.String()
}

func supportsFormalGoCompareOp(expr *ast.BinaryExpr, operandTy string) bool {
	operandTy = normalizeFormalType(operandTy)
	switch {
	case isFormalStringType(operandTy):
		return true
	case isFormalPointerType(operandTy):
		return true
	case isFormalSliceType(operandTy):
		return isFormalNilExpr(expr.X) || isFormalNilExpr(expr.Y)
	case operandTy == "!go.error":
		return isFormalNilExpr(expr.X) || isFormalNilExpr(expr.Y)
	default:
		return false
	}
}

func formalBinaryOperandHint(expr *ast.BinaryExpr, env *formalEnv) string {
	switch expr.Op {
	case token.LAND, token.LOR:
		return "i1"
	}
	if isFormalNilExpr(expr.X) {
		return normalizeFormalType(inferFormalExprType(expr.Y, env))
	}
	if isFormalNilExpr(expr.Y) {
		return normalizeFormalType(inferFormalExprType(expr.X, env))
	}

	lhsTy := inferFormalExprType(expr.X, env)
	rhsTy := inferFormalExprType(expr.Y, env)
	hint := chooseFormalCommonType(lhsTy, rhsTy)
	if !isFormalOpaquePlaceholderType(hint) {
		return hint
	}
	if !isFormalOpaquePlaceholderType(normalizeFormalType(lhsTy)) {
		return normalizeFormalType(lhsTy)
	}
	if !isFormalOpaquePlaceholderType(normalizeFormalType(rhsTy)) {
		return normalizeFormalType(rhsTy)
	}
	return hint
}

func isFormalNilExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func emitFormalCallOperands(args []ast.Expr, env *formalEnv) ([]string, []string, string) {
	return emitFormalCallOperandsWithHints(args, nil, env)
}

func emitFormalCallOperandsWithHints(args []ast.Expr, hints []string, env *formalEnv) ([]string, []string, string) {
	var (
		values []string
		types  []string
		buf    strings.Builder
	)
	for i, arg := range args {
		hint := ""
		if i < len(hints) {
			hint = hints[i]
		}
		value, ty, prelude := emitFormalExpr(arg, hint, env)
		buf.WriteString(prelude)
		values = append(values, value)
		types = append(types, ty)
	}
	return values, types, buf.String()
}

func formalDirectCallSymbol(expr ast.Expr, argTys []string, resultTys []string, env *formalEnv) (string, bool) {
	switch callee := expr.(type) {
	case *ast.Ident:
		if env != nil {
			if _, ok := env.locals[callee.Name]; ok {
				return "", false
			}
		}
	case *ast.SelectorExpr:
		if !isFormalPackageSelector(callee, env) {
			return "", false
		}
	default:
		return "", false
	}

	symbol := formalCallSymbol(expr, argTys, resultTys, env.module)
	return symbol, symbol != ""
}

func formalExprFuncSig(expr ast.Expr, env *formalEnv) (formalFuncSig, bool) {
	if env != nil && env.module != nil {
		if sig, ok := formalTypedExprFuncSig(expr, env.module); ok {
			return sig, true
		}
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if env != nil {
			if binding, ok := env.locals[e.Name]; ok && binding.funcSig != nil {
				return *binding.funcSig, true
			}
		}
		if env != nil && env.module != nil {
			if sig, ok := env.module.definedFuncs[env.module.topLevelSymbol(e.Name)]; ok {
				return sig, true
			}
		}
	case *ast.SelectorExpr:
		if env != nil && !isFormalPackageSelector(e, env) && env.module != nil {
			if symbol := formalMethodObjectSymbol(e, env.module); symbol != "" {
				if sig, ok := env.module.definedFuncs[symbol]; ok {
					return sig, true
				}
			}
			if sig, ok := env.module.definedFuncs[env.module.methodSymbol(e.Sel.Name)]; ok {
				return sig, true
			}
		}
	case *ast.FuncLit:
		return formalFuncSigFromType(e.Type, env.module), true
	}
	return parseFormalFuncType(inferFormalExprType(expr, env))
}

func emitFormalCallExpr(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string) {
	if isMakeBuiltin(call) {
		return emitFormalMakeCall(call, env)
	}
	if value, ty, prelude, ok := emitFormalBuiltinCall(call, hintedTy, env); ok {
		return value, ty, prelude
	}
	if isFormalTypeConversionCall(call, env.module) {
		targetTy := formalTypeExprToMLIR(call.Fun, env.module)
		value, valueTy, prelude := emitFormalExpr(call.Args[0], targetTy, env)
		if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, valueTy, targetTy, env); ok {
			return coercedValue, coercedTy, prelude + coercedPrelude
		}
		todoValue, todoTy, todoPrelude := emitFormalTodoValue("type_conversion", targetTy, env)
		return todoValue, todoTy, prelude + todoPrelude
	}

	argHints := []string(nil)
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	if value, ty, prelude, ok := emitFormalMethodCallExpr(call, hintedTy, env, argHints); ok {
		return value, ty, prelude
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	var buf strings.Builder
	buf.WriteString(prelude)

	resultTy := inferFormalCallResultType(call, hintedTy, env)
	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, []string{resultTy}, env); ok {
		tmp := env.temp("call")
		buf.WriteString(fmt.Sprintf("    %s = func.call @%s(%s) : (%s) -> %s\n", tmp, symbol, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
		return tmp, resultTy, buf.String()
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok {
		return emitFormalTodoValue("indirect_call", normalizeFormalType(hintedTy), env)
	}
	if len(sig.results) > 1 {
		return emitFormalTodoValue("indirect_call_multi_result", normalizeFormalType(resultTy), env)
	}

	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	buf.WriteString(calleePrelude)
	tmp := env.temp("call")
	buf.WriteString(fmt.Sprintf("    %s = func.call_indirect %s(%s) : (%s) -> %s\n", tmp, calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
	return tmp, resultTy, buf.String()
}

func emitFormalMakeCall(call *ast.CallExpr, env *formalEnv) (string, string, string) {
	targetTy := formalTypeExprToMLIR(call.Args[0], env.module)
	if !strings.HasPrefix(targetTy, "!go.slice<") {
		return emitFormalHelperMakeCall(call.Args[1:], targetTy, env)
	}
	if len(call.Args) < 2 {
		return emitFormalTodoValue("make_missing_len", formalOpaqueType("make"), env)
	}
	length, lengthTy, lengthPrelude := emitFormalExpr(call.Args[1], "i32", env)
	capacity := length
	capacityPrelude := ""
	if len(call.Args) > 2 {
		capacity, _, capacityPrelude = emitFormalExpr(call.Args[2], lengthTy, env)
	}
	tmp := env.temp("make")
	var buf strings.Builder
	buf.WriteString(lengthPrelude)
	buf.WriteString(capacityPrelude)
	buf.WriteString(fmt.Sprintf("    %s = go.make_slice %s, %s : %s to %s\n", tmp, length, capacity, lengthTy, targetTy))
	return tmp, targetTy, buf.String()
}

func emitFormalUnaryExpr(expr *ast.UnaryExpr, hintedTy string, env *formalEnv) (string, string, string) {
	switch expr.Op {
	case token.SUB:
		value, ty, prelude := emitFormalExpr(expr.X, hintedTy, env)
		if !isFormalIntegerType(ty) {
			return emitFormalTodoValue("unary_sub", normalizeFormalType(hintedTy), env)
		}
		zero, zeroPrelude := emitFormalZeroValue(ty, env)
		tmp := env.temp("neg")
		return tmp, ty, prelude + zeroPrelude + fmt.Sprintf("    %s = arith.subi %s, %s : %s\n", tmp, zero, value, ty)
	case token.NOT:
		value, ty, prelude := emitFormalExpr(expr.X, "i1", env)
		if ty != "i1" {
			return emitFormalTodoValue("unary_not", normalizeFormalType(hintedTy), env)
		}
		one := env.temp("const")
		tmp := env.temp("not")
		return tmp, "i1", prelude + fmt.Sprintf("    %s = arith.constant true\n    %s = arith.xori %s, %s : i1\n", one, tmp, value, one)
	case token.AND:
		if composite, ok := expr.X.(*ast.CompositeLit); ok {
			resultTy := normalizeFormalType(hintedTy)
			if isFormalOpaquePlaceholderType(resultTy) {
				resultTy = "!go.ptr<" + goTypeToFormalMLIR(composite.Type) + ">"
			}
			tmp, prelude := emitFormalHelperCall(
				formalHelperCallSpec{
					base:       "__mlse_new_" + sanitizeName(resultTy),
					resultTy:   resultTy,
					tempPrefix: "new",
				},
				env,
			)
			return tmp, resultTy, prelude
		}
		value, ty, prelude := emitFormalExpr(expr.X, "", env)
		resultTy := normalizeFormalType(hintedTy)
		if isFormalOpaquePlaceholderType(resultTy) {
			resultTy = "!go.ptr<" + ty + ">"
		}
		tmp, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       "__mlse_addrof_" + sanitizeName(ty),
				args:       []string{value},
				argTys:     []string{ty},
				resultTy:   resultTy,
				tempPrefix: "addr",
			},
			env,
		)
		return tmp, resultTy, prelude + helperPrelude
	default:
		return emitFormalTodoValue("unary_"+sanitizeName(expr.Op.String()), normalizeFormalType(hintedTy), env)
	}
}

func emitFormalCallStmt(call *ast.CallExpr, env *formalEnv) (string, bool) {
	if isMakeBuiltin(call) {
		value, ty, prelude := emitFormalCallExpr(call, "", env)
		return prelude + fmt.Sprintf("    go.todo %q\n", "discarded_"+sanitizeName(ty)+"_"+sanitizeName(value)), true
	}
	if isFormalTypeConversionCall(call, env.module) {
		return fmt.Sprintf("    go.todo %q\n", "type_conversion_stmt"), true
	}
	argHints := []string(nil)
	if sig, ok := formalExprFuncSig(call.Fun, env); ok && len(sig.params) == len(call.Args) {
		argHints = sig.params
	}
	if text, ok := emitFormalMethodCallStmt(call, env, argHints); ok {
		return text, true
	}
	args, argTys, prelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	var buf strings.Builder
	buf.WriteString(prelude)

	if symbol, ok := formalDirectCallSymbol(call.Fun, argTys, nil, env); ok {
		buf.WriteString(fmt.Sprintf("    func.call @%s(%s) : (%s) -> ()\n", symbol, strings.Join(args, ", "), strings.Join(argTys, ", ")))
		return buf.String(), true
	}

	sig, ok := formalExprFuncSig(call.Fun, env)
	if !ok {
		return fmt.Sprintf("    go.todo %q\n", "indirect_call_stmt"), true
	}
	if len(sig.results) != 0 {
		return fmt.Sprintf("    go.todo %q\n", "discarded_call_result"), true
	}
	calleeTy := formatFormalFuncType(sig.params, sig.results)
	calleeValue, _, calleePrelude := emitFormalExpr(call.Fun, calleeTy, env)
	buf.WriteString(calleePrelude)
	buf.WriteString(fmt.Sprintf("    func.call_indirect %s(%s) : (%s) -> ()\n", calleeValue, strings.Join(args, ", "), strings.Join(argTys, ", ")))
	return buf.String(), true
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
	buf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", result, cond, resultTy))
	buf.WriteString(indentBlock(thenPrelude, 2))
	buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", thenValue, resultTy))
	buf.WriteString("    } else {\n")
	buf.WriteString(indentBlock(elsePrelude, 2))
	buf.WriteString(fmt.Sprintf("        scf.yield %s : %s\n", elseValue, resultTy))
	buf.WriteString("    }\n")
	buf.WriteString(fmt.Sprintf("    return %s : %s\n", result, resultTy))
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
	return body + emitFormalReturnLine(values, types), consumed, true, true
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
	buf.WriteString(fmt.Sprintf("    %s = scf.if %s -> (%s) {\n", formalIfResultBinding(result, len(resultTypes)), cond, strings.Join(resultTypes, ", ")))
	buf.WriteString(indentBlock(thenBody, 2))
	buf.WriteString(emitFormalYieldLine(thenValues, thenTypes))
	buf.WriteString("    } else {\n")
	buf.WriteString(indentBlock(elseBody, 2))
	buf.WriteString(emitFormalYieldLine(elseValues, elseTypes))
	buf.WriteString("    }\n")
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String(), formalMultiResultRefs(result, len(resultTypes)), append([]string(nil), resultTypes...), consumed, true
}

func emitFormalFallbackReturn(resultTypes []string, env *formalEnv) string {
	return emitFormalReturnValues(resultTypes, env)
}

func emitFormalReturnValues(resultTypes []string, env *formalEnv) string {
	if len(resultTypes) == 0 {
		return "    return\n"
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
	buf.WriteString(emitFormalReturnLine(values, resultTypes))
	return buf.String()
}

func emitFormalReturnLine(values []string, types []string) string {
	return fmt.Sprintf("    return %s : %s\n", strings.Join(values, ", "), strings.Join(types, ", "))
}

func emitFormalZeroValue(ty string, env *formalEnv) (string, string) {
	ty = normalizeFormalType(ty)
	switch {
	case ty == "i1":
		tmp := env.temp("const")
		return tmp, fmt.Sprintf("    %s = arith.constant false\n", tmp)
	case isFormalIntegerType(ty):
		tmp := env.temp("const")
		return tmp, fmt.Sprintf("    %s = arith.constant 0 : %s\n", tmp, ty)
	case ty == "!go.string":
		tmp := env.temp("str")
		return tmp, fmt.Sprintf("    %s = go.string_constant \"\" : !go.string\n", tmp)
	case isFormalNilableType(ty):
		tmp := env.temp("nil")
		return tmp, fmt.Sprintf("    %s = go.nil : %s\n", tmp, ty)
	default:
		tmp, prelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       "__mlse_zero_" + sanitizeName(ty),
				resultTy:   ty,
				tempPrefix: "zero",
			},
			env,
		)
		return tmp, prelude
	}
}

func emitFormalTodoValue(reason string, ty string, env *formalEnv) (string, string, string) {
	ty = normalizeFormalType(ty)
	tmp := env.temp("todo")
	return tmp, ty, fmt.Sprintf("    %s = go.todo_value %q : %s\n", tmp, reason, ty)
}

func syntheticFormalAssignType(name string, env *formalEnv) string {
	if ty := env.typeOf(name); ty != formalOpaqueType("value") {
		return ty
	}
	switch name {
	case "ok", "found", "exists":
		return "i1"
	case "err":
		return "!go.error"
	default:
		return formalOpaqueType("value")
	}
}

func formalMutatedOuterNames(base *formalEnv, thenEnv *formalEnv, elseEnv *formalEnv, hasElse bool) []string {
	names := make([]string, 0)
	for name, binding := range base.locals {
		thenBinding, ok := thenEnv.locals[name]
		if !ok {
			continue
		}
		elseBinding := binding
		if hasElse {
			var ok bool
			elseBinding, ok = elseEnv.locals[name]
			if !ok {
				continue
			}
		}
		if binding.current != thenBinding.current || binding.current != elseBinding.current || binding.ty != thenBinding.ty || binding.ty != elseBinding.ty {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	return names
}

func collectAssignedOuterNames(stmts []ast.Stmt, env *formalEnv, exclude string) []string {
	excludes := make(map[string]struct{})
	if exclude != "" {
		excludes[exclude] = struct{}{}
	}
	return collectAssignedOuterNamesWithExcludes(stmts, env, excludes)
}

func collectAssignedOuterNamesWithExcludes(stmts []ast.Stmt, env *formalEnv, excludes map[string]struct{}) []string {
	seen := make(map[string]struct{})
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" {
					continue
				}
				if _, skip := excludes[ident.Name]; skip {
					continue
				}
				if _, ok := env.locals[ident.Name]; ok {
					seen[ident.Name] = struct{}{}
				}
			}
		case *ast.IncDecStmt:
			ident, ok := s.X.(*ast.Ident)
			if !ok || ident.Name == "_" {
				continue
			}
			if _, skip := excludes[ident.Name]; skip {
				continue
			}
			if _, ok := env.locals[ident.Name]; ok {
				seen[ident.Name] = struct{}{}
			}
		}
	}
	names := make([]string, 0, len(seen))
	for name := range seen {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func matchFormalCountedLoopCond(expr ast.Expr) (string, ast.Expr, bool) {
	binary, ok := expr.(*ast.BinaryExpr)
	if !ok || binary.Op != token.LSS {
		return "", nil, false
	}
	ident, ok := binary.X.(*ast.Ident)
	if !ok || ident.Name == "_" {
		return "", nil, false
	}
	return ident.Name, binary.Y, true
}

func isFormalLoopIncrement(stmt ast.Stmt, ivName string) bool {
	switch s := stmt.(type) {
	case *ast.IncDecStmt:
		ident, ok := s.X.(*ast.Ident)
		return ok && ident.Name == ivName && s.Tok == token.INC
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 || s.Tok != token.ASSIGN {
			return false
		}
		lhs, ok := s.Lhs[0].(*ast.Ident)
		if !ok || lhs.Name != ivName {
			return false
		}
		binary, ok := s.Rhs[0].(*ast.BinaryExpr)
		if !ok || binary.Op != token.ADD {
			return false
		}
		x, ok := binary.X.(*ast.Ident)
		if !ok || x.Name != ivName {
			return false
		}
		lit, ok := binary.Y.(*ast.BasicLit)
		return ok && lit.Kind == token.INT && lit.Value == "1"
	default:
		return false
	}
}

func inferFormalExprType(expr ast.Expr, env *formalEnv) string {
	if env != nil && env.module != nil {
		if ty, ok := formalTypedExprType(expr, env.module); ok && isFormalTypedInfoUsableType(ty) {
			return ty
		}
	}
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			return "i32"
		case token.STRING:
			return "!go.string"
		}
	case *ast.Ident:
		switch e.Name {
		case "nil":
			return "!go.error"
		case "true", "false":
			return "i1"
		default:
			if env != nil && env.module != nil {
				if sig, ok := env.module.definedFuncs[env.module.topLevelSymbol(e.Name)]; ok {
					return formatFormalFuncType(sig.params, sig.results)
				}
			}
			return env.typeOf(e.Name)
		}
	case *ast.BinaryExpr:
		switch e.Op {
		case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ, token.LAND, token.LOR:
			return "i1"
		default:
			return inferFormalExprType(e.X, env)
		}
	case *ast.CallExpr:
		return inferFormalCallResultType(e, "", env)
	case *ast.FuncLit:
		return formalTypeExprToMLIR(e.Type, env.module)
	case *ast.IndexExpr:
		return formalIndexResultType(inferFormalExprType(e.X, env))
	case *ast.ParenExpr:
		return inferFormalExprType(e.X, env)
	case *ast.SelectorExpr:
		return formalOpaqueType("value")
	case *ast.StarExpr:
		return formalDerefType(inferFormalExprType(e.X, env))
	case *ast.TypeAssertExpr:
		if e.Type != nil {
			return formalTypeExprToMLIR(e.Type, env.module)
		}
		return formalOpaqueType("value")
	case *ast.UnaryExpr:
		if e.Op == token.AND {
			return "!go.ptr<" + inferFormalExprType(e.X, env) + ">"
		}
		return inferFormalExprType(e.X, env)
	}
	return formalOpaqueType("value")
}

func formalOpaqueType(name string) string {
	return "!go.named<\"" + sanitizeName(name) + "\">"
}

func normalizeFormalType(ty string) string {
	if ty == "" {
		return formalOpaqueType("value")
	}
	return ty
}

func normalizeFormalElementType(ty string) string {
	return normalizeFormalType(ty)
}

func isFormalIntegerType(ty string) bool {
	switch ty {
	case "i8", "i16", "i32", "i64", "index":
		return true
	}
	return false
}

func isFormalNilableType(ty string) bool {
	return ty == "!go.error" || strings.HasPrefix(ty, "!go.ptr<") || strings.HasPrefix(ty, "!go.slice<")
}

func isMakeBuiltin(call *ast.CallExpr) bool {
	ident, ok := call.Fun.(*ast.Ident)
	return ok && ident.Name == "make" && len(call.Args) > 0
}

func isFormalTypeConversionCall(call *ast.CallExpr, module *formalModuleContext) bool {
	if len(call.Args) != 1 {
		return false
	}
	return isFormalTypeExpr(call.Fun, module)
}

func isFormalTypeExpr(expr ast.Expr, module *formalModuleContext) bool {
	switch t := expr.(type) {
	case *ast.Ident:
		switch t.Name {
		case "any", "bool", "byte", "complex128", "complex64", "error", "float32", "float64", "int", "int16", "int32", "int64", "int8", "rune", "string", "uint", "uint16", "uint32", "uint64", "uint8", "uintptr":
			return true
		default:
			return module != nil && module.isNamedType(t.Name)
		}
	case *ast.StarExpr, *ast.ArrayType, *ast.InterfaceType, *ast.StructType, *ast.MapType, *ast.FuncType, *ast.ChanType, *ast.Ellipsis:
		return true
	case *ast.ParenExpr:
		return isFormalTypeExpr(t.X, module)
	default:
		return false
	}
}

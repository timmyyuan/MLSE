package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"sort"
	"strconv"
	"strings"
)

type formalBinding struct {
	current string
	ty      string
}

type formalEnv struct {
	locals map[string]*formalBinding
	tempID int
}

func newFormalEnv() *formalEnv {
	return &formalEnv{locals: make(map[string]*formalBinding)}
}

func (e *formalEnv) define(name string, ty string) string {
	if binding, ok := e.locals[name]; ok {
		if ty != "" {
			binding.ty = ty
		}
		return binding.current
	}
	ssa := "%" + sanitizeName(name)
	e.locals[name] = &formalBinding{current: ssa, ty: ty}
	return ssa
}

func (e *formalEnv) assign(name string, ty string) string {
	if _, ok := e.locals[name]; !ok {
		return e.define(name, ty)
	}
	binding := e.locals[name]
	if ty != "" {
		binding.ty = ty
	}
	return binding.current
}

func (e *formalEnv) defineOrAssign(name string, ty string) string {
	if _, ok := e.locals[name]; ok {
		return e.assign(name, ty)
	}
	return e.define(name, ty)
}

func (e *formalEnv) bindValue(name string, value string, ty string) {
	if binding, ok := e.locals[name]; ok {
		binding.current = value
		if ty != "" {
			binding.ty = ty
		}
		return
	}
	e.locals[name] = &formalBinding{current: value, ty: ty}
}

func (e *formalEnv) use(name string) string {
	if binding, ok := e.locals[name]; ok {
		return binding.current
	}
	return e.define(name, formalOpaqueType("value"))
}

func (e *formalEnv) typeOf(name string) string {
	if binding, ok := e.locals[name]; ok && binding.ty != "" {
		return binding.ty
	}
	return formalOpaqueType("value")
}

func (e *formalEnv) temp(prefix string) string {
	e.tempID++
	return fmt.Sprintf("%%%s%d", sanitizeName(prefix), e.tempID)
}

func (e *formalEnv) clone() *formalEnv {
	cloned := &formalEnv{
		locals: make(map[string]*formalBinding, len(e.locals)),
		tempID: e.tempID,
	}
	for name, binding := range e.locals {
		copied := *binding
		cloned.locals[name] = &copied
	}
	return cloned
}

func syncFormalTempID(target *formalEnv, others ...*formalEnv) {
	for _, other := range others {
		if other != nil && other.tempID > target.tempID {
			target.tempID = other.tempID
		}
	}
}

func emitFormalFunc(fn *ast.FuncDecl) string {
	env := newFormalEnv()
	params := emitFormalParams(fn.Type.Params, env)
	results := emitFormalResultTypes(fn.Type.Results)

	var buf strings.Builder
	switch len(results) {
	case 0:
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", ")))
	case 1:
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) -> %s {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", "), results[0]))
	default:
		buf.WriteString(fmt.Sprintf("  func.func @%s(%s) -> (%s) {\n", sanitizeName(fn.Name.Name), strings.Join(params, ", "), strings.Join(results, ", ")))
	}

	terminated := false
	if fn.Body == nil {
		buf.WriteString("    go.todo \"missing_body\"\n")
	} else {
		body, term := emitFormalFuncBlock(fn.Body.List, env, results)
		buf.WriteString(body)
		terminated = term
	}
	if !terminated {
		buf.WriteString("    go.todo \"implicit_return_placeholder\"\n")
		buf.WriteString(emitFormalFallbackReturn(results, env))
	}
	buf.WriteString("  }\n")
	return buf.String()
}

func emitFormalFuncBlock(stmts []ast.Stmt, env *formalEnv, resultTypes []string) (string, bool) {
	var buf strings.Builder
	for i := 0; i < len(stmts); i++ {
		if ifStmt, ok := stmts[i].(*ast.IfStmt); ok {
			var next ast.Stmt
			if i+1 < len(stmts) {
				next = stmts[i+1]
			}
			if text, consumed, term, ok := emitFormalTerminatingIfStmt(ifStmt, next, env, resultTypes); ok {
				buf.WriteString(text)
				if term {
					return buf.String(), true
				}
				i += consumed - 1
				continue
			}
		}
		text, term := emitFormalStmt(stmts[i], env, resultTypes)
		buf.WriteString(text)
		if term {
			return buf.String(), true
		}
	}
	return buf.String(), false
}

func emitFormalRegionBlock(stmts []ast.Stmt, env *formalEnv) (string, bool) {
	var buf strings.Builder
	for _, stmt := range stmts {
		text, term := emitFormalStmt(stmt, env, nil)
		buf.WriteString(text)
		if term {
			return buf.String(), true
		}
	}
	return buf.String(), false
}

func emitFormalParams(fields *ast.FieldList, env *formalEnv) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := goTypeToFormalMLIR(field.Type)
		if len(field.Names) == 0 {
			name := env.temp("arg")
			out = append(out, fmt.Sprintf("%s: %s", name, ty))
			continue
		}
		for _, name := range field.Names {
			ssa := env.define(name.Name, ty)
			out = append(out, fmt.Sprintf("%s: %s", ssa, ty))
		}
	}
	return out
}

func emitFormalResultTypes(fields *ast.FieldList) []string {
	if fields == nil || len(fields.List) == 0 {
		return nil
	}
	var out []string
	for _, field := range fields.List {
		ty := goTypeToFormalMLIR(field.Type)
		count := len(field.Names)
		if count == 0 {
			count = 1
		}
		for i := 0; i < count; i++ {
			out = append(out, ty)
		}
	}
	return out
}

func emitFormalStmt(stmt ast.Stmt, env *formalEnv, resultTypes []string) (string, bool) {
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
	case *ast.IncDecStmt:
		return emitFormalIncDecStmt(s, env), false
	case *ast.EmptyStmt:
		return "", false
	default:
		return fmt.Sprintf("    go.todo %q\n", shortNodeName(stmt)), false
	}
}

func emitFormalAssignStmt(s *ast.AssignStmt, env *formalEnv) string {
	if len(s.Lhs) != len(s.Rhs) {
		return fmt.Sprintf("    go.todo %q\n", "assign_arity_mismatch")
	}

	var buf strings.Builder
	for i := range s.Lhs {
		ident, ok := s.Lhs[i].(*ast.Ident)
		if !ok || ident.Name == "_" {
			buf.WriteString(fmt.Sprintf("    go.todo %q\n", "assign_target"))
			continue
		}
		hint := env.typeOf(ident.Name)
		if s.Tok == token.DEFINE && hint == formalOpaqueType("value") {
			hint = inferFormalExprType(s.Rhs[i], env)
		}
		value, ty, prelude := emitFormalExpr(s.Rhs[i], hint, env)
		buf.WriteString(prelude)
		switch s.Tok {
		case token.DEFINE:
			env.defineOrAssign(ident.Name, ty)
		case token.ASSIGN:
			env.assign(ident.Name, ty)
		default:
			env.assign(ident.Name, ty)
		}
		env.bindValue(ident.Name, value, ty)
	}
	return buf.String()
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
				ty = goTypeToFormalMLIR(valueSpec.Type)
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
		return "    go.todo \"IfStmt_init\"\n"
	}

	cond, condTy, prelude := emitFormalExpr(s.Cond, "i1", env)
	if condTy != "i1" {
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
		syncFormalTempID(env, thenEnv, elseEnv)
		return prelude + "    go.todo \"IfStmt_multi_merge\"\n"
	}
	syncFormalTempID(env, thenEnv, elseEnv)
	return buf.String()
}

func emitFormalForStmt(s *ast.ForStmt, env *formalEnv) string {
	if s.Init != nil || s.Post != nil || s.Cond == nil {
		return "    go.todo \"ForStmt\"\n"
	}

	ivName, upperExpr, ok := matchFormalCountedLoopCond(s.Cond)
	if !ok || len(s.Body.List) == 0 {
		return "    go.todo \"ForStmt\"\n"
	}

	bodyStmts := s.Body.List
	last := bodyStmts[len(bodyStmts)-1]
	if !isFormalLoopIncrement(last, ivName) {
		return "    go.todo \"ForStmt\"\n"
	}
	bodyStmts = bodyStmts[:len(bodyStmts)-1]

	ivInit := env.use(ivName)
	ivTy := env.typeOf(ivName)
	if !isFormalIntegerType(ivTy) {
		return "    go.todo \"ForStmt_iv_type\"\n"
	}

	upper, upperTy, upperPrelude := emitFormalExpr(upperExpr, ivTy, env)
	if upperTy != ivTy {
		return upperPrelude + "    go.todo \"ForStmt_bound_type\"\n"
	}

	carried := collectAssignedOuterNames(bodyStmts, env, ivName)
	if len(carried) > 1 {
		return upperPrelude + "    go.todo \"ForStmt_multi_iter_args\"\n"
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

	step := env.temp("const")
	ivSSA := env.temp(sanitizeName(ivName) + "_iv")
	var buf strings.Builder
	buf.WriteString(upperPrelude)
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
		bodyText, bodyTerm := emitFormalRegionBlock(bodyStmts, bodyEnv)
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
	bodyText, bodyTerm := emitFormalRegionBlock(bodyStmts, bodyEnv)
	syncFormalTempID(env, bodyEnv)
	if bodyTerm {
		return buf.String() + "    go.todo \"ForStmt_returning_body\"\n"
	}
	yieldValue := bodyEnv.use(carriedName)
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

func emitFormalExpr(expr ast.Expr, hintedTy string, env *formalEnv) (string, string, string) {
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			tmp := env.temp("const")
			return tmp, "i32", fmt.Sprintf("    %s = arith.constant %s : i32\n", tmp, e.Value)
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
			if !isFormalNilableType(ty) {
				ty = "!go.error"
			}
			tmp := env.temp("nil")
			return tmp, ty, fmt.Sprintf("    %s = go.nil : %s\n", tmp, ty)
		case "true", "false":
			tmp := env.temp("const")
			return tmp, "i1", fmt.Sprintf("    %s = arith.constant %s\n", tmp, e.Name)
		default:
			return env.use(e.Name), env.typeOf(e.Name), ""
		}
	case *ast.BinaryExpr:
		return emitFormalBinaryExpr(e, hintedTy, env)
	case *ast.CallExpr:
		return emitFormalCallExpr(e, hintedTy, env)
	case *ast.ParenExpr:
		return emitFormalExpr(e.X, hintedTy, env)
	case *ast.UnaryExpr:
		return emitFormalUnaryExpr(e, hintedTy, env)
	default:
		return emitFormalTodoValue(shortNodeName(expr), normalizeFormalType(hintedTy), env)
	}
}

func emitFormalBinaryExpr(expr *ast.BinaryExpr, hintedTy string, env *formalEnv) (string, string, string) {
	lhs, lhsTy, lhsPrelude := emitFormalExpr(expr.X, "", env)
	rhs, rhsTy, rhsPrelude := emitFormalExpr(expr.Y, lhsTy, env)
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

	if !isFormalIntegerType(operandTy) && operandTy != "i1" {
		return emitFormalTodoValue("binary_"+sanitizeName(expr.Op.String()), normalizeFormalType(hintedTy), env)
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

func emitFormalCallExpr(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string) {
	if isMakeBuiltin(call) {
		return emitFormalMakeCall(call, env)
	}
	if isTypeExpr(call.Fun) && len(call.Args) == 1 {
		targetTy := goTypeToFormalMLIR(call.Fun)
		value, valueTy, prelude := emitFormalExpr(call.Args[0], targetTy, env)
		if valueTy == targetTy {
			return value, targetTy, prelude
		}
		todoValue, todoTy, todoPrelude := emitFormalTodoValue("type_conversion", targetTy, env)
		return todoValue, todoTy, prelude + todoPrelude
	}

	callee := formalCalleeName(call.Fun)
	if callee == "" {
		return emitFormalTodoValue("indirect_call", normalizeFormalType(hintedTy), env)
	}

	var (
		args   []string
		argTys []string
		buf    strings.Builder
	)
	for _, arg := range call.Args {
		value, ty, prelude := emitFormalExpr(arg, "", env)
		buf.WriteString(prelude)
		args = append(args, value)
		argTys = append(argTys, ty)
	}

	resultTy := inferFormalCallResultType(call, hintedTy, env)
	tmp := env.temp("call")
	buf.WriteString(fmt.Sprintf("    %s = func.call @%s(%s) : (%s) -> %s\n", tmp, callee, strings.Join(args, ", "), strings.Join(argTys, ", "), resultTy))
	return tmp, resultTy, buf.String()
}

func emitFormalMakeCall(call *ast.CallExpr, env *formalEnv) (string, string, string) {
	if len(call.Args) < 2 {
		return emitFormalTodoValue("make_missing_len", formalOpaqueType("make"), env)
	}
	targetTy := goTypeToFormalMLIR(call.Args[0])
	length, lengthTy, lengthPrelude := emitFormalExpr(call.Args[1], "i32", env)
	capacity := length
	capacityPrelude := ""
	if len(call.Args) > 2 {
		capacity, _, capacityPrelude = emitFormalExpr(call.Args[2], lengthTy, env)
	}
	if !strings.HasPrefix(targetTy, "!go.slice<") {
		return emitFormalTodoValue("make_non_slice", targetTy, env)
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
	default:
		return emitFormalTodoValue("unary_"+sanitizeName(expr.Op.String()), normalizeFormalType(hintedTy), env)
	}
}

func emitFormalCallStmt(call *ast.CallExpr, env *formalEnv) (string, bool) {
	if isMakeBuiltin(call) {
		value, ty, prelude := emitFormalCallExpr(call, "", env)
		return prelude + fmt.Sprintf("    go.todo %q\n", "discarded_"+sanitizeName(ty)+"_"+sanitizeName(value)), true
	}
	if isTypeExpr(call.Fun) {
		return fmt.Sprintf("    go.todo %q\n", "type_conversion_stmt"), true
	}
	callee := formalCalleeName(call.Fun)
	if callee == "" {
		return fmt.Sprintf("    go.todo %q\n", "indirect_call_stmt"), true
	}

	var (
		args   []string
		argTys []string
		buf    strings.Builder
	)
	for _, arg := range call.Args {
		value, ty, prelude := emitFormalExpr(arg, "", env)
		buf.WriteString(prelude)
		args = append(args, value)
		argTys = append(argTys, ty)
	}
	buf.WriteString(fmt.Sprintf("    func.call @%s(%s) : (%s) -> ()\n", callee, strings.Join(args, ", "), strings.Join(argTys, ", ")))
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

	cond, condTy, prelude := emitFormalExpr(s.Cond, "i1", env)
	if condTy != "i1" {
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
		value, _, prelude := emitFormalTodoValue("zero_value", ty, env)
		return value, prelude
	}
}

func emitFormalTodoValue(reason string, ty string, env *formalEnv) (string, string, string) {
	ty = normalizeFormalType(ty)
	tmp := env.temp("todo")
	return tmp, ty, fmt.Sprintf("    %s = go.todo_value %q : %s\n", tmp, reason, ty)
}

func extractSingleReturnExpr(stmts []ast.Stmt) (ast.Expr, bool) {
	if len(stmts) != 1 {
		return nil, false
	}
	ret, ok := stmts[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil, false
	}
	return ret.Results[0], true
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
	seen := make(map[string]struct{})
	for _, stmt := range stmts {
		switch s := stmt.(type) {
		case *ast.AssignStmt:
			for _, lhs := range s.Lhs {
				ident, ok := lhs.(*ast.Ident)
				if !ok || ident.Name == "_" || ident.Name == exclude {
					continue
				}
				if _, ok := env.locals[ident.Name]; ok {
					seen[ident.Name] = struct{}{}
				}
			}
		case *ast.IncDecStmt:
			ident, ok := s.X.(*ast.Ident)
			if !ok || ident.Name == "_" || ident.Name == exclude {
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
	case *ast.ParenExpr:
		return inferFormalExprType(e.X, env)
	case *ast.UnaryExpr:
		return inferFormalExprType(e.X, env)
	}
	return formalOpaqueType("value")
}

func inferFormalCallResultType(call *ast.CallExpr, hintedTy string, env *formalEnv) string {
	if hintedTy != "" {
		return normalizeFormalType(hintedTy)
	}
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		switch fun.Name {
		case "len", "cap", "copy":
			return "i32"
		case "append":
			if len(call.Args) > 0 {
				return inferFormalExprType(call.Args[0], env)
			}
			return "!go.slice<i32>"
		case "make":
			if len(call.Args) > 0 {
				return goTypeToFormalMLIR(call.Args[0])
			}
		}
	case *ast.SelectorExpr:
		switch renderSelector(fun) {
		case "fmt.Sprintf":
			return "!go.string"
		case "fmt.Errorf":
			return "!go.error"
		}
	}
	return formalOpaqueType("result")
}

func goTypeToFormalMLIR(expr ast.Expr) string {
	switch t := expr.(type) {
	case nil:
		return formalOpaqueType("unit")
	case *ast.Ident:
		switch t.Name {
		case "int":
			return "i32"
		case "bool":
			return "i1"
		case "string":
			return "!go.string"
		case "error":
			return "!go.error"
		case "any", "interface{}":
			return formalOpaqueType("any")
		default:
			return "!go.named<\"" + sanitizeName(t.Name) + "\">"
		}
	case *ast.SelectorExpr:
		return "!go.named<\"" + sanitizeName(renderSelector(t)) + "\">"
	case *ast.StarExpr:
		return "!go.ptr<" + goTypeToFormalMLIR(t.X) + ">"
	case *ast.ArrayType:
		if t.Len == nil {
			return "!go.slice<" + normalizeFormalElementType(goTypeToFormalMLIR(t.Elt)) + ">"
		}
		return formalOpaqueType("array")
	case *ast.MapType:
		return formalOpaqueType("map")
	case *ast.InterfaceType:
		return formalOpaqueType("interface")
	case *ast.FuncType:
		return formalOpaqueType("func")
	case *ast.StructType:
		return formalOpaqueType("struct")
	case *ast.ChanType:
		return formalOpaqueType("chan")
	case *ast.Ellipsis:
		return formalOpaqueType("vararg")
	case *ast.ParenExpr:
		return goTypeToFormalMLIR(t.X)
	default:
		return formalOpaqueType("type")
	}
}

func formalCalleeName(expr ast.Expr) string {
	switch callee := expr.(type) {
	case *ast.Ident:
		return sanitizeName(callee.Name)
	case *ast.SelectorExpr:
		return sanitizeName(renderSelector(callee))
	default:
		return ""
	}
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
	if strings.HasPrefix(ty, "!go.slice<") || strings.HasPrefix(ty, "!go.ptr<") {
		return formalOpaqueType("element")
	}
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

func isTypeExpr(expr ast.Expr) bool {
	switch expr.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.StarExpr, *ast.ArrayType, *ast.InterfaceType, *ast.StructType, *ast.MapType, *ast.FuncType, *ast.ChanType, *ast.Ellipsis, *ast.ParenExpr:
		return true
	default:
		return false
	}
}

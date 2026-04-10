package gofrontend

import (
	"go/ast"
	"go/token"
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

type formalBinaryOperands struct {
	lhs        string
	lhsTy      string
	rhs        string
	rhsTy      string
	operandTy  string
	resultTy   string
	lhsPrelude string
	rhsPrelude string
}

type formalBinaryLowering struct {
	op        string
	resultTy  string
	operandTy string
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
	lowering, ok := planFormalBinaryLowering(expr, operandTy, env)
	if !ok {
		return emitFormalTodoValue("binary_"+sanitizeName(expr.Op.String()), normalizeFormalType(hintedTy), env)
	}
	resultTy := lowering.resultTy
	op := lowering.op

	operandTy = chooseFormalCommonType(lhsTy, rhsTy)
	if isFormalNilExpr(expr.X) {
		operandTy = normalizeFormalType(rhsTy)
	}
	if isFormalNilExpr(expr.Y) {
		operandTy = normalizeFormalType(lhsTy)
	}
	if lowering.operandTy == "i1" {
		operandTy = "i1"
	}
	if !formalBinaryExprReturnsBool(expr.Op) {
		resultTy = operandTy
	}
	if isFormalFloatType(operandTy) {
		if floatLowering, ok := planFormalFloatBinaryLowering(expr.Op, operandTy); ok {
			op = floatLowering.op
			resultTy = floatLowering.resultTy
		}
	}
	if !isFormalIntegerType(operandTy) && !isFormalFloatType(operandTy) && operandTy != "i1" {
		return emitFormalNonPrimitiveBinaryExpr(expr, formalBinaryOperands{
			lhs:        lhs,
			lhsTy:      lhsTy,
			rhs:        rhs,
			rhsTy:      rhsTy,
			operandTy:  operandTy,
			resultTy:   resultTy,
			lhsPrelude: lhsPrelude,
			rhsPrelude: rhsPrelude,
		}, env)
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
	buf.WriteString(emitFormalLinef(expr, env, "    %s = %s %s, %s : %s", tmp, op, lhs, rhs, operandTy))
	return tmp, resultTy, buf.String()
}

func emitFormalBinaryOpValues(op token.Token, operands formalBinaryOperands, node ast.Node, env *formalEnv) (string, string, string, bool) {
	lhs := operands.lhs
	lhsTy := operands.lhsTy
	rhs := operands.rhs
	rhsTy := operands.rhsTy
	operandTy := chooseFormalCommonType(lhsTy, rhsTy)
	lowering, ok := planFormalBinaryLowering(&ast.BinaryExpr{Op: op}, operandTy, env)
	if !ok {
		return "", "", "", false
	}
	resultTy := lowering.resultTy
	operandTy = chooseFormalCommonType(lhsTy, rhsTy)
	if lowering.operandTy == "i1" {
		operandTy = "i1"
	}
	if !formalBinaryExprReturnsBool(op) {
		resultTy = operandTy
	}
	if !isFormalIntegerType(operandTy) && !isFormalFloatType(operandTy) && operandTy != "i1" {
		if op == token.ADD && normalizeFormalType(operandTy) == "!go.string" {
			var buf strings.Builder
			if coercedLHS, _, coercedPrelude, ok := emitFormalCoerceValue(lhs, lhsTy, operandTy, env); ok {
				lhs = coercedLHS
				buf.WriteString(coercedPrelude)
			}
			if coercedRHS, _, coercedPrelude, ok := emitFormalCoerceValue(rhs, rhsTy, operandTy, env); ok {
				rhs = coercedRHS
				buf.WriteString(coercedPrelude)
			}
			tmp, helperPrelude := emitFormalHelperCall(
				formalHelperCallSpec{
					base:       formalRuntimeAddSymbol(operandTy).String(),
					args:       []string{lhs, rhs},
					argTys:     []string{operandTy, operandTy},
					resultTy:   operandTy,
					tempPrefix: "add",
				},
				env,
			)
			buf.WriteString(helperPrelude)
			return tmp, operandTy, buf.String(), true
		}
		return "", "", "", false
	}
	var buf strings.Builder
	if coercedLHS, _, coercedPrelude, ok := emitFormalCoerceValue(lhs, lhsTy, operandTy, env); ok {
		lhs = coercedLHS
		buf.WriteString(coercedPrelude)
	}
	if coercedRHS, _, coercedPrelude, ok := emitFormalCoerceValue(rhs, rhsTy, operandTy, env); ok {
		rhs = coercedRHS
		buf.WriteString(coercedPrelude)
	}
	tmp := env.temp("bin")
	buf.WriteString(emitFormalLinef(node, env, "    %s = %s %s, %s : %s", tmp, lowering.op, lhs, rhs, operandTy))
	return tmp, resultTy, buf.String(), true
}

func formalBinaryExprReturnsBool(op token.Token) bool {
	switch op {
	case token.EQL, token.NEQ, token.GTR, token.LSS, token.GEQ, token.LEQ, token.LAND, token.LOR:
		return true
	default:
		return false
	}
}

func planFormalBinaryLowering(expr *ast.BinaryExpr, operandTy string, env *formalEnv) (formalBinaryLowering, bool) {
	unsignedIntegerSemantics := formalExprUsesUnsignedIntegerSemantics(expr.X, env.module) || formalExprUsesUnsignedIntegerSemantics(expr.Y, env.module)
	switch expr.Op {
	case token.ADD:
		return formalBinaryLowering{op: "arith.addi", resultTy: operandTy, operandTy: operandTy}, true
	case token.SUB:
		return formalBinaryLowering{op: "arith.subi", resultTy: operandTy, operandTy: operandTy}, true
	case token.MUL:
		return formalBinaryLowering{op: "arith.muli", resultTy: operandTy, operandTy: operandTy}, true
	case token.QUO:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.divui", resultTy: operandTy, operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.divsi", resultTy: operandTy, operandTy: operandTy}, true
	case token.REM:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.remui", resultTy: operandTy, operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.remsi", resultTy: operandTy, operandTy: operandTy}, true
	case token.EQL:
		return formalBinaryLowering{op: "arith.cmpi eq,", resultTy: "i1", operandTy: operandTy}, true
	case token.NEQ:
		return formalBinaryLowering{op: "arith.cmpi ne,", resultTy: "i1", operandTy: operandTy}, true
	case token.GTR:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.cmpi ugt,", resultTy: "i1", operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.cmpi sgt,", resultTy: "i1", operandTy: operandTy}, true
	case token.LSS:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.cmpi ult,", resultTy: "i1", operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.cmpi slt,", resultTy: "i1", operandTy: operandTy}, true
	case token.GEQ:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.cmpi uge,", resultTy: "i1", operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.cmpi sge,", resultTy: "i1", operandTy: operandTy}, true
	case token.LEQ:
		if unsignedIntegerSemantics {
			return formalBinaryLowering{op: "arith.cmpi ule,", resultTy: "i1", operandTy: operandTy}, true
		}
		return formalBinaryLowering{op: "arith.cmpi sle,", resultTy: "i1", operandTy: operandTy}, true
	case token.LAND:
		return formalBinaryLowering{op: "arith.andi", resultTy: "i1", operandTy: "i1"}, true
	case token.LOR:
		return formalBinaryLowering{op: "arith.ori", resultTy: "i1", operandTy: "i1"}, true
	case token.AND:
		return formalBinaryLowering{op: "arith.andi", resultTy: operandTy, operandTy: operandTy}, true
	case token.OR:
		return formalBinaryLowering{op: "arith.ori", resultTy: operandTy, operandTy: operandTy}, true
	case token.XOR:
		return formalBinaryLowering{op: "arith.xori", resultTy: operandTy, operandTy: operandTy}, true
	default:
		return formalBinaryLowering{}, false
	}
}

func planFormalFloatBinaryLowering(op token.Token, operandTy string) (formalBinaryLowering, bool) {
	switch op {
	case token.ADD:
		return formalBinaryLowering{op: "arith.addf", resultTy: operandTy, operandTy: operandTy}, true
	case token.SUB:
		return formalBinaryLowering{op: "arith.subf", resultTy: operandTy, operandTy: operandTy}, true
	case token.MUL:
		return formalBinaryLowering{op: "arith.mulf", resultTy: operandTy, operandTy: operandTy}, true
	case token.QUO:
		return formalBinaryLowering{op: "arith.divf", resultTy: operandTy, operandTy: operandTy}, true
	case token.EQL:
		return formalBinaryLowering{op: "arith.cmpf oeq,", resultTy: "i1", operandTy: operandTy}, true
	case token.NEQ:
		return formalBinaryLowering{op: "arith.cmpf une,", resultTy: "i1", operandTy: operandTy}, true
	case token.GTR:
		return formalBinaryLowering{op: "arith.cmpf ogt,", resultTy: "i1", operandTy: operandTy}, true
	case token.LSS:
		return formalBinaryLowering{op: "arith.cmpf olt,", resultTy: "i1", operandTy: operandTy}, true
	case token.GEQ:
		return formalBinaryLowering{op: "arith.cmpf oge,", resultTy: "i1", operandTy: operandTy}, true
	case token.LEQ:
		return formalBinaryLowering{op: "arith.cmpf ole,", resultTy: "i1", operandTy: operandTy}, true
	default:
		return formalBinaryLowering{}, false
	}
}

func emitFormalNonPrimitiveBinaryExpr(expr *ast.BinaryExpr, spec formalBinaryOperands, env *formalEnv) (string, string, string) {
	if expr.Op == token.ADD && spec.operandTy == "!go.string" {
		tmp, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeAddSymbol(spec.operandTy).String(),
				args:       []string{spec.lhs, spec.rhs},
				argTys:     []string{spec.operandTy, spec.operandTy},
				resultTy:   spec.operandTy,
				tempPrefix: "add",
			},
			env,
		)
		return tmp, spec.operandTy, spec.lhsPrelude + spec.rhsPrelude + helperPrelude
	}
	if expr.Op == token.EQL || expr.Op == token.NEQ {
		if supportsFormalGoCompareOp(expr, spec.operandTy) {
			return emitFormalGoCompare(
				formalGoCompareSpec{
					op:         expr.Op,
					lhs:        spec.lhs,
					rhs:        spec.rhs,
					operandTy:  spec.operandTy,
					lhsPrelude: spec.lhsPrelude,
					rhsPrelude: spec.rhsPrelude,
				},
				env,
			)
		}
		helperBase := formalRuntimeEqSymbol(spec.operandTy).String()
		if expr.Op == token.NEQ {
			helperBase = formalRuntimeNeqSymbol(spec.operandTy).String()
		}
		tmp, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       helperBase,
				args:       []string{spec.lhs, spec.rhs},
				argTys:     []string{spec.operandTy, spec.operandTy},
				resultTy:   "i1",
				tempPrefix: "cmp",
			},
			env,
		)
		return tmp, "i1", spec.lhsPrelude + spec.rhsPrelude + helperPrelude
	}
	if coercedLHS, _, coercedPrelude, ok := emitFormalCoerceValue(spec.lhs, spec.lhsTy, spec.operandTy, env); ok {
		spec.lhs = coercedLHS
		spec.lhsPrelude += coercedPrelude
	}
	if coercedRHS, _, coercedPrelude, ok := emitFormalCoerceValue(spec.rhs, spec.rhsTy, spec.operandTy, env); ok {
		spec.rhs = coercedRHS
		spec.rhsPrelude += coercedPrelude
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalRuntimeBinaryOpSymbol(formalBinaryOpHelperName(expr.Op), spec.operandTy).String(),
			args:       []string{spec.lhs, spec.rhs},
			argTys:     []string{spec.operandTy, spec.operandTy},
			resultTy:   normalizeFormalType(spec.resultTy),
			tempPrefix: "bin",
		},
		env,
	)
	return tmp, normalizeFormalType(spec.resultTy), spec.lhsPrelude + spec.rhsPrelude + helperPrelude
}

func formalBinaryOpHelperName(op token.Token) string {
	switch op {
	case token.ADD:
		return "add"
	case token.SUB:
		return "sub"
	case token.MUL:
		return "mul"
	case token.QUO:
		return "quo"
	case token.EQL:
		return "eq"
	case token.NEQ:
		return "neq"
	case token.GTR:
		return "gtr"
	case token.LSS:
		return "lss"
	case token.GEQ:
		return "geq"
	case token.LEQ:
		return "leq"
	case token.LAND:
		return "land"
	case token.LOR:
		return "lor"
	default:
		return sanitizeName(op.String())
	}
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
	buf.WriteString(emitFormalLinef(nil, env,
		"    %s = %s %s, %s : (%s, %s) -> i1",
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

func isFormalNilExpr(expr ast.Expr) bool {
	ident, ok := expr.(*ast.Ident)
	return ok && ident.Name == "nil"
}

func emitFormalUnaryExpr(expr *ast.UnaryExpr, hintedTy string, env *formalEnv) (string, string, string) {
	switch expr.Op {
	case token.ADD:
		value, ty, prelude := emitFormalExpr(expr.X, hintedTy, env)
		coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(value, ty, hintedTy, env)
		return coercedValue, coercedTy, prelude + coercedPrelude
	case token.SUB:
		value, ty, prelude := emitFormalExpr(expr.X, hintedTy, env)
		if !isFormalIntegerType(ty) {
			return emitFormalTodoValue("unary_sub", normalizeFormalType(hintedTy), env)
		}
		zero, zeroPrelude := emitFormalZeroValue(ty, env)
		tmp := env.temp("neg")
		return tmp, ty, prelude + zeroPrelude + emitFormalLinef(expr, env, "    %s = arith.subi %s, %s : %s", tmp, zero, value, ty)
	case token.NOT:
		value, ty, prelude := emitFormalExpr(expr.X, "i1", env)
		if ty != "i1" {
			return emitFormalTodoValue("unary_not", normalizeFormalType(hintedTy), env)
		}
		one := env.temp("const")
		tmp := env.temp("not")
		return tmp, "i1", prelude +
			emitFormalLinef(expr, env, "    %s = arith.constant true", one) +
			emitFormalLinef(expr, env, "    %s = arith.xori %s, %s : i1", tmp, value, one)
	case token.XOR:
		value, ty, prelude := emitFormalExpr(expr.X, hintedTy, env)
		if !isFormalIntegerType(ty) {
			return emitFormalTodoValue("unary_xor", normalizeFormalType(hintedTy), env)
		}
		mask := env.temp("const")
		tmp := env.temp("not")
		return tmp, ty, prelude +
			emitFormalLinef(expr, env, "    %s = arith.constant -1 : %s", mask, ty) +
			emitFormalLinef(expr, env, "    %s = arith.xori %s, %s : %s", tmp, value, mask, ty)
	case token.AND:
		if composite, ok := expr.X.(*ast.CompositeLit); ok {
			return emitFormalCompositeAddrExpr(composite, hintedTy, env)
		}
		value, ty, prelude := emitFormalExpr(expr.X, "", env)
		resultTy := normalizeFormalType(hintedTy)
		if isFormalOpaquePlaceholderType(resultTy) {
			resultTy = "!go.ptr<" + ty + ">"
		}
		tmp, helperPrelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeAddrofSymbol(ty).String(),
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

func emitFormalZeroValue(ty string, env *formalEnv) (string, string) {
	ty = normalizeFormalType(ty)
	switch {
	case ty == "i1":
		tmp := env.temp("const")
		return tmp, emitFormalLinef(nil, env, "    %s = arith.constant false", tmp)
	case isFormalIntegerType(ty):
		tmp := env.temp("const")
		return tmp, emitFormalLinef(nil, env, "    %s = arith.constant 0 : %s", tmp, ty)
	case ty == "!go.string":
		tmp := env.temp("str")
		return tmp, emitFormalLinef(nil, env, "    %s = go.string_constant \"\" : !go.string", tmp)
	case isFormalNilableType(ty):
		tmp := env.temp("nil")
		return tmp, emitFormalLinef(nil, env, "    %s = go.nil : %s", tmp, ty)
	default:
		tmp, prelude := emitFormalHelperCall(
			formalHelperCallSpec{
				base:       formalRuntimeZeroSymbol(ty).String(),
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
	return tmp, ty, emitFormalLinef(nil, env, "    %s = go.todo_value %q : %s", tmp, reason, ty)
}

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
	if isFormalFloatType(operandTy) {
		switch expr.Op {
		case token.ADD:
			op = "arith.addf"
			resultTy = operandTy
		case token.SUB:
			op = "arith.subf"
			resultTy = operandTy
		case token.MUL:
			op = "arith.mulf"
			resultTy = operandTy
		case token.QUO:
			op = "arith.divf"
			resultTy = operandTy
		case token.EQL:
			op = "arith.cmpf oeq,"
			resultTy = "i1"
		case token.NEQ:
			op = "arith.cmpf une,"
			resultTy = "i1"
		case token.GTR:
			op = "arith.cmpf ogt,"
			resultTy = "i1"
		case token.LSS:
			op = "arith.cmpf olt,"
			resultTy = "i1"
		case token.GEQ:
			op = "arith.cmpf oge,"
			resultTy = "i1"
		case token.LEQ:
			op = "arith.cmpf ole,"
			resultTy = "i1"
		}
	}
	if !isFormalIntegerType(operandTy) && !isFormalFloatType(operandTy) && operandTy != "i1" {
		if expr.Op == token.ADD && operandTy == "!go.string" {
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
			helperBase := formalRuntimeEqSymbol(operandTy).String()
			if expr.Op == token.NEQ {
				helperBase = formalRuntimeNeqSymbol(operandTy).String()
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
				base:       formalRuntimeBinaryOpSymbol(formalBinaryOpHelperName(expr.Op), operandTy).String(),
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
	buf.WriteString(emitFormalLinef(expr, env, "    %s = %s %s, %s : %s", tmp, op, lhs, rhs, operandTy))
	return tmp, resultTy, buf.String()
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

package gofrontend

import (
	"go/ast"
	goconstant "go/constant"
	"go/types"
	"math/big"
)

func emitFormalTypedConstExpr(expr ast.Expr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	if env == nil || env.module == nil {
		return "", "", "", false
	}
	value, constTy, ok := formalTypedConstValue(expr, env.module)
	if !ok || value == nil {
		return "", "", "", false
	}
	if !formalCanMaterializeTypedConst(constTy) {
		return "", "", "", false
	}

	switch value.Kind() {
	case goconstant.Bool:
		tmp := env.temp("const")
		text := "false"
		if goconstant.BoolVal(value) {
			text = "true"
		}
		return tmp, "i1", emitFormalLinef(expr, env, "    %s = arith.constant %s", tmp, text), true
	case goconstant.String:
		tmp := env.temp("str")
		return tmp, "!go.string", emitFormalLinef(expr, env, "    %s = go.string_constant %q : !go.string", tmp, goconstant.StringVal(value)), true
	case goconstant.Int:
		litTy := normalizeFormalType(hintedTy)
		if !isFormalIntegerType(litTy) && constTy != nil {
			litTy = normalizeFormalType(goTypesTypeToFormalMLIR(constTy, env.module))
		}
		if !isFormalIntegerType(litTy) {
			litTy = formalTargetIntType(env.module)
		}
		if !formalIntegerConstFits(value, litTy) {
			return "", "", "", false
		}
		tmp := env.temp("const")
		return tmp, litTy, emitFormalLinef(expr, env, "    %s = arith.constant %s : %s", tmp, value.ExactString(), litTy), true
	default:
		return "", "", "", false
	}
}

func formalCanMaterializeTypedConst(ty types.Type) bool {
	if ty == nil {
		return true
	}
	switch t := ty.(type) {
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.UntypedBool,
			types.String, types.UntypedString,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Uintptr, types.UntypedInt, types.UntypedRune:
			return true
		default:
			return false
		}
	case *types.Alias:
		basic, ok := t.Underlying().(*types.Basic)
		if !ok {
			return false
		}
		switch basic.Kind() {
		case types.Bool, types.UntypedBool,
			types.String, types.UntypedString,
			types.Int, types.Int8, types.Int16, types.Int32, types.Int64,
			types.Uint, types.Uint8, types.Uint16, types.Uint32, types.Uint64,
			types.Uintptr, types.UntypedInt, types.UntypedRune:
			return true
		default:
			return false
		}
	default:
		return false
	}
}

func formalIntegerConstFits(value goconstant.Value, ty string) bool {
	width := 0
	switch normalizeFormalType(ty) {
	case "i8":
		width = 8
	case "i16":
		width = 16
	case "i32":
		width = 32
	case "i64":
		width = 64
	}
	if width == 0 {
		return true
	}

	var intValue big.Int
	if _, ok := intValue.SetString(value.ExactString(), 10); !ok {
		return false
	}

	var max big.Int
	max.Lsh(big.NewInt(1), uint(width-1))
	max.Sub(&max, big.NewInt(1))

	var min big.Int
	min.Lsh(big.NewInt(1), uint(width-1))
	min.Neg(&min)

	return intValue.Cmp(&min) >= 0 && intValue.Cmp(&max) <= 0
}

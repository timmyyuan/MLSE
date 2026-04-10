package gofrontend

import "go/ast"

// emitFormalStarExpr lowers `*ptr` reads through the current typed helper path.
func emitFormalStarExpr(expr *ast.StarExpr, hintedTy string, env *formalEnv) (string, string, string) {
	ptr, ptrTy, prelude := emitFormalExpr(expr.X, "", env)
	resultTy := normalizeFormalType(hintedTy)
	if isFormalOpaquePlaceholderType(resultTy) {
		resultTy = formalDerefType(ptrTy)
	}
	tmp, loadedTy, loadPrelude, ok := emitFormalLoad(ptr, ptrTy, resultTy, env)
	if ok {
		coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(tmp, loadedTy, hintedTy, env)
		return coercedValue, coercedTy, prelude + loadPrelude + coercedPrelude
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalRuntimeDerefSymbol(ptrTy).String(),
			args:       []string{ptr},
			argTys:     []string{ptrTy},
			resultTy:   resultTy,
			tempPrefix: "deref",
		},
		env,
	)
	return tmp, resultTy, prelude + helperPrelude
}

// emitFormalTypeAssertExpr lowers `x.(T)` through the current typed helper path.
func emitFormalTypeAssertExpr(expr *ast.TypeAssertExpr, hintedTy string, env *formalEnv) (string, string, string) {
	value, valueTy, prelude := emitFormalExpr(expr.X, "", env)
	resultTy := normalizeFormalType(hintedTy)
	if expr.Type != nil {
		resultTy = formalTypeExprToMLIR(expr.Type, env.module)
	} else if isFormalOpaquePlaceholderType(resultTy) {
		resultTy = formalOpaqueType("value")
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalRuntimeTypeAssertSymbol(valueTy, resultTy).String(),
			args:       []string{value},
			argTys:     []string{valueTy},
			resultTy:   resultTy,
			tempPrefix: "typeassert",
		},
		env,
	)
	return tmp, resultTy, prelude + helperPrelude
}

func formalBuiltinType(name string) (string, bool) {
	return formalBuiltinTypeWithModule(name, nil)
}

func formalBuiltinTypeWithModule(name string, module *formalModuleContext) (string, bool) {
	targetIntTy := formalTargetIntType(module)
	switch name {
	case "bool":
		return "i1", true
	case "byte", "int8", "uint8":
		return "i8", true
	case "int16", "uint16":
		return "i16", true
	case "int", "int32", "rune", "uint", "uint32":
		return targetIntTy, true
	case "int64", "uint64":
		return "i64", true
	case "uintptr":
		return targetIntTy, true
	case "float32":
		return "f32", true
	case "float64":
		return "f64", true
	case "string":
		return "!go.string", true
	case "error":
		return "!go.error", true
	case "any", "interface{}":
		return formalOpaqueType("any"), true
	default:
		return "", false
	}
}

// emitFormalCoerceValue converts a computed value to the requested target type when possible.
func emitFormalCoerceValue(value string, valueTy string, targetTy string, env *formalEnv) (string, string, string, bool) {
	valueTy = normalizeFormalType(valueTy)
	targetTy = normalizeFormalType(targetTy)
	if targetTy == formalOpaqueType("value") || targetTy == formalOpaqueType("result") {
		return value, valueTy, "", true
	}
	if valueTy == targetTy {
		return value, targetTy, "", true
	}
	if isFormalIntegerType(valueTy) && isFormalIntegerType(targetTy) {
		return emitFormalIntegerCast(value, valueTy, targetTy, env)
	}
	if isFormalIntegerType(valueTy) && isFormalFloatType(targetTy) {
		return emitFormalIntToFloatCast(value, valueTy, targetTy, env)
	}
	if isFormalFloatType(valueTy) && isFormalIntegerType(targetTy) {
		return emitFormalFloatToIntegerCast(value, valueTy, targetTy, env)
	}
	if isFormalFloatType(valueTy) && isFormalFloatType(targetTy) {
		return emitFormalFloatCast(value, valueTy, targetTy, env)
	}
	if isFormalOpaquePlaceholderType(targetTy) {
		return "", "", "", false
	}
	tmp, prelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalRuntimeConvertSymbol(valueTy, targetTy).String(),
			args:       []string{value},
			argTys:     []string{valueTy},
			resultTy:   targetTy,
			tempPrefix: "conv",
		},
		env,
	)
	return tmp, targetTy, prelude, true
}

func emitFormalIntegerCast(value string, valueTy string, targetTy string, env *formalEnv) (string, string, string, bool) {
	valueTy = normalizeFormalType(valueTy)
	targetTy = normalizeFormalType(targetTy)
	if valueTy == targetTy {
		return value, targetTy, "", true
	}

	tmp := env.temp("conv")
	if valueTy == "index" || targetTy == "index" {
		return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.index_cast %s : %s to %s", tmp, value, valueTy, targetTy), true
	}

	valueWidth := formalIntegerWidth(valueTy)
	targetWidth := formalIntegerWidth(targetTy)
	if valueWidth == 0 || targetWidth == 0 {
		return "", "", "", false
	}
	if valueWidth < targetWidth {
		return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.extsi %s : %s to %s", tmp, value, valueTy, targetTy), true
	}
	return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.trunci %s : %s to %s", tmp, value, valueTy, targetTy), true
}

func formalIntegerWidth(ty string) int {
	switch normalizeFormalType(ty) {
	case "i8":
		return 8
	case "i16":
		return 16
	case "i32":
		return 32
	case "i64":
		return 64
	case "index":
		return 64
	default:
		return 0
	}
}

func isFormalFloatType(ty string) bool {
	switch normalizeFormalType(ty) {
	case "f32", "f64":
		return true
	default:
		return false
	}
}

func formalFloatWidth(ty string) int {
	switch normalizeFormalType(ty) {
	case "f32":
		return 32
	case "f64":
		return 64
	default:
		return 0
	}
}

func emitFormalIntToFloatCast(value string, valueTy string, targetTy string, env *formalEnv) (string, string, string, bool) {
	tmp := env.temp("conv")
	return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.sitofp %s : %s to %s", tmp, value, valueTy, targetTy), true
}

func emitFormalFloatToIntegerCast(value string, valueTy string, targetTy string, env *formalEnv) (string, string, string, bool) {
	tmp := env.temp("conv")
	return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.fptosi %s : %s to %s", tmp, value, valueTy, targetTy), true
}

func emitFormalFloatCast(value string, valueTy string, targetTy string, env *formalEnv) (string, string, string, bool) {
	valueTy = normalizeFormalType(valueTy)
	targetTy = normalizeFormalType(targetTy)
	if valueTy == targetTy {
		return value, targetTy, "", true
	}
	valueWidth := formalFloatWidth(valueTy)
	targetWidth := formalFloatWidth(targetTy)
	if valueWidth == 0 || targetWidth == 0 {
		return "", "", "", false
	}
	tmp := env.temp("conv")
	if valueWidth < targetWidth {
		return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.extf %s : %s to %s", tmp, value, valueTy, targetTy), true
	}
	return tmp, targetTy, emitFormalLinef(nil, env, "    %s = arith.truncf %s : %s to %s", tmp, value, valueTy, targetTy), true
}

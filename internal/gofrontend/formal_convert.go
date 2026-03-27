package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

// emitFormalStarExpr lowers `*ptr` reads through the current typed helper path.
func emitFormalStarExpr(expr *ast.StarExpr, hintedTy string, env *formalEnv) (string, string, string) {
	ptr, ptrTy, prelude := emitFormalExpr(expr.X, "", env)
	resultTy := normalizeFormalType(hintedTy)
	if isFormalOpaquePlaceholderType(resultTy) {
		resultTy = formalDerefType(ptrTy)
	}
	tmp, loadedTy, loadPrelude, ok := emitFormalLoad(ptr, ptrTy, resultTy, env)
	if ok {
		return tmp, loadedTy, prelude + loadPrelude
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       "__mlse_deref_" + sanitizeName(ptrTy),
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
			base:       "__mlse_type_assert__" + sanitizeName(valueTy) + "__to__" + sanitizeName(resultTy),
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
	switch name {
	case "bool":
		return "i1", true
	case "byte", "int8", "uint8":
		return "i8", true
	case "int16", "uint16":
		return "i16", true
	case "int", "int32", "rune", "uint", "uint32":
		return "i32", true
	case "int64", "uint64", "uintptr":
		return "i64", true
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
	if isFormalOpaquePlaceholderType(targetTy) {
		return "", "", "", false
	}
	tmp, prelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       "__mlse_convert_" + sanitizeName(valueTy) + "__to__" + sanitizeName(targetTy),
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
		return tmp, targetTy, fmt.Sprintf("    %s = arith.index_cast %s : %s to %s\n", tmp, value, valueTy, targetTy), true
	}

	valueWidth := formalIntegerWidth(valueTy)
	targetWidth := formalIntegerWidth(targetTy)
	if valueWidth == 0 || targetWidth == 0 {
		return "", "", "", false
	}
	if valueWidth < targetWidth {
		return tmp, targetTy, fmt.Sprintf("    %s = arith.extsi %s : %s to %s\n", tmp, value, valueTy, targetTy), true
	}
	return tmp, targetTy, fmt.Sprintf("    %s = arith.trunci %s : %s to %s\n", tmp, value, valueTy, targetTy), true
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

func emitFormalHelperMakeCall(args []ast.Expr, targetTy string, env *formalEnv) (string, string, string) {
	argValues := make([]string, 0, len(args))
	argTypes := make([]string, 0, len(args))
	var buf strings.Builder
	for _, arg := range args {
		value, ty, prelude := emitFormalExpr(arg, "i32", env)
		buf.WriteString(prelude)
		argValues = append(argValues, value)
		argTypes = append(argTypes, ty)
	}
	tmp, prelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       "__mlse_make_" + sanitizeName(targetTy),
			args:       argValues,
			argTys:     argTypes,
			resultTy:   targetTy,
			tempPrefix: "make",
		},
		env,
	)
	return tmp, targetTy, buf.String() + prelude
}

package gofrontend

import (
	"go/ast"
	"strings"
)

// emitFormalCompositeLitExpr lowers the currently supported composite-literal subset.
func emitFormalCompositeLitExpr(lit *ast.CompositeLit, hintedTy string, env *formalEnv) (string, string, string) {
	targetTy := formalTypeExprToMLIR(lit.Type, env.module)
	if lit.Type == nil || targetTy == formalOpaqueType("type") || targetTy == formalOpaqueType("unit") {
		targetTy = normalizeFormalType(hintedTy)
	}
	if isFormalOpaquePlaceholderType(targetTy) {
		targetTy = formalOpaqueType("value")
	}
	if isFormalSliceType(targetTy) && len(lit.Elts) == 0 {
		zero := env.temp("const")
		tmp := env.temp("make")
		intTy := formalTargetIntType(env.module)
		return tmp, targetTy,
			emitFormalLinef(lit, env, "    %s = arith.constant 0 : %s", zero, intTy) +
				emitFormalLinef(lit, env, "    %s = go.make_slice %s, %s : %s to %s", tmp, zero, zero, intTy, targetTy)
	}

	elementHint := formalCompositeElementHint(targetTy)
	args := make([]string, 0, len(lit.Elts))
	argTys := make([]string, 0, len(lit.Elts))
	var buf strings.Builder
	for _, elt := range lit.Elts {
		valueExpr := elt
		if kv, ok := elt.(*ast.KeyValueExpr); ok {
			valueExpr = kv.Value
		}
		value, ty, prelude := emitFormalExpr(valueExpr, elementHint, env)
		buf.WriteString(prelude)
		args = append(args, value)
		argTys = append(argTys, ty)
	}

	tmp, helperPrelude := emitFormalRuntimeCompositeHelper(lit, targetTy, args, argTys, env)
	return tmp, targetTy, buf.String() + helperPrelude
}

func emitFormalCompositeAddrExpr(lit *ast.CompositeLit, hintedTy string, env *formalEnv) (string, string, string) {
	resultTy := normalizeFormalType(hintedTy)
	if !isFormalPointerType(resultTy) {
		baseTy := formalTypeExprToMLIR(lit.Type, env.module)
		if isFormalOpaquePlaceholderType(baseTy) || baseTy == formalOpaqueType("type") || baseTy == formalOpaqueType("unit") {
			baseTy = formalOpaqueType("value")
		}
		resultTy = formalPointerType(baseTy)
	}

	tmp, prelude, ok := emitFormalStaticAllocForComposite(lit, resultTy, env)
	if !ok {
		tmp, prelude = emitFormalRuntimeTypedNew(resultTy, env)
	}
	initPrelude, ok := emitFormalCompositeAddrInit(lit, tmp, resultTy, env)
	if !ok {
		return tmp, resultTy, prelude
	}
	return tmp, resultTy, prelude + initPrelude
}

func emitFormalStaticAllocForComposite(lit *ast.CompositeLit, resultTy string, env *formalEnv) (string, string, bool) {
	size, align, ok := formalCompositeStaticSizeAlign(lit, env.module)
	if !ok {
		return "", "", false
	}
	return emitFormalStaticAlloc(formalStaticAllocSpec{
		resultTy: resultTy,
		size:     size,
		align:    align,
	}, env)
}

func formalCompositeElementHint(targetTy string) string {
	targetTy = normalizeFormalType(targetTy)
	if strings.HasPrefix(targetTy, "!go.slice<") && strings.HasSuffix(targetTy, ">") {
		return strings.TrimSuffix(strings.TrimPrefix(targetTy, "!go.slice<"), ">")
	}
	return ""
}

func emitFormalCompositeAddrInit(lit *ast.CompositeLit, addr string, addrTy string, env *formalEnv) (string, bool) {
	if lit == nil || len(lit.Elts) == 0 {
		return "", true
	}

	var buf strings.Builder
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			return "", false
		}
		fieldName := formalCompositeFieldName(kv.Key)
		if fieldName == "" {
			return "", false
		}

		fieldTy := formalCompositeFieldType(lit, fieldName, env.module)
		value, valueTy, valuePrelude := emitFormalExpr(kv.Value, fieldTy, env)
		buf.WriteString(valuePrelude)
		if fieldTy == "" || isFormalOpaquePlaceholderType(fieldTy) {
			fieldTy = valueTy
		}
		if normalizeFormalType(valueTy) != normalizeFormalType(fieldTy) {
			coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(value, valueTy, fieldTy, env)
			if !ok {
				return "", false
			}
			buf.WriteString(coercedPrelude)
			value = coercedValue
			valueTy = coercedTy
		}
		fieldOffset, hasFieldOffset := formalCompositeFieldOffset(lit, fieldName, env.module)
		fieldAddr, fieldAddrTy, fieldPrelude, ok := emitFormalFieldAddr(formalFieldAddrSpec{
			base:      addr,
			baseTy:    addrTy,
			field:     fieldName,
			fieldTy:   fieldTy,
			offset:    fieldOffset,
			hasOffset: hasFieldOffset,
		}, env)
		if !ok {
			return "", false
		}
		buf.WriteString(fieldPrelude)
		storePrelude, ok := emitFormalStore(value, valueTy, fieldAddr, fieldAddrTy, env)
		if !ok {
			return "", false
		}
		buf.WriteString(storePrelude)
	}
	return buf.String(), true
}

func formalCompositeFieldName(expr ast.Expr) string {
	switch key := expr.(type) {
	case *ast.Ident:
		return key.Name
	case *ast.SelectorExpr:
		return key.Sel.Name
	default:
		return ""
	}
}

func formalTypeHelperSuffix(ty string) string {
	ty = normalizeFormalType(ty)
	switch {
	case ty == "i1":
		return "bool"
	case ty == "i8" || ty == "i16" || ty == "i32" || ty == "i64" || ty == "index":
		return ty
	case ty == "!go.string":
		return "string"
	case ty == "!go.error":
		return "error"
	case isFormalPointerType(ty):
		return "ptr." + formalTypeHelperSuffix(formalDerefType(ty))
	case isFormalSliceType(ty):
		return "slice." + formalTypeHelperSuffix(formalIndexResultType(ty))
	case isFormalNamedType(ty):
		return formalNamedTypeHelperSuffix(ty)
	default:
		return sanitizeName(ty)
	}
}

func formalNamedTypeHelperSuffix(ty string) string {
	ty = normalizeFormalType(ty)
	if !strings.HasPrefix(ty, "!go.named<\"") || !strings.HasSuffix(ty, "\">") {
		return sanitizeName(ty)
	}
	return sanitizeName(strings.TrimSuffix(strings.TrimPrefix(ty, "!go.named<\""), "\">"))
}

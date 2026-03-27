package gofrontend

import (
	"fmt"
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
		return tmp, targetTy, fmt.Sprintf(
			"    %s = arith.constant 0 : i32\n    %s = go.make_slice %s, %s : i32 to %s\n",
			zero, tmp, zero, zero, targetTy,
		)
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

	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalCompositeHelperBase(lit, targetTy),
			args:       args,
			argTys:     argTys,
			resultTy:   targetTy,
			tempPrefix: "composite",
		},
		env,
	)
	return tmp, targetTy, buf.String() + helperPrelude
}

func formalCompositeElementHint(targetTy string) string {
	targetTy = normalizeFormalType(targetTy)
	if strings.HasPrefix(targetTy, "!go.slice<") && strings.HasSuffix(targetTy, ">") {
		return strings.TrimSuffix(strings.TrimPrefix(targetTy, "!go.slice<"), ">")
	}
	return ""
}

func formalCompositeHelperBase(lit *ast.CompositeLit, targetTy string) string {
	base := "__mlse_composite_" + sanitizeName(targetTy)
	if lit == nil {
		return base
	}
	keys := make([]string, 0, len(lit.Elts))
	for _, elt := range lit.Elts {
		kv, ok := elt.(*ast.KeyValueExpr)
		if !ok {
			continue
		}
		switch key := kv.Key.(type) {
		case *ast.Ident:
			keys = append(keys, sanitizeName(key.Name))
		case *ast.SelectorExpr:
			keys = append(keys, sanitizeName(renderSelector(key)))
		}
	}
	if len(keys) == 0 {
		return base
	}
	return base + "__" + strings.Join(keys, "__")
}

package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

type formalElemAddrSpec struct {
	base    string
	baseTy  string
	index   string
	indexTy string
	elemTy  string
}

type formalFieldAddrSpec struct {
	base      string
	baseTy    string
	field     string
	fieldTy   string
	offset    int64
	hasOffset bool
}

func formalPointerType(pointee string) string {
	return "!go.ptr<" + normalizeFormalType(pointee) + ">"
}

func emitFormalFieldAddr(spec formalFieldAddrSpec, env *formalEnv) (string, string, string, bool) {
	spec.baseTy = normalizeFormalType(spec.baseTy)
	spec.fieldTy = normalizeFormalType(spec.fieldTy)
	if spec.field == "" || (!isFormalPointerType(spec.baseTy) && !isFormalNamedType(spec.baseTy)) {
		return "", "", "", false
	}
	addrTy := formalPointerType(spec.fieldTy)
	tmp := env.temp("field")
	attrText := ""
	if spec.hasOffset {
		attrText = fmt.Sprintf(" {offset = %d : i64}", spec.offset)
	}
	return tmp, addrTy, emitFormalLinef(nil, env, "    %s = go.field_addr %s, %q%s : %s -> %s",
		tmp, spec.base, spec.field, attrText, spec.baseTy, addrTy,
	), true
}

func emitFormalElemAddr(spec formalElemAddrSpec, env *formalEnv) (string, string, string, bool) {
	spec.baseTy = normalizeFormalType(spec.baseTy)
	spec.indexTy = normalizeFormalType(spec.indexTy)
	spec.elemTy = normalizeFormalType(spec.elemTy)
	if !isFormalSliceType(spec.baseTy) {
		return "", "", "", false
	}
	if isFormalOpaquePlaceholderType(spec.elemTy) {
		spec.elemTy = formalIndexResultType(spec.baseTy)
	}
	addrTy := formalPointerType(spec.elemTy)
	tmp := env.temp("elem")
	return tmp, addrTy, emitFormalLinef(nil, env, "    %s = go.elem_addr %s, %s : (%s, %s) -> %s",
		tmp, spec.base, spec.index, spec.baseTy, spec.indexTy, addrTy,
	), true
}

func emitFormalLoad(addr string, addrTy string, _ string, env *formalEnv) (string, string, string, bool) {
	addrTy = normalizeFormalType(addrTy)
	if !isFormalPointerType(addrTy) {
		return "", "", "", false
	}
	resultTy := formalDerefType(addrTy)
	tmp := env.temp("load")
	return tmp, resultTy, emitFormalLinef(nil, env, "    %s = go.load %s : %s -> %s",
		tmp, addr, addrTy, resultTy,
	), true
}

func emitFormalStore(value string, valueTy string, addr string, addrTy string, env *formalEnv) (string, bool) {
	valueTy = normalizeFormalType(valueTy)
	addrTy = normalizeFormalType(addrTy)
	if !isFormalPointerType(addrTy) || formalDerefType(addrTy) != valueTy {
		return "", false
	}
	return emitFormalLinef(nil, env, "    go.store %s, %s : %s to %s", value, addr, valueTy, addrTy), true
}

func formalSelectorResultType(expr *ast.SelectorExpr, hintedTy string, env *formalEnv) string {
	ty := normalizeFormalType(hintedTy)
	if !isFormalOpaquePlaceholderType(ty) {
		return ty
	}
	ty = inferFormalExprType(expr, env)
	if isFormalOpaquePlaceholderType(ty) {
		return formalOpaqueType("value")
	}
	return ty
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
		return ""
	}
	return ""
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
			base:       formalRuntimeIndexSymbol(sourceTy).String(),
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
	if value, ty, prelude, ok := emitFormalTypedConstExpr(expr, hintedTy, env); ok {
		return value, ty, prelude
	}
	ty := formalSelectorResultType(expr, hintedTy, env)
	if sig, ok := parseFormalFuncType(ty); ok {
		symbol := formalCallSymbol(expr, sig.params, sig.results, env.module)
		if symbol == "" {
			return emitFormalTodoValue("SelectorExpr", ty, env)
		}
		tmp := env.temp("sel")
		return tmp, ty, emitFormalLinef(expr, env, "    %s = func.constant @%s : %s", tmp, symbol, ty)
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
	fieldOffset, hasFieldOffset := formalSelectorFieldOffset(expr, env.module)
	fieldAddr, fieldAddrTy, fieldAddrPrelude, ok := emitFormalFieldAddr(formalFieldAddrSpec{
		base:      base,
		baseTy:    baseTy,
		field:     expr.Sel.Name,
		fieldTy:   ty,
		offset:    fieldOffset,
		hasOffset: hasFieldOffset,
	}, env)
	if ok {
		tmp, loadedTy, loadPrelude, loadOK := emitFormalLoad(fieldAddr, fieldAddrTy, ty, env)
		if loadOK {
			coercedValue, coercedTy, coercedPrelude := coerceFormalValueToHint(tmp, loadedTy, hintedTy, env)
			return coercedValue, coercedTy, basePrelude + fieldAddrPrelude + loadPrelude + coercedPrelude
		}
	}
	tmp, helperPrelude := emitFormalHelperCall(
		formalHelperCallSpec{
			base:       formalRuntimeSelectorSymbol(expr.Sel.Name).String(),
			args:       []string{base},
			argTys:     []string{baseTy},
			resultTy:   ty,
			tempPrefix: "sel",
		},
		env,
	)
	return tmp, ty, basePrelude + helperPrelude
}

func isFormalPointerType(ty string) bool {
	return len(ty) > 0 && len(ty) >= len("!go.ptr<>") && ty[:8] == "!go.ptr<"
}

func isFormalNamedType(ty string) bool {
	return len(ty) > 0 && len(ty) >= len("!go.named<>") && ty[:10] == "!go.named<"
}

func isFormalStringType(ty string) bool {
	return normalizeFormalType(ty) == "!go.string"
}

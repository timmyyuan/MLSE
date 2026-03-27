package gofrontend

import (
	"fmt"
	"go/ast"
)

type formalElemAddrSpec struct {
	base    string
	baseTy  string
	index   string
	indexTy string
	elemTy  string
}

func formalPointerType(pointee string) string {
	return "!go.ptr<" + normalizeFormalType(pointee) + ">"
}

func emitFormalFieldAddr(base string, baseTy string, field string, fieldTy string, env *formalEnv) (string, string, string, bool) {
	baseTy = normalizeFormalType(baseTy)
	fieldTy = normalizeFormalType(fieldTy)
	if field == "" || (!isFormalPointerType(baseTy) && !isFormalNamedType(baseTy)) {
		return "", "", "", false
	}
	addrTy := formalPointerType(fieldTy)
	tmp := env.temp("field")
	return tmp, addrTy, fmt.Sprintf(
		"    %s = go.field_addr %s, %q : %s -> %s\n",
		tmp, base, field, baseTy, addrTy,
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
	return tmp, addrTy, fmt.Sprintf(
		"    %s = go.elem_addr %s, %s : (%s, %s) -> %s\n",
		tmp, spec.base, spec.index, spec.baseTy, spec.indexTy, addrTy,
	), true
}

func emitFormalLoad(addr string, addrTy string, resultTy string, env *formalEnv) (string, string, string, bool) {
	addrTy = normalizeFormalType(addrTy)
	if !isFormalPointerType(addrTy) {
		return "", "", "", false
	}
	resultTy = normalizeFormalType(resultTy)
	if isFormalOpaquePlaceholderType(resultTy) {
		resultTy = formalDerefType(addrTy)
	}
	tmp := env.temp("load")
	return tmp, resultTy, fmt.Sprintf(
		"    %s = go.load %s : %s -> %s\n",
		tmp, addr, addrTy, resultTy,
	), true
}

func emitFormalStore(value string, valueTy string, addr string, addrTy string) (string, bool) {
	valueTy = normalizeFormalType(valueTy)
	addrTy = normalizeFormalType(addrTy)
	if !isFormalPointerType(addrTy) || formalDerefType(addrTy) != valueTy {
		return "", false
	}
	return fmt.Sprintf("    go.store %s, %s : %s to %s\n", value, addr, valueTy, addrTy), true
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

func isFormalPointerType(ty string) bool {
	return len(ty) > 0 && len(ty) >= len("!go.ptr<>") && ty[:8] == "!go.ptr<"
}

func isFormalNamedType(ty string) bool {
	return len(ty) > 0 && len(ty) >= len("!go.named<>") && ty[:10] == "!go.named<"
}

func isFormalStringType(ty string) bool {
	return normalizeFormalType(ty) == "!go.string"
}

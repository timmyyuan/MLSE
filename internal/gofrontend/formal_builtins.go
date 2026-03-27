package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// emitFormalBuiltinCall handles builtin calls that already have a dedicated `go.*` op.
func emitFormalBuiltinCall(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	switch formalBuiltinCallName(call) {
	case "len":
		return emitFormalLenCapBuiltinCall("len", call, hintedTy, env)
	case "cap":
		return emitFormalLenCapBuiltinCall("cap", call, hintedTy, env)
	case "append":
		return emitFormalAppendBuiltinCall(call, hintedTy, env)
	default:
		return "", "", "", false
	}
}

func formalBuiltinCallName(call *ast.CallExpr) string {
	ident, ok := call.Fun.(*ast.Ident)
	if !ok {
		return ""
	}
	return ident.Name
}

func emitFormalLenCapBuiltinCall(opName string, call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	if len(call.Args) != 1 {
		return "", "", "", false
	}
	value, valueTy, prelude := emitFormalExpr(call.Args[0], "", env)
	if opName == "len" && !isFormalLenLikeType(valueTy) {
		return "", "", "", false
	}
	if opName == "cap" && !isFormalSliceType(valueTy) {
		return "", "", "", false
	}
	resultTy := inferFormalCallResultType(call, hintedTy, env)
	tmp := env.temp(opName)
	return tmp, resultTy, prelude + fmt.Sprintf("    %s = go.%s %s : %s -> %s\n", tmp, opName, value, valueTy, resultTy), true
}

func emitFormalAppendBuiltinCall(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	if len(call.Args) < 2 {
		return "", "", "", false
	}
	if call.Ellipsis != token.NoPos {
		return emitFormalAppendSliceBuiltinCall(call, hintedTy, env)
	}
	sliceValue, sliceTy, slicePrelude := emitFormalExpr(call.Args[0], "", env)
	if !isFormalSliceType(sliceTy) {
		return "", "", "", false
	}
	values, valueTys, valuePrelude := emitFormalCallOperands(call.Args[1:], env)
	elementTy := formalIndexResultType(sliceTy)
	for _, valueTy := range valueTys {
		if valueTy != elementTy {
			return "", "", "", false
		}
	}
	resultTy := inferFormalCallResultType(call, hintedTy, env)
	operands := append([]string{sliceValue}, values...)
	operandTys := append([]string{sliceTy}, valueTys...)
	tmp := env.temp("append")
	return tmp, resultTy, slicePrelude + valuePrelude + fmt.Sprintf(
		"    %s = go.append %s : (%s) -> %s\n",
		tmp,
		strings.Join(operands, ", "),
		strings.Join(operandTys, ", "),
		resultTy,
	), true
}

func emitFormalAppendSliceBuiltinCall(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	if len(call.Args) != 2 || call.Ellipsis == token.NoPos {
		return "", "", "", false
	}
	dstValue, dstTy, dstPrelude := emitFormalExpr(call.Args[0], "", env)
	srcValue, srcTy, srcPrelude := emitFormalExpr(call.Args[1], "", env)
	if !isFormalSliceType(dstTy) || !isFormalSliceType(srcTy) {
		return "", "", "", false
	}
	if formalIndexResultType(dstTy) != formalIndexResultType(srcTy) {
		return "", "", "", false
	}
	resultTy := inferFormalCallResultType(call, hintedTy, env)
	tmp := env.temp("append_slice")
	return tmp, resultTy, dstPrelude + srcPrelude + fmt.Sprintf(
		"    %s = go.append_slice %s, %s : (%s, %s) -> %s\n",
		tmp,
		dstValue,
		srcValue,
		dstTy,
		srcTy,
		resultTy,
	), true
}

func emitFormalGoLenValue(source string, sourceTy string, resultTy string, tempPrefix string, env *formalEnv) (string, string, bool) {
	if !isFormalLenLikeType(sourceTy) {
		return "", "", false
	}
	tmp := env.temp(tempPrefix)
	return tmp, fmt.Sprintf("    %s = go.len %s : %s -> %s\n", tmp, source, sourceTy, resultTy), true
}

type formalGoIndexSpec struct {
	source     string
	sourceTy   string
	index      string
	indexTy    string
	hintedTy   string
	tempPrefix string
}

func emitFormalGoIndexValue(spec formalGoIndexSpec, env *formalEnv) (string, string, string, bool) {
	if !isFormalStringType(spec.sourceTy) {
		return "", "", "", false
	}
	resultTy := normalizeFormalType(spec.hintedTy)
	if isFormalOpaquePlaceholderType(resultTy) {
		resultTy = formalIndexResultType(spec.sourceTy)
	}
	tmp := env.temp(spec.tempPrefix)
	return tmp, resultTy, fmt.Sprintf("    %s = go.index %s, %s : (%s, %s) -> %s\n", tmp, spec.source, spec.index, spec.sourceTy, spec.indexTy, resultTy), true
}

func emitFormalIndexedReadValue(spec formalGoIndexSpec, env *formalEnv) (string, string, string, bool) {
	if isFormalSliceType(spec.sourceTy) {
		resultTy := normalizeFormalType(spec.hintedTy)
		if isFormalOpaquePlaceholderType(resultTy) {
			resultTy = formalIndexResultType(spec.sourceTy)
		}
		addr, addrTy, addrPrelude, ok := emitFormalElemAddr(formalElemAddrSpec{
			base:    spec.source,
			baseTy:  spec.sourceTy,
			index:   spec.index,
			indexTy: spec.indexTy,
			elemTy:  resultTy,
		}, env)
		if !ok {
			return "", "", "", false
		}
		tmp, loadedTy, loadPrelude, loadOK := emitFormalLoad(addr, addrTy, resultTy, env)
		if !loadOK {
			return "", "", "", false
		}
		return tmp, loadedTy, addrPrelude + loadPrelude, true
	}
	return emitFormalGoIndexValue(spec, env)
}

func isFormalLenLikeType(ty string) bool {
	ty = normalizeFormalType(ty)
	return ty == "!go.string" || isFormalSliceType(ty)
}

func isFormalIndexLikeType(ty string) bool {
	return isFormalLenLikeType(ty)
}

func isFormalSliceType(ty string) bool {
	return strings.HasPrefix(normalizeFormalType(ty), "!go.slice<")
}

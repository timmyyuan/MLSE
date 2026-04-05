package gofrontend

import (
	"fmt"
	"go/ast"
	"strings"
)

type formalRuntimeCallSpec struct {
	symbol     formalRuntimeSymbol
	args       []string
	argTys     []string
	resultTy   string
	tempPrefix string
}

func formalAnyHandleType() string {
	return formalOpaqueType("any")
}

func formalAnySliceType() string {
	return "!go.slice<" + formalAnyHandleType() + ">"
}

func isFormalAnyHandleType(ty string) bool {
	return normalizeFormalType(ty) == formalAnyHandleType()
}

func emitFormalRuntimeCall(spec formalRuntimeCallSpec, env *formalEnv) (string, string) {
	return emitFormalHelperCall(
		formalHelperCallSpec{
			base:       spec.symbol.String(),
			args:       spec.args,
			argTys:     spec.argTys,
			resultTy:   spec.resultTy,
			tempPrefix: spec.tempPrefix,
		},
		env,
	)
}

func emitFormalRuntimeVoidCall(symbol formalRuntimeSymbol, args []string, argTys []string, env *formalEnv) string {
	callee := registerFormalExtern(env.module, symbol.String(), argTys, nil)
	return emitFormalLinef(nil, env, "    func.call @%s(%s) : (%s) -> ()", callee, strings.Join(args, ", "), strings.Join(argTys, ", "))
}

func emitFormalRuntimeNewObject(resultTy string, size int64, align int64, env *formalEnv) (string, string, bool) {
	resultTy = normalizeFormalType(resultTy)
	if !isFormalPointerType(resultTy) || size < 0 || align <= 0 {
		return "", "", false
	}
	sizeConst := env.temp("size")
	alignConst := env.temp("align")
	tmp, callPrelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     formalRuntimeSymbolNewObject,
			args:       []string{sizeConst, alignConst},
			argTys:     []string{"i64", "i64"},
			resultTy:   resultTy,
			tempPrefix: "new",
		},
		env,
	)
	prelude := fmt.Sprintf(
		"%s%s",
		emitFormalLinef(nil, env, "    %s = arith.constant %d : i64", sizeConst, size),
		emitFormalLinef(nil, env, "    %s = arith.constant %d : i64", alignConst, align),
	)
	return tmp, prelude + callPrelude, true
}

func emitFormalRuntimeMakeHelper(args []ast.Expr, targetTy string, env *formalEnv) (string, string, string) {
	argValues := make([]string, 0, len(args))
	argTypes := make([]string, 0, len(args))
	var buf strings.Builder
	argHint := formalTargetIntType(env.module)
	for _, arg := range args {
		value, ty, prelude := emitFormalExpr(arg, argHint, env)
		buf.WriteString(prelude)
		argValues = append(argValues, value)
		argTypes = append(argTypes, ty)
	}
	tmp, prelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     formalRuntimeMakeHelperSymbol(targetTy),
			args:       argValues,
			argTys:     argTypes,
			resultTy:   targetTy,
			tempPrefix: "make",
		},
		env,
	)
	return tmp, targetTy, buf.String() + prelude
}

func emitFormalRuntimeTypedNew(resultTy string, env *formalEnv) (string, string) {
	return emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     formalRuntimeNewHelperSymbol(resultTy),
			resultTy:   resultTy,
			tempPrefix: "new",
		},
		env,
	)
}

func emitFormalRuntimeCompositeHelper(lit *ast.CompositeLit, targetTy string, args []string, argTys []string, env *formalEnv) (string, string) {
	return emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     formalRuntimeCompositeHelperSymbol(lit, targetTy),
			args:       args,
			argTys:     argTys,
			resultTy:   targetTy,
			tempPrefix: "composite",
		},
		env,
	)
}

func emitFormalRuntimeBoxAny(value string, valueTy string, env *formalEnv) (string, string, bool) {
	valueTy = normalizeFormalType(valueTy)
	if valueTy == "" {
		valueTy = formalOpaqueType("value")
	}
	if isFormalAnyHandleType(valueTy) {
		return value, "", true
	}
	tmp, prelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     formalRuntimeAnyBoxSymbol(valueTy),
			args:       []string{value},
			argTys:     []string{valueTy},
			resultTy:   formalAnyHandleType(),
			tempPrefix: "any",
		},
		env,
	)
	return tmp, prelude, true
}

func emitFormalRuntimePackAnyArgs(args []ast.Expr, env *formalEnv) (string, string, string, bool) {
	sliceTy := formalAnySliceType()
	intTy := formalTargetIntType(env.module)
	count := env.temp("argc")
	packed := env.temp("args")
	var buf strings.Builder
	buf.WriteString(emitFormalLinef(nil, env, "    %s = arith.constant %d : %s", count, len(args), intTy))
	buf.WriteString(emitFormalLinef(nil, env, "    %s = go.make_slice %s, %s : %s to %s", packed, count, count, intTy, sliceTy))
	for i, arg := range args {
		value, valueTy, valuePrelude := emitFormalExpr(arg, "", env)
		buf.WriteString(valuePrelude)
		boxed, boxPrelude, ok := emitFormalRuntimeBoxAny(value, valueTy, env)
		if !ok {
			return "", "", "", false
		}
		buf.WriteString(boxPrelude)
		index := env.temp("argi")
		buf.WriteString(emitFormalLinef(nil, env, "    %s = arith.constant %d : %s", index, i, intTy))
		slot, slotTy, slotPrelude, ok := emitFormalElemAddr(
			formalElemAddrSpec{
				base:    packed,
				baseTy:  sliceTy,
				index:   index,
				indexTy: intTy,
				elemTy:  formalAnyHandleType(),
			},
			env,
		)
		if !ok {
			return "", "", "", false
		}
		buf.WriteString(slotPrelude)
		storePrelude, ok := emitFormalStore(boxed, formalAnyHandleType(), slot, slotTy, env)
		if !ok {
			return "", "", "", false
		}
		buf.WriteString(storePrelude)
	}
	return packed, sliceTy, buf.String(), true
}

func emitFormalRuntimeFormatCall(call *ast.CallExpr, resultTy string, symbol formalRuntimeSymbol, env *formalEnv) (string, string, string, bool) {
	if len(call.Args) == 0 {
		return "", "", "", false
	}
	format, formatTy, formatPrelude := emitFormalExpr(call.Args[0], "!go.string", env)
	if normalizeFormalType(formatTy) != "!go.string" {
		if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(format, formatTy, "!go.string", env); ok {
			format = coercedValue
			formatTy = coercedTy
			formatPrelude += coercedPrelude
		} else {
			return "", "", "", false
		}
	}
	argsValue, argsTy, argsPrelude, ok := emitFormalRuntimePackAnyArgs(call.Args[1:], env)
	if !ok {
		return "", "", "", false
	}
	tmp, callPrelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     symbol,
			args:       []string{format, argsValue},
			argTys:     []string{formatTy, argsTy},
			resultTy:   resultTy,
			tempPrefix: "call",
		},
		env,
	)
	return tmp, resultTy, formatPrelude + argsPrelude + callPrelude, true
}

func emitFormalRuntimeAnySliceCall(args []ast.Expr, resultTy string, symbol formalRuntimeSymbol, env *formalEnv) (string, string, string, bool) {
	argsValue, argsTy, argsPrelude, ok := emitFormalRuntimePackAnyArgs(args, env)
	if !ok {
		return "", "", "", false
	}
	tmp, callPrelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     symbol,
			args:       []string{argsValue},
			argTys:     []string{argsTy},
			resultTy:   resultTy,
			tempPrefix: "call",
		},
		env,
	)
	return tmp, resultTy, argsPrelude + callPrelude, true
}

func emitFormalRuntimePrintCall(call *ast.CallExpr, symbol formalRuntimeSymbol, env *formalEnv) (string, bool) {
	argsValue, argsTy, argsPrelude, ok := emitFormalRuntimePackAnyArgs(call.Args, env)
	if !ok {
		return "", false
	}
	return argsPrelude + emitFormalRuntimeVoidCall(symbol, []string{argsValue}, []string{argsTy}, env), true
}

func emitFormalRuntimePrintfCall(call *ast.CallExpr, symbol formalRuntimeSymbol, env *formalEnv) (string, bool) {
	if len(call.Args) == 0 {
		return "", false
	}
	format, formatTy, formatPrelude := emitFormalExpr(call.Args[0], "!go.string", env)
	if normalizeFormalType(formatTy) != "!go.string" {
		if coercedValue, coercedTy, coercedPrelude, ok := emitFormalCoerceValue(format, formatTy, "!go.string", env); ok {
			format = coercedValue
			formatTy = coercedTy
			formatPrelude += coercedPrelude
		} else {
			return "", false
		}
	}
	argsValue, argsTy, argsPrelude, ok := emitFormalRuntimePackAnyArgs(call.Args[1:], env)
	if !ok {
		return "", false
	}
	return formatPrelude + argsPrelude + emitFormalRuntimeVoidCall(symbol, []string{format, argsValue}, []string{formatTy, argsTy}, env), true
}

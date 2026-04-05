package gofrontend

import (
	"go/ast"
	"strings"
)

// emitFormalMethodCallExpr lowers immediate method calls to the current `package.method` symbol convention.
func emitFormalMethodCallExpr(call *ast.CallExpr, hintedTy string, env *formalEnv, argHints []string) (string, string, string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || isFormalPackageSelector(selector, env) {
		return "", "", "", false
	}

	recv, recvTy, recvPrelude := emitFormalExpr(selector.X, "", env)
	args, argTys, argPrelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	resultTy := inferFormalCallResultType(call, hintedTy, env)
	symbol := formalMethodSymbol(selector, append([]string{recvTy}, argTys...), []string{resultTy}, env.module)
	tmp := env.temp("call")
	var buf strings.Builder
	buf.WriteString(recvPrelude)
	buf.WriteString(argPrelude)
	buf.WriteString(emitFormalLinef(
		call,
		env,
		"    %s = func.call @%s(%s) : (%s) -> %s",
		tmp,
		symbol,
		strings.Join(append([]string{recv}, args...), ", "),
		strings.Join(append([]string{recvTy}, argTys...), ", "),
		resultTy,
	))
	return tmp, resultTy, buf.String(), true
}

// emitFormalMethodCallStmt lowers statement-position method calls to the current `package.method` symbol convention.
func emitFormalMethodCallStmt(call *ast.CallExpr, env *formalEnv, argHints []string) (string, bool) {
	selector, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || isFormalPackageSelector(selector, env) {
		return "", false
	}

	recv, recvTy, recvPrelude := emitFormalExpr(selector.X, "", env)
	args, argTys, argPrelude := emitFormalCallOperandsWithHints(call.Args, argHints, env)
	symbol := formalMethodSymbol(selector, append([]string{recvTy}, argTys...), nil, env.module)
	var buf strings.Builder
	buf.WriteString(recvPrelude)
	buf.WriteString(argPrelude)
	buf.WriteString(emitFormalLinef(
		call,
		env,
		"    func.call @%s(%s) : (%s) -> ()",
		symbol,
		strings.Join(append([]string{recv}, args...), ", "),
		strings.Join(append([]string{recvTy}, argTys...), ", "),
	))
	return buf.String(), true
}

func formalMethodSymbol(selector *ast.SelectorExpr, params []string, results []string, module *formalModuleContext) string {
	name := ""
	if selector != nil && selector.Sel != nil {
		name = selector.Sel.Name
	}
	if module == nil {
		return sanitizeName(name)
	}
	symbol := formalMethodObjectSymbol(selector, module)
	if symbol == "" {
		symbol = formalMethodBaseSymbol(module, name)
	}
	if formalModuleIsDefinedFunc(module, symbol) {
		return symbol
	}
	return registerFormalExtern(module, symbol, params, results)
}

package gofrontend

import (
	"go/ast"
	"strings"

	"github.com/yuanting/MLSE/internal/gofrontend/formalstdlib"
)

type formalStdlibCallModel = formalstdlib.CallModel

func formalStdlibCallKey(expr ast.Expr, module *formalModuleContext) string {
	selector, ok := expr.(*ast.SelectorExpr)
	if !ok {
		return ""
	}
	if importPath := formalImportPathForSelector(selector, module); importPath != "" {
		return importPath + "." + selector.Sel.Name
	}
	return renderSelector(selector)
}

func lookupFormalStdlibCallModel(expr ast.Expr, module *formalModuleContext) (formalStdlibCallModel, bool) {
	key := formalStdlibCallKey(expr, module)
	if key == "" {
		return formalStdlibCallModel{}, false
	}
	return formalstdlib.Lookup(key)
}

func inferFormalStdlibCallResultType(call *ast.CallExpr, env *formalEnv) (string, bool) {
	module := (*formalModuleContext)(nil)
	if env != nil {
		module = env.module
	}
	model, ok := lookupFormalStdlibCallModel(call.Fun, module)
	if !ok {
		return "", false
	}
	if ty := formalStdlibResultType(model.ResultKind); ty != "" {
		return ty, true
	}
	return "", false
}

func formalStdlibResultType(kind formalstdlib.ResultKind) string {
	switch kind {
	case formalstdlib.ResultString:
		return "!go.string"
	case formalstdlib.ResultError:
		return "!go.error"
	case formalstdlib.ResultBool:
		return "i1"
	case formalstdlib.ResultStringSlice:
		return "!go.slice<!go.string>"
	default:
		return ""
	}
}

func inferFormalStdlibCallArgHint(call *ast.CallExpr, index int, env *formalEnv) (string, bool) {
	module := (*formalModuleContext)(nil)
	if env != nil {
		module = env.module
	}
	model, ok := lookupFormalStdlibCallModel(call.Fun, module)
	if !ok {
		return "", false
	}
	hint := normalizeFormalType(formalStdlibArgHint(model.ArgHintKind, index))
	if hint == "" {
		return "", false
	}
	return hint, true
}

func formalStdlibArgHint(kind formalstdlib.ArgHintKind, index int) string {
	switch kind {
	case formalstdlib.ArgHintAllString:
		return "!go.string"
	case formalstdlib.ArgHintFirstString, formalstdlib.ArgHintFormat:
		if index == 0 {
			return "!go.string"
		}
	}
	return ""
}

func emitFormalStdlibCall(call *ast.CallExpr, hintedTy string, env *formalEnv) (string, string, string, bool) {
	module := (*formalModuleContext)(nil)
	if env != nil {
		module = env.module
	}
	model, ok := lookupFormalStdlibCallModel(call.Fun, module)
	if !ok {
		return "", "", "", false
	}
	switch model.ExprKind {
	case formalstdlib.ExprRuntimeDirect:
		return emitFormalStdlibDirectRuntimeExprCall(call, hintedTy, model, env)
	case formalstdlib.ExprRuntimeFormat:
		resultTy := formalStdlibResolvedResultType(call, hintedTy, env)
		if resultTy == "" || model.RuntimeSymbol.IsZero() {
			return "", "", "", false
		}
		return emitFormalRuntimeFormatCall(call, resultTy, model.RuntimeSymbol, env)
	case formalstdlib.ExprRuntimeAnySlice:
		resultTy := formalStdlibResolvedResultType(call, hintedTy, env)
		if resultTy == "" || model.RuntimeSymbol.IsZero() {
			return "", "", "", false
		}
		return emitFormalRuntimeAnySliceCall(call.Args, resultTy, model.RuntimeSymbol, env)
	}
	return "", "", "", false
}

func emitFormalStdlibCallStmt(call *ast.CallExpr, env *formalEnv) (string, bool) {
	module := (*formalModuleContext)(nil)
	if env != nil {
		module = env.module
	}
	model, ok := lookupFormalStdlibCallModel(call.Fun, module)
	if !ok {
		return "", false
	}
	switch model.StmtKind {
	case formalstdlib.StmtRuntimeAnySlice:
		if model.RuntimeSymbol.IsZero() {
			return "", false
		}
		return emitFormalRuntimePrintCall(call, model.RuntimeSymbol, env)
	case formalstdlib.StmtRuntimeFormat:
		if model.RuntimeSymbol.IsZero() {
			return "", false
		}
		return emitFormalRuntimePrintfCall(call, model.RuntimeSymbol, env)
	}
	return "", false
}

func emitFormalStdlibDirectRuntimeExprCall(call *ast.CallExpr, hintedTy string, model formalStdlibCallModel, env *formalEnv) (string, string, string, bool) {
	if model.RuntimeSymbol.IsZero() {
		return "", "", "", false
	}
	args, argTys, prelude := emitFormalStdlibCallOperands(call, model, env)
	resultTy := formalStdlibResolvedResultType(call, hintedTy, env)
	if resultTy == "" {
		return "", "", "", false
	}
	tmp, callPrelude := emitFormalRuntimeCall(
		formalRuntimeCallSpec{
			symbol:     model.RuntimeSymbol,
			args:       args,
			argTys:     argTys,
			resultTy:   resultTy,
			tempPrefix: "call",
		},
		env,
	)
	return tmp, resultTy, prelude + callPrelude, true
}

func formalStdlibResolvedResultType(call *ast.CallExpr, hintedTy string, env *formalEnv) string {
	resultTy := normalizeFormalType(hintedTy)
	if resultTy != "" && !isFormalOpaquePlaceholderType(resultTy) {
		return resultTy
	}
	if inferred, ok := inferFormalStdlibCallResultType(call, env); ok {
		return inferred
	}
	return ""
}

func emitFormalStdlibCallOperands(call *ast.CallExpr, model formalStdlibCallModel, env *formalEnv) ([]string, []string, string) {
	var (
		args  []string
		types []string
		buf   strings.Builder
	)
	for i, arg := range call.Args {
		hint := normalizeFormalType(formalStdlibArgHint(model.ArgHintKind, i))
		value, ty, prelude := emitFormalExpr(arg, hint, env)
		buf.WriteString(prelude)
		args = append(args, value)
		types = append(types, ty)
	}
	return args, types, buf.String()
}

// This file contains the core Go AST -> formal MLIR dispatcher.
// See docs/go-frontend-lowering.md#core-dispatch.
package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strconv"
	"strings"
)

// emitFormalFunc lowers one parsed Go function or method into module text.
func emitFormalFunc(fn *ast.FuncDecl, module *formalModuleContext) string {
	return emitFormalFuncBody(formalFuncBodySpec{
		name:      formalFuncSymbol(fn, module),
		recv:      fn.Recv,
		fnType:    fn.Type,
		body:      fn.Body,
		scopeNode: fn,
	}, module)
}

func emitFormalFuncBody(spec formalFuncBodySpec, module *formalModuleContext) string {
	env := newFormalEnv(module)
	restoreNode := env.pushNode(spec.scopeNode)
	defer restoreNode()
	env.currentFunc = sanitizeName(spec.name)
	params := emitFormalParams(formalJoinFieldLists(spec.recv, spec.fnType.Params), env)
	results := emitFormalResultTypes(spec.fnType.Results, module)
	env.resultTypes = append([]string(nil), results...)

	var buf strings.Builder
	funcAttrs := ""
	if module != nil {
		funcAttrs = module.scopeAttrForNode(spec.scopeNode)
	}
	if spec.private {
		buf.WriteString(formatPrivateFuncHeaderWithAttrs(spec.name, params, results, funcAttrs))
	} else {
		buf.WriteString(formatFuncHeaderWithAttrs(spec.name, params, results, funcAttrs))
	}

	terminated := false
	if spec.body == nil {
		buf.WriteString(emitFormalLinef(spec.scopeNode, env, "    go.todo %q", "missing_body"))
	} else {
		normalizedBody := normalizeFormalTopLevelLabelsWithReserved(
			spec.body.List,
			collectFormalReservedNames(spec.recv, spec.fnType, spec.body),
		)
		bodyText, term := emitFormalFuncBlock(normalizedBody, env, results)
		buf.WriteString(bodyText)
		terminated = term
	}
	if !terminated {
		if len(results) > 0 {
			buf.WriteString(emitFormalLinef(spec.scopeNode, env, "    go.todo %q", "implicit_return_placeholder"))
		}
		buf.WriteString(emitFormalFallbackReturn(results, env))
	}
	buf.WriteString("  }\n")
	return annotateFormalStructuredOp(buf.String(), spec.scopeNode, env)
}

func emitFormalParams(fields *ast.FieldList, env *formalEnv) []string {
	return emitBoundParams(
		fields,
		func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, env.module) },
		func() string { return env.temp("arg") },
		func(name string, ty string) string { return env.define(name, ty) },
	)
}

func formalJoinFieldLists(lists ...*ast.FieldList) *ast.FieldList {
	var combined []*ast.Field
	for _, list := range lists {
		if list == nil {
			continue
		}
		combined = append(combined, list.List...)
	}
	if len(combined) == 0 {
		return nil
	}
	return &ast.FieldList{List: combined}
}

func emitFormalResultTypes(fields *ast.FieldList, module *formalModuleContext) []string {
	return emitFieldTypes(fields, func(expr ast.Expr) string { return formalTypeExprToMLIR(expr, module) })
}

// emitFormalStmt dispatches statement nodes to the file-specific lowerers.
func emitFormalStmt(stmt ast.Stmt, env *formalEnv, resultTypes []string) (string, bool) {
	restoreNode := env.pushNode(stmt)
	defer restoreNode()
	if resultTypes == nil && env != nil {
		resultTypes = env.resultTypes
	}
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		return emitFormalAssignStmt(s, env), false
	case *ast.ReturnStmt:
		return emitFormalReturnStmt(s, env, resultTypes), true
	case *ast.ExprStmt:
		if text, term := emitFormalTerminatingStmt(s, env); term {
			return text, true
		}
		return emitFormalExprStmt(s, env), false
	case *ast.DeclStmt:
		return emitFormalDeclStmt(s, env), false
	case *ast.IfStmt:
		return emitFormalIfStmt(s, env), false
	case *ast.ForStmt:
		return emitFormalForStmt(s, nil, env), false
	case *ast.RangeStmt:
		return emitFormalRangeStmt(s, env), false
	case *ast.IncDecStmt:
		return emitFormalIncDecStmt(s, env), false
	case *ast.EmptyStmt:
		return "", false
	default:
		return emitFormalLinef(stmt, env, "    go.todo %q", shortNodeName(stmt)), false
	}
}

// emitFormalExpr dispatches expression nodes while threading type hints through lowering.
func emitFormalExpr(expr ast.Expr, hintedTy string, env *formalEnv) (string, string, string) {
	restoreNode := env.pushNode(expr)
	defer restoreNode()
	switch e := expr.(type) {
	case *ast.BasicLit:
		switch e.Kind {
		case token.INT:
			if isFormalFloatType(hintedTy) {
				intTy := formalTargetIntType(env.module)
				intConst := env.temp("const")
				floatConst := env.temp("const")
				targetTy := normalizeFormalType(hintedTy)
				return floatConst, targetTy, fmt.Sprintf(
					"%s%s",
					emitFormalLinef(e, env, "    %s = arith.constant %s : %s", intConst, e.Value, intTy),
					emitFormalLinef(e, env, "    %s = arith.sitofp %s : %s to %s", floatConst, intConst, intTy, targetTy),
				)
			}
			litTy := formalTargetIntType(env.module)
			if isFormalIntegerType(hintedTy) {
				litTy = normalizeFormalType(hintedTy)
			}
			tmp := env.temp("const")
			return tmp, litTy, emitFormalLinef(e, env, "    %s = arith.constant %s : %s", tmp, e.Value, litTy)
		case token.FLOAT:
			litTy := "f64"
			if isFormalFloatType(hintedTy) {
				litTy = normalizeFormalType(hintedTy)
			}
			tmp := env.temp("const")
			return tmp, litTy, emitFormalLinef(e, env, "    %s = arith.constant %s : %s", tmp, e.Value, litTy)
		case token.STRING:
			tmp := env.temp("str")
			quoted := strconv.Quote(strings.Trim(e.Value, "\"`"))
			return tmp, "!go.string", emitFormalLinef(e, env, "    %s = go.string_constant %s : !go.string", tmp, quoted)
		default:
			return emitFormalTodoValue("literal_"+sanitizeName(e.Kind.String()), normalizeFormalType(hintedTy), env)
		}
	case *ast.Ident:
		switch e.Name {
		case "nil":
			ty := normalizeFormalType(hintedTy)
			if hintedTy == "" {
				ty = "!go.error"
			}
			if !isFormalNilableType(ty) {
				value, prelude := emitFormalZeroValue(ty, env)
				return value, ty, prelude
			}
			tmp := env.temp("nil")
			return tmp, ty, emitFormalLinef(e, env, "    %s = go.nil : %s", tmp, ty)
		case "true", "false":
			tmp := env.temp("const")
			return tmp, "i1", emitFormalLinef(e, env, "    %s = arith.constant %s", tmp, e.Name)
		default:
			if _, ok := env.locals[e.Name]; ok {
				return env.use(e.Name), env.typeOf(e.Name), ""
			}
			if value, ty, prelude, ok := emitFormalTypedConstExpr(e, hintedTy, env); ok {
				return value, ty, prelude
			}
			if env.module != nil {
				symbol := formalTopLevelSymbol(env.module, e.Name)
				if sig, ok := env.module.definedFuncs[symbol]; ok {
					funcTy := formatFormalFuncType(sig.params, sig.results)
					tmp := env.temp("fn")
					return tmp, funcTy, emitFormalLinef(e, env, "    %s = func.constant @%s : %s", tmp, symbol, funcTy)
				}
			}
			ty := env.typeOf(e.Name)
			if env.module != nil {
				if typedTy, ok := formalTypedExprType(e, env.module); ok && isFormalTypedInfoUsableType(typedTy) {
					ty = typedTy
				}
			}
			tmp, prelude := emitFormalHelperCall(formalHelperCallSpec{
				base:       e.Name,
				resultTy:   ty,
				tempPrefix: "global",
			}, env)
			return tmp, ty, prelude
		}
	case *ast.BinaryExpr:
		return emitFormalBinaryExpr(e, hintedTy, env)
	case *ast.CallExpr:
		return emitFormalCallExpr(e, hintedTy, env)
	case *ast.CompositeLit:
		return emitFormalCompositeLitExpr(e, hintedTy, env)
	case *ast.FuncLit:
		return emitFormalFuncLitExpr(e, hintedTy, env)
	case *ast.IndexExpr:
		return emitFormalIndexExpr(e, hintedTy, env)
	case *ast.ParenExpr:
		return emitFormalExpr(e.X, hintedTy, env)
	case *ast.SelectorExpr:
		return emitFormalSelectorExpr(e, hintedTy, env)
	case *ast.StarExpr:
		return emitFormalStarExpr(e, hintedTy, env)
	case *ast.TypeAssertExpr:
		return emitFormalTypeAssertExpr(e, hintedTy, env)
	case *ast.UnaryExpr:
		return emitFormalUnaryExpr(e, hintedTy, env)
	default:
		return emitFormalTodoValue(shortNodeName(expr), normalizeFormalType(hintedTy), env)
	}
}

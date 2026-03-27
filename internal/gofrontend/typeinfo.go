package gofrontend

import (
	"bufio"
	"go/ast"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// formalTypeContext is an optional typed layer used to stabilize package selectors,
// method ownership and call signatures before the frontend becomes fully types-driven.
type formalTypeContext struct {
	packagePath string
	imports     map[string]string
	info        *types.Info
	pkg         *types.Package
}

func buildFormalTypeContext(filePath string, fset *token.FileSet, file *ast.File) *formalTypeContext {
	ctx := &formalTypeContext{
		packagePath: discoverFormalPackagePath(filePath, file),
		imports:     collectFormalImports(file),
		info: &types.Info{
			Types:      make(map[ast.Expr]types.TypeAndValue),
			Defs:       make(map[*ast.Ident]types.Object),
			Uses:       make(map[*ast.Ident]types.Object),
			Selections: make(map[*ast.SelectorExpr]*types.Selection),
		},
	}

	if file == nil || fset == nil {
		return ctx
	}

	cfg := types.Config{
		Importer:                 importer.ForCompiler(fset, "source", nil),
		DisableUnusedImportCheck: true,
		Error:                    func(error) {},
		FakeImportC:              true,
	}
	pkg, _ := cfg.Check(ctx.packagePath, fset, []*ast.File{file}, ctx.info)
	ctx.pkg = pkg
	return ctx
}

func collectFormalImports(file *ast.File) map[string]string {
	imports := make(map[string]string)
	if file == nil {
		return imports
	}
	for _, spec := range file.Imports {
		pathValue := strings.Trim(spec.Path.Value, "\"")
		if pathValue == "" {
			continue
		}
		name := path.Base(pathValue)
		if spec.Name != nil && spec.Name.Name != "" && spec.Name.Name != "." && spec.Name.Name != "_" {
			name = spec.Name.Name
		}
		imports[name] = pathValue
	}
	return imports
}

func discoverFormalPackagePath(filePath string, file *ast.File) string {
	moduleDir, modulePath := findFormalModuleRoot(filepath.Dir(filePath))
	if moduleDir != "" && modulePath != "" {
		if rel, err := filepath.Rel(moduleDir, filepath.Dir(filePath)); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			if rel == "." {
				return modulePath
			}
			return modulePath + "/" + filepath.ToSlash(rel)
		}
	}
	return formalPackageName(file)
}

func findFormalModuleRoot(dir string) (string, string) {
	current := dir
	for {
		goModPath := filepath.Join(current, "go.mod")
		modulePath := readFormalModulePath(goModPath)
		if modulePath != "" {
			return current, modulePath
		}
		parent := filepath.Dir(current)
		if parent == current {
			return "", ""
		}
		current = parent
	}
}

func readFormalModulePath(goModPath string) string {
	file, err := os.Open(goModPath)
	if err != nil {
		return ""
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "module ") {
			return strings.TrimSpace(strings.TrimPrefix(line, "module "))
		}
	}
	return ""
}

func formalTypedExprType(expr ast.Expr, module *formalModuleContext) (string, bool) {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return "", false
	}
	if tv, ok := module.typed.info.Types[expr]; ok && tv.Type != nil {
		return goTypesTypeToFormalMLIR(tv.Type, module), true
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if obj := module.typed.info.ObjectOf(e); obj != nil {
			return goTypesTypeToFormalMLIR(obj.Type(), module), true
		}
	case *ast.SelectorExpr:
		if sel := module.typed.info.Selections[e]; sel != nil && sel.Type() != nil {
			return goTypesTypeToFormalMLIR(sel.Type(), module), true
		}
		if obj := module.typed.info.ObjectOf(e.Sel); obj != nil {
			return goTypesTypeToFormalMLIR(obj.Type(), module), true
		}
	}
	return "", false
}

func formalTypeExprToMLIR(expr ast.Expr, module *formalModuleContext) string {
	if expr == nil {
		return goTypeToFormalMLIR(expr)
	}
	if module != nil && module.typed != nil && module.typed.info != nil {
		if tv, ok := module.typed.info.Types[expr]; ok && tv.Type != nil {
			return goTypesTypeToFormalMLIR(tv.Type, module)
		}
		switch e := expr.(type) {
		case *ast.Ident:
			if obj := module.typed.info.ObjectOf(e); obj != nil {
				return goTypesTypeToFormalMLIR(obj.Type(), module)
			}
		case *ast.SelectorExpr:
			if obj := module.typed.info.ObjectOf(e.Sel); obj != nil {
				return goTypesTypeToFormalMLIR(obj.Type(), module)
			}
		}
	}
	return goTypeToFormalMLIR(expr)
}

func isFormalTypedInfoUsableType(ty string) bool {
	ty = normalizeFormalType(ty)
	if isFormalOpaquePlaceholderType(ty) {
		return false
	}
	switch ty {
	case formalOpaqueType("basic"),
		formalOpaqueType("type"),
		formalOpaqueType("struct"),
		formalOpaqueType("array"),
		formalOpaqueType("map"),
		formalOpaqueType("interface"),
		formalOpaqueType("chan"),
		formalOpaqueType("unit"),
		formalOpaqueType("vararg"):
		return false
	}
	if strings.HasPrefix(ty, "!go.ptr<") && strings.HasSuffix(ty, ">") {
		return isFormalTypedInfoUsableType(strings.TrimSuffix(strings.TrimPrefix(ty, "!go.ptr<"), ">"))
	}
	if strings.HasPrefix(ty, "!go.slice<") && strings.HasSuffix(ty, ">") {
		return isFormalTypedInfoUsableType(strings.TrimSuffix(strings.TrimPrefix(ty, "!go.slice<"), ">"))
	}
	return true
}

func formalTypedExprFuncSig(expr ast.Expr, module *formalModuleContext) (formalFuncSig, bool) {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return formalFuncSig{}, false
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if obj := module.typed.info.ObjectOf(e); obj != nil {
			return formalFuncSigFromGoTypes(obj.Type(), module)
		}
	case *ast.SelectorExpr:
		if sel := module.typed.info.Selections[e]; sel != nil {
			return formalFuncSigFromGoTypes(sel.Type(), module)
		}
		if obj := module.typed.info.ObjectOf(e.Sel); obj != nil {
			return formalFuncSigFromGoTypes(obj.Type(), module)
		}
	}
	if tv, ok := module.typed.info.Types[expr]; ok && tv.Type != nil {
		return formalFuncSigFromGoTypes(tv.Type, module)
	}
	return formalFuncSig{}, false
}

func formalFuncSigFromGoTypes(ty types.Type, module *formalModuleContext) (formalFuncSig, bool) {
	sig, ok := ty.(*types.Signature)
	if !ok || sig == nil {
		return formalFuncSig{}, false
	}
	return formalFuncSig{
		params:  formalTupleTypes(sig.Params(), module),
		results: formalTupleTypes(sig.Results(), module),
	}, true
}

func formalTupleTypes(tuple *types.Tuple, module *formalModuleContext) []string {
	if tuple == nil || tuple.Len() == 0 {
		return nil
	}
	out := make([]string, 0, tuple.Len())
	for i := 0; i < tuple.Len(); i++ {
		out = append(out, goTypesTypeToFormalMLIR(tuple.At(i).Type(), module))
	}
	return out
}

func goTypesTypeToFormalMLIR(ty types.Type, module *formalModuleContext) string {
	switch t := ty.(type) {
	case nil:
		return formalOpaqueType("type")
	case *types.Basic:
		switch t.Kind() {
		case types.Bool, types.UntypedBool:
			return "i1"
		case types.Int8, types.Uint8:
			return "i8"
		case types.Int16, types.Uint16:
			return "i16"
		case types.Int, types.Int32, types.Uint, types.Uint32, types.UntypedInt, types.UntypedRune:
			return "i32"
		case types.Int64, types.Uint64, types.Uintptr:
			return "i64"
		case types.String, types.UntypedString:
			return "!go.string"
		case types.UntypedNil:
			return "!go.error"
		default:
			return formalOpaqueType("basic")
		}
	case *types.Pointer:
		return "!go.ptr<" + normalizeFormalElementType(goTypesTypeToFormalMLIR(t.Elem(), module)) + ">"
	case *types.Slice:
		return "!go.slice<" + normalizeFormalElementType(goTypesTypeToFormalMLIR(t.Elem(), module)) + ">"
	case *types.Named:
		if preferred, ok := goTypesPreferredNamedLowering(t, module); ok {
			return preferred
		}
		obj := t.Obj()
		if obj == nil {
			return goTypesTypeToFormalMLIR(t.Underlying(), module)
		}
		if obj.Pkg() == nil {
			if builtinTy, ok := formalBuiltinType(obj.Name()); ok {
				return builtinTy
			}
			if obj.Name() == "error" {
				return "!go.error"
			}
		}
		name := obj.Name()
		if obj.Pkg() != nil && (module == nil || module.packageName == "" || obj.Pkg().Name() != module.packageName) {
			name = obj.Pkg().Name() + "." + name
		}
		return "!go.named<\"" + sanitizeName(name) + "\">"
	case *types.Alias:
		if preferred, ok := goTypesPreferredAliasLowering(t, module); ok {
			return preferred
		}
		if obj := t.Obj(); obj != nil {
			if builtinTy, ok := formalBuiltinType(obj.Name()); ok {
				return builtinTy
			}
		}
		return goTypesTypeToFormalMLIR(t.Underlying(), module)
	case *types.Signature:
		return formatFormalFuncType(formalTupleTypes(t.Params(), module), formalTupleTypes(t.Results(), module))
	case *types.Interface:
		if t.Empty() {
			return formalOpaqueType("any")
		}
		if t.String() == "error" {
			return "!go.error"
		}
		return formalOpaqueType("interface")
	case *types.Map:
		return formalOpaqueType("map")
	case *types.Struct:
		return formalOpaqueType("struct")
	case *types.Array:
		return formalOpaqueType("array")
	case *types.Chan:
		return formalOpaqueType("chan")
	default:
		return formalOpaqueType("type")
	}
}

func goTypesPreferredNamedLowering(named *types.Named, module *formalModuleContext) (string, bool) {
	if named == nil {
		return "", false
	}
	switch underlying := named.Underlying().(type) {
	case *types.Slice, *types.Pointer:
		return goTypesTypeToFormalMLIR(underlying, module), true
	case *types.Basic:
		if underlying.Kind() == types.String {
			return goTypesTypeToFormalMLIR(underlying, module), true
		}
	}
	return "", false
}

func goTypesPreferredAliasLowering(alias *types.Alias, module *formalModuleContext) (string, bool) {
	if alias == nil {
		return "", false
	}
	switch underlying := alias.Underlying().(type) {
	case *types.Slice, *types.Pointer:
		return goTypesTypeToFormalMLIR(underlying, module), true
	case *types.Basic:
		if underlying.Kind() == types.String {
			return goTypesTypeToFormalMLIR(underlying, module), true
		}
	}
	return "", false
}

func formalImportPathForSelector(expr *ast.SelectorExpr, module *formalModuleContext) string {
	root := selectorRootIdent(expr)
	if root == nil || module == nil || module.typed == nil {
		return ""
	}
	if module.typed.info != nil {
		if pkgName, ok := module.typed.info.ObjectOf(root).(*types.PkgName); ok && pkgName.Imported() != nil {
			return pkgName.Imported().Path()
		}
	}
	return module.typed.imports[root.Name]
}

func formalPackageSelectorSymbol(expr *ast.SelectorExpr, module *formalModuleContext) string {
	if expr == nil {
		return ""
	}
	if importPath := formalImportPathForSelector(expr, module); importPath != "" {
		return sanitizeName(strings.ReplaceAll(importPath, "/", ".") + "." + expr.Sel.Name)
	}
	return sanitizeName(renderSelector(expr))
}

func formalMethodObjectSymbol(expr *ast.SelectorExpr, module *formalModuleContext) string {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return ""
	}
	selection := module.typed.info.Selections[expr]
	if selection == nil {
		return ""
	}
	obj := selection.Obj()
	if obj == nil || obj.Pkg() == nil {
		return ""
	}
	return sanitizeName(obj.Pkg().Name() + "." + formalReceiverSymbolFromType(selection.Recv()) + "." + obj.Name())
}

func formalReceiverSymbolFromType(ty types.Type) string {
	switch recv := ty.(type) {
	case *types.Pointer:
		base := formalReceiverSymbolFromType(recv.Elem())
		if base == "" {
			return "ptr"
		}
		return "ptr." + base
	case *types.Named:
		if obj := recv.Obj(); obj != nil {
			return sanitizeName(obj.Name())
		}
	case *types.Alias:
		if obj := recv.Obj(); obj != nil {
			return sanitizeName(obj.Name())
		}
	}
	return sanitizeName(types.TypeString(ty, func(*types.Package) string { return "" }))
}

package gofrontend

import (
	"bufio"
	"fmt"
	"go/ast"
	goconstant "go/constant"
	"go/importer"
	"go/token"
	"go/types"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"strings"

	"golang.org/x/tools/go/packages"
)

// formalTypeContext is an optional typed layer used to stabilize package selectors,
// method ownership and call signatures before the frontend becomes fully types-driven.
type formalTypeContext struct {
	packagePath string
	imports     map[string]string
	goarch      string
	sizes       types.Sizes
	info        *types.Info
	pkg         *types.Package
	fset        *token.FileSet
	sourcePath  string
}

const formalSourceDisplayPathEnv = "MLSE_SOURCE_DISPLAY_PATH"

func buildFormalTypeContext(filePath string, fset *token.FileSet, file *ast.File) *formalTypeContext {
	goarch, _ := detectFormalTarget()
	ctx := &formalTypeContext{
		packagePath: discoverFormalPackagePath(filePath, file),
		imports:     collectFormalImports(file),
		goarch:      goarch,
		sizes:       buildFormalSizes(goarch),
		fset:        fset,
		sourcePath:  resolveFormalDisplaySourcePath(filePath),
	}

	if file == nil || fset == nil {
		return ctx
	}

	importers := []types.Importer{
		importer.Default(),
		importer.ForCompiler(fset, "source", nil),
	}
	for _, imp := range importers {
		info := newFormalTypesInfo()
		cfg := types.Config{
			Importer:                 imp,
			DisableUnusedImportCheck: true,
			Error:                    func(error) {},
			FakeImportC:              true,
			Sizes:                    ctx.sizes,
		}
		pkg, _ := cfg.Check(ctx.packagePath, fset, []*ast.File{file}, info)
		if pkg == nil {
			continue
		}
		ctx.info = info
		ctx.pkg = pkg
		return ctx
	}
	ctx.info = newFormalTypesInfo()
	return ctx
}

func newFormalTypesInfo() *types.Info {
	return &types.Info{
		Types:      make(map[ast.Expr]types.TypeAndValue),
		Defs:       make(map[*ast.Ident]types.Object),
		Uses:       make(map[*ast.Ident]types.Object),
		Selections: make(map[*ast.SelectorExpr]*types.Selection),
	}
}

func loadFormalFileWithPackages(filePath string) (*ast.File, *formalTypeContext, error) {
	absPath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, nil, err
	}

	cfg := &packages.Config{
		Mode: packages.NeedName |
			packages.NeedFiles |
			packages.NeedCompiledGoFiles |
			packages.NeedImports |
			packages.NeedDeps |
			packages.NeedSyntax |
			packages.NeedTypes |
			packages.NeedTypesInfo |
			packages.NeedTypesSizes,
		Dir: filepath.Dir(absPath),
		Env: os.Environ(),
	}
	pkgs, err := packages.Load(cfg, "file="+absPath)
	if err != nil {
		return nil, nil, err
	}
	if len(pkgs) == 0 {
		return nil, nil, fmt.Errorf("packages.Load(%q) returned no packages", absPath)
	}
	for _, pkg := range pkgs {
		if pkg == nil || pkg.Types == nil || pkg.TypesInfo == nil {
			continue
		}
		if len(pkg.Errors) != 0 {
			continue
		}
		for i, compiled := range pkg.CompiledGoFiles {
			if !sameFormalFilePath(compiled, absPath) || i >= len(pkg.Syntax) {
				continue
			}
			file := pkg.Syntax[i]
			goarch, _ := detectFormalTarget()
			sizes := pkg.TypesSizes
			if sizes == nil {
				sizes = buildFormalSizes(goarch)
			}
			return file, &formalTypeContext{
				packagePath: pkg.PkgPath,
				imports:     collectFormalImports(file),
				goarch:      goarch,
				sizes:       sizes,
				info:        pkg.TypesInfo,
				pkg:         pkg.Types,
				fset:        pkg.Fset,
				sourcePath:  resolveFormalDisplaySourcePath(absPath),
			}, nil
		}
	}
	return nil, nil, fmt.Errorf("packages.Load(%q) did not return syntax for file", absPath)
}

func sameFormalFilePath(lhs string, rhs string) bool {
	left := filepath.Clean(lhs)
	right := filepath.Clean(rhs)
	if left == right {
		return true
	}
	if resolved, err := filepath.EvalSymlinks(left); err == nil {
		left = filepath.Clean(resolved)
	}
	if resolved, err := filepath.EvalSymlinks(right); err == nil {
		right = filepath.Clean(resolved)
	}
	return left == right
}

func detectFormalTarget() (string, int) {
	goarch := strings.TrimSpace(os.Getenv("GOARCH"))
	if goarch == "" {
		goarch = runtime.GOARCH
	}
	switch goarch {
	case "386", "arm", "mips", "mipsle", "wasm":
		return goarch, 32
	default:
		return goarch, 64
	}
}

func formalDisplaySourcePath(filePath string) string {
	if filePath == "" {
		return "unknown.go"
	}

	clean := filepath.Clean(filePath)
	if !filepath.IsAbs(clean) {
		return filepath.ToSlash(clean)
	}

	srcMarker := string(filepath.Separator) + "src" + string(filepath.Separator)
	if idx := strings.LastIndex(clean, srcMarker); idx >= 0 {
		return filepath.ToSlash(clean[idx+len(srcMarker):])
	}

	if cwd, err := os.Getwd(); err == nil {
		if rel, err := filepath.Rel(cwd, clean); err == nil && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
			return filepath.ToSlash(rel)
		}
	}

	return filepath.Base(clean)
}

func resolveFormalDisplaySourcePath(filePath string) string {
	if override := strings.TrimSpace(os.Getenv(formalSourceDisplayPathEnv)); override != "" {
		return filepath.ToSlash(filepath.Clean(override))
	}
	return formalDisplaySourcePath(filePath)
}

func buildFormalSizes(goarch string) types.Sizes {
	sizes := types.SizesFor("gc", goarch)
	if sizes != nil {
		return sizes
	}
	return types.SizesFor("gc", runtime.GOARCH)
}

func formalIntegerTypeForBits(bits int) string {
	if bits <= 32 {
		return "i32"
	}
	return "i64"
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

func formalResolvedGoTypesType(expr ast.Expr, module *formalModuleContext) (types.Type, bool) {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return nil, false
	}
	if tv, ok := module.typed.info.Types[expr]; ok && tv.Type != nil {
		return tv.Type, true
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if obj := module.typed.info.ObjectOf(e); obj != nil && obj.Type() != nil {
			return obj.Type(), true
		}
	case *ast.SelectorExpr:
		if obj := module.typed.info.ObjectOf(e.Sel); obj != nil && obj.Type() != nil {
			return obj.Type(), true
		}
	}
	return nil, false
}

func formalTypedConstValue(expr ast.Expr, module *formalModuleContext) (goconstant.Value, types.Type, bool) {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil {
		return nil, nil, false
	}
	if tv, ok := module.typed.info.Types[expr]; ok && tv.Value != nil {
		return tv.Value, tv.Type, true
	}
	switch e := expr.(type) {
	case *ast.Ident:
		if obj, ok := module.typed.info.ObjectOf(e).(*types.Const); ok && obj != nil {
			return obj.Val(), obj.Type(), true
		}
	case *ast.SelectorExpr:
		if obj, ok := module.typed.info.ObjectOf(e.Sel).(*types.Const); ok && obj != nil {
			return obj.Val(), obj.Type(), true
		}
	}
	return nil, nil, false
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

func formalCompositeFieldType(lit *ast.CompositeLit, field string, module *formalModuleContext) string {
	if lit == nil || field == "" || module == nil {
		return ""
	}
	ty, ok := formalResolvedGoTypesType(lit, module)
	if (!ok || ty == nil) && lit.Type != nil {
		ty, ok = formalResolvedGoTypesType(lit.Type, module)
	}
	if !ok || ty == nil {
		return ""
	}
	st, ok := formalUnderlyingGoStruct(ty)
	if !ok {
		return ""
	}
	for i := 0; i < st.NumFields(); i++ {
		if st.Field(i).Name() == field {
			return goTypesTypeToFormalMLIR(st.Field(i).Type(), module)
		}
	}
	return ""
}

func formalCompositeFieldOffset(lit *ast.CompositeLit, field string, module *formalModuleContext) (int64, bool) {
	if lit == nil || field == "" || module == nil || module.typed == nil || module.typed.sizes == nil {
		return 0, false
	}
	ty, ok := formalResolvedGoTypesType(lit, module)
	if (!ok || ty == nil) && lit.Type != nil {
		ty, ok = formalResolvedGoTypesType(lit.Type, module)
	}
	if !ok || ty == nil {
		return 0, false
	}
	return formalStructFieldOffset(ty, field, module.typed.sizes)
}

func formalSelectorFieldOffset(expr *ast.SelectorExpr, module *formalModuleContext) (int64, bool) {
	if expr == nil || module == nil || module.typed == nil || module.typed.info == nil || module.typed.sizes == nil {
		return 0, false
	}
	selection := module.typed.info.Selections[expr]
	if selection == nil || selection.Kind() != types.FieldVal {
		return 0, false
	}
	return formalStructPathOffset(selection.Recv(), selection.Index(), module.typed.sizes)
}

func formalStaticTypeExprSizeAlign(expr ast.Expr, module *formalModuleContext) (int64, int64, bool) {
	if expr == nil {
		return 0, 0, false
	}
	ty, ok := formalResolvedGoTypesType(expr, module)
	if !ok || ty == nil {
		return 0, 0, false
	}
	return formalStaticTypeSizeAlign(ty, module)
}

func formalCompositeStaticSizeAlign(lit *ast.CompositeLit, module *formalModuleContext) (int64, int64, bool) {
	if lit == nil {
		return 0, 0, false
	}
	ty, ok := formalResolvedGoTypesType(lit, module)
	if (!ok || ty == nil) && lit.Type != nil {
		ty, ok = formalResolvedGoTypesType(lit.Type, module)
	}
	if !ok || ty == nil {
		return 0, 0, false
	}
	return formalStaticTypeSizeAlign(ty, module)
}

func formalStaticTypeSizeAlign(ty types.Type, module *formalModuleContext) (int64, int64, bool) {
	if ty == nil {
		return 0, 0, false
	}
	sizes := formalModuleSizes(module)
	if sizes == nil {
		return 0, 0, false
	}
	size := sizes.Sizeof(ty)
	align := sizes.Alignof(ty)
	if size < 0 || align <= 0 {
		return 0, 0, false
	}
	return size, align, true
}

func formalModuleSizes(module *formalModuleContext) types.Sizes {
	if module != nil && module.typed != nil && module.typed.sizes != nil {
		return module.typed.sizes
	}
	goarch, _ := detectFormalTarget()
	return buildFormalSizes(goarch)
}

func formalStructFieldOffset(ty types.Type, field string, sizes types.Sizes) (int64, bool) {
	st, ok := formalUnderlyingGoStruct(ty)
	if !ok || sizes == nil {
		return 0, false
	}
	fields := make([]*types.Var, 0, st.NumFields())
	for i := 0; i < st.NumFields(); i++ {
		fields = append(fields, st.Field(i))
	}
	offsets := sizes.Offsetsof(fields)
	if len(offsets) != len(fields) {
		return 0, false
	}
	for i, candidate := range fields {
		if candidate.Name() == field {
			return offsets[i], true
		}
	}
	return 0, false
}

func formalStructPathOffset(ty types.Type, path []int, sizes types.Sizes) (int64, bool) {
	if sizes == nil || len(path) == 0 {
		return 0, false
	}
	current := ty
	var total int64
	for _, index := range path {
		st, ok := formalUnderlyingGoStruct(current)
		if !ok || index < 0 || index >= st.NumFields() {
			return 0, false
		}
		fields := make([]*types.Var, 0, st.NumFields())
		for i := 0; i < st.NumFields(); i++ {
			fields = append(fields, st.Field(i))
		}
		offsets := sizes.Offsetsof(fields)
		if len(offsets) != len(fields) {
			return 0, false
		}
		total += offsets[index]
		current = st.Field(index).Type()
	}
	return total, true
}

func formalUnderlyingGoStruct(ty types.Type) (*types.Struct, bool) {
	for ty != nil {
		ty = types.Unalias(ty)
		switch t := ty.(type) {
		case *types.Struct:
			return t, true
		case *types.Named:
			ty = t.Underlying()
		case *types.Pointer:
			ty = t.Elem()
		default:
			return nil, false
		}
	}
	return nil, false
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
		case types.Float32:
			return "f32"
		case types.Float64, types.UntypedFloat:
			return "f64"
		case types.Int8, types.Uint8:
			return "i8"
		case types.Int16, types.Uint16:
			return "i16"
		case types.Int, types.Int32, types.Uint, types.Uint32, types.UntypedInt, types.UntypedRune:
			if module != nil && module.targetIntTy != "" {
				return module.targetIntTy
			}
			return formalTargetIntType(module)
		case types.Int64, types.Uint64:
			return "i64"
		case types.Uintptr:
			if module != nil && module.targetIntTy != "" {
				return module.targetIntTy
			}
			return formalTargetIntType(module)
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
			if builtinTy, ok := formalBuiltinTypeWithModule(obj.Name(), module); ok {
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
			if builtinTy, ok := formalBuiltinTypeWithModule(obj.Name(), module); ok {
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

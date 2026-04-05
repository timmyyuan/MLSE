package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

// formalModuleContext stores module-level facts needed while printing one file.
type formalModuleContext struct {
	packageName   string
	goarch        string
	targetIntTy   string
	targetIntBits int
	typed         *formalTypeContext
	definedFuncs  map[string]formalFuncSig
	namedTypes    map[string]struct{}
	externByKey   map[string]formalExternDecl
	externOrder   []string
	generated     []string
	nextFuncLit   int
	funcLitByKey  map[string]int
	scopes        []formalScopeEntry
	scopeByNode   map[ast.Node]int
	parentByNode  map[ast.Node]ast.Node
}

// newFormalModuleContext collects module-level declarations before function lowering starts.
func newFormalModuleContext(file *ast.File, funcs []*ast.FuncDecl, typed *formalTypeContext) *formalModuleContext {
	goarch, targetIntBits := detectFormalTarget()
	module := &formalModuleContext{
		packageName:   sanitizeName(formalPackageName(file)),
		goarch:        goarch,
		targetIntTy:   formalIntegerTypeForBits(targetIntBits),
		targetIntBits: targetIntBits,
		typed:         typed,
		definedFuncs:  make(map[string]formalFuncSig, len(funcs)),
		namedTypes:    make(map[string]struct{}),
		externByKey:   make(map[string]formalExternDecl),
		funcLitByKey:  make(map[string]int),
		scopeByNode:   make(map[ast.Node]int),
		parentByNode:  make(map[ast.Node]ast.Node),
	}
	for _, fn := range funcs {
		module.definedFuncs[formalFuncSymbol(fn, module)] = formalFuncSigFromDecl(fn, module)
	}
	if file != nil {
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, spec := range gen.Specs {
				typeSpec, ok := spec.(*ast.TypeSpec)
				if !ok {
					continue
				}
				module.namedTypes[typeSpec.Name.Name] = struct{}{}
			}
		}
		module.indexScopeMetadata(file)
	}
	return module
}

func formalPackageName(file *ast.File) string {
	if file == nil || file.Name == nil || file.Name.Name == "" {
		return "pkg"
	}
	return file.Name.Name
}

func formalFuncSymbol(fn *ast.FuncDecl, module *formalModuleContext) string {
	if fn == nil || fn.Name == nil {
		return "anon"
	}
	if fn.Recv != nil && len(fn.Recv.List) != 0 && module != nil {
		return module.methodSymbolForReceiver(formalReceiverSymbolFromFieldList(fn.Recv), fn.Name.Name)
	}
	if module != nil {
		return module.topLevelSymbol(fn.Name.Name)
	}
	return sanitizeName(fn.Name.Name)
}

func (m *formalModuleContext) emitExternDecls() string {
	var buf strings.Builder
	for _, key := range m.externOrder {
		decl := m.externByKey[key]
		buf.WriteString(formatFuncDecl(decl.symbol, decl.params, decl.results))
	}
	return buf.String()
}

func (m *formalModuleContext) emitGeneratedFuncs() string {
	return strings.Join(m.generated, "")
}

func (m *formalModuleContext) emitScopeTableAttr() string {
	if m == nil {
		return ""
	}
	parts := make([]string, 0, len(m.scopes))
	for _, scope := range m.scopes {
		parts = append(parts, fmt.Sprintf(
			"{id = %d : i64, label = %q, parent = %d : i64, kind = %q, name = %q, file = %q, line = %d : i64, col = %d : i64}",
			scope.ID,
			scope.Label,
			scope.Parent,
			scope.Kind,
			scope.Name,
			scope.File,
			scope.Line,
			scope.Column,
		))
	}
	return "go.scope_table = [" + strings.Join(parts, ", ") + "]"
}

func (m *formalModuleContext) isDefinedFunc(name string) bool {
	if m == nil {
		return false
	}
	_, ok := m.definedFuncs[sanitizeName(name)]
	return ok
}

func (m *formalModuleContext) topLevelSymbol(name string) string {
	if m == nil || m.packageName == "" {
		return sanitizeName(name)
	}
	return sanitizeName(m.packageName + "." + name)
}

func (m *formalModuleContext) methodSymbol(name string) string {
	return m.methodSymbolForReceiver("", name)
}

func (m *formalModuleContext) methodSymbolForReceiver(recv string, name string) string {
	base := sanitizeName(name)
	if recv != "" {
		base = sanitizeName(recv) + "." + base
	}
	if m == nil || m.packageName == "" {
		return base
	}
	return sanitizeName(m.packageName + "." + base)
}

func (m *formalModuleContext) isNamedType(name string) bool {
	if m == nil {
		return false
	}
	_, ok := m.namedTypes[name]
	return ok
}

func (m *formalModuleContext) registerExtern(base string, params []string, results []string) string {
	if m == nil {
		return sanitizeName(base)
	}
	key := base + "|(" + strings.Join(params, ",") + ")->(" + strings.Join(results, ",") + ")"
	if decl, ok := m.externByKey[key]; ok {
		return decl.symbol
	}
	symbol := sanitizeName(base)
	if m.isDefinedFunc(symbol) || m.externSymbolTaken(symbol) {
		for i := 2; ; i++ {
			candidate := fmt.Sprintf("%s__sig%d", symbol, i)
			if !m.isDefinedFunc(candidate) && !m.externSymbolTaken(candidate) {
				symbol = candidate
				break
			}
		}
	}
	m.externByKey[key] = formalExternDecl{
		symbol:  symbol,
		params:  append([]string(nil), params...),
		results: append([]string(nil), results...),
	}
	m.externOrder = append(m.externOrder, key)
	return symbol
}

func (m *formalModuleContext) externSymbolTaken(symbol string) bool {
	for _, decl := range m.externByKey {
		if decl.symbol == symbol {
			return true
		}
	}
	return false
}

func (m *formalModuleContext) reserveFuncLitSymbol(sig formalFuncSig, enclosing string) string {
	if m == nil {
		return sanitizeName(enclosing + ".__lit0")
	}
	key := sanitizeName(enclosing)
	if key == "" || key == "anon" {
		key = m.topLevelSymbol("anon")
	}
	for {
		index := m.funcLitByKey[key]
		m.funcLitByKey[key] = index + 1
		m.nextFuncLit++
		symbol := sanitizeName(fmt.Sprintf("%s.__lit%d", key, index))
		if m.isDefinedFunc(symbol) || m.externSymbolTaken(symbol) {
			continue
		}
		m.definedFuncs[symbol] = formalFuncSig{
			params:  append([]string(nil), sig.params...),
			results: append([]string(nil), sig.results...),
		}
		return symbol
	}
}

func (m *formalModuleContext) addGeneratedFunc(text string) {
	if m == nil {
		return
	}
	m.generated = append(m.generated, text)
}

func (m *formalModuleContext) scopeForNode(node ast.Node) (formalScopeEntry, bool) {
	if m == nil || node == nil {
		return formalScopeEntry{}, false
	}
	for current := node; current != nil; current = m.parentByNode[current] {
		if id, ok := m.scopeByNode[current]; ok && id >= 0 && id < len(m.scopes) {
			return m.scopes[id], true
		}
	}
	return formalScopeEntry{}, false
}

func (m *formalModuleContext) scopeAttrForNode(node ast.Node) string {
	scope, ok := m.scopeForNode(node)
	if !ok {
		return ""
	}
	return fmt.Sprintf("attributes {go.scope = %d : i64}", scope.ID)
}

func (m *formalModuleContext) sourcePosition(node ast.Node) (string, int, int, bool) {
	if m == nil || m.typed == nil || m.typed.fset == nil || node == nil {
		return "", 0, 0, false
	}
	pos := m.typed.fset.PositionFor(node.Pos(), false)
	if !pos.IsValid() {
		return "", 0, 0, false
	}
	file := m.typed.sourcePath
	if file == "" {
		file = formalDisplaySourcePath(pos.Filename)
	}
	return file, pos.Line, pos.Column, true
}

func (m *formalModuleContext) indexScopeMetadata(file *ast.File) {
	if m == nil || file == nil {
		return
	}
	var stack []ast.Node
	ast.Inspect(file, func(n ast.Node) bool {
		if n == nil {
			if len(stack) != 0 {
				stack = stack[:len(stack)-1]
			}
			return false
		}
		if len(stack) != 0 {
			m.parentByNode[n] = stack[len(stack)-1]
		}
		if kind, name, ok := m.scopeDescriptor(n); ok {
			m.addScope(n, kind, name, m.scopeParentID(stack))
		}
		stack = append(stack, n)
		return true
	})
}

func (m *formalModuleContext) scopeDescriptor(node ast.Node) (string, string, bool) {
	switch n := node.(type) {
	case *ast.FuncDecl:
		return "func", formalFuncSymbol(n, m), true
	case *ast.FuncLit:
		return "funclit", "funclit", true
	case *ast.IfStmt:
		return "if", "if", true
	case *ast.ForStmt:
		return "for", "for", true
	case *ast.RangeStmt:
		return "range", "range", true
	default:
		return "", "", false
	}
}

func (m *formalModuleContext) scopeParentID(stack []ast.Node) int {
	for i := len(stack) - 1; i >= 0; i-- {
		if id, ok := m.scopeByNode[stack[i]]; ok {
			return id
		}
	}
	return -1
}

func (m *formalModuleContext) addScope(node ast.Node, kind string, name string, parent int) {
	if m == nil || node == nil {
		return
	}
	file, line, col, _ := m.sourcePosition(node)
	id := len(m.scopes)
	entry := formalScopeEntry{
		ID:     id,
		Label:  fmt.Sprintf("scope%d", id),
		Parent: parent,
		Kind:   kind,
		Name:   sanitizeName(name),
		File:   file,
		Line:   line,
		Column: col,
	}
	m.scopes = append(m.scopes, entry)
	m.scopeByNode[node] = id
}

func formalReceiverSymbolFromFieldList(fields *ast.FieldList) string {
	if fields == nil || len(fields.List) == 0 {
		return ""
	}
	return formalReceiverSymbolFromExpr(fields.List[0].Type)
}

func formalReceiverSymbolFromExpr(expr ast.Expr) string {
	switch recv := expr.(type) {
	case nil:
		return ""
	case *ast.Ident:
		return sanitizeName(recv.Name)
	case *ast.ParenExpr:
		return formalReceiverSymbolFromExpr(recv.X)
	case *ast.StarExpr:
		base := formalReceiverSymbolFromExpr(recv.X)
		if base == "" {
			return "ptr"
		}
		return "ptr." + base
	case *ast.SelectorExpr:
		return sanitizeName(renderSelector(recv))
	case *ast.IndexExpr:
		return formalReceiverSymbolFromExpr(recv.X)
	case *ast.IndexListExpr:
		return formalReceiverSymbolFromExpr(recv.X)
	default:
		return sanitizeName(shortNodeName(expr))
	}
}

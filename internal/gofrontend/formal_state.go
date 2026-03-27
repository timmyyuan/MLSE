package gofrontend

import (
	"fmt"
	"go/ast"
	"go/token"
	"strings"
)

type formalBinding struct {
	current string
	ty      string
	funcSig *formalFuncSig
}

type formalFuncSig struct {
	params  []string
	results []string
}

type formalExternDecl struct {
	symbol  string
	params  []string
	results []string
}

// formalModuleContext stores module-level facts needed while printing one file.
type formalModuleContext struct {
	packageName  string
	typed        *formalTypeContext
	definedFuncs map[string]formalFuncSig
	namedTypes   map[string]struct{}
	externByKey  map[string]formalExternDecl
	externOrder  []string
	generated    []string
	nextFuncLit  int
	funcLitByKey map[string]int
}

// formalEnv tracks local SSA names, inferred types and generated temporaries for one lowering scope.
type formalEnv struct {
	locals      map[string]*formalBinding
	tempID      int
	module      *formalModuleContext
	resultTypes []string
	currentFunc string
}

type formalFuncBodySpec struct {
	name    string
	recv    *ast.FieldList
	fnType  *ast.FuncType
	body    *ast.BlockStmt
	private bool
}

type formalHelperCallSpec struct {
	base       string
	args       []string
	argTys     []string
	resultTy   string
	tempPrefix string
}

// newFormalEnv allocates one function-scoped lowering environment.
func newFormalEnv(module *formalModuleContext) *formalEnv {
	return &formalEnv{locals: make(map[string]*formalBinding), module: module}
}

func (e *formalEnv) define(name string, ty string) string {
	if binding, ok := e.locals[name]; ok {
		if ty != "" {
			binding.ty = ty
			binding.funcSig = formalFuncSigForType(ty)
		}
		return binding.current
	}
	ssa := "%" + sanitizeName(name)
	e.locals[name] = &formalBinding{current: ssa, ty: ty, funcSig: formalFuncSigForType(ty)}
	return ssa
}

func (e *formalEnv) assign(name string, ty string) string {
	if _, ok := e.locals[name]; !ok {
		return e.define(name, ty)
	}
	binding := e.locals[name]
	if ty != "" {
		binding.ty = ty
		binding.funcSig = formalFuncSigForType(ty)
	}
	return binding.current
}

func (e *formalEnv) defineOrAssign(name string, ty string) string {
	if _, ok := e.locals[name]; ok {
		return e.assign(name, ty)
	}
	return e.define(name, ty)
}

func (e *formalEnv) bindValue(name string, value string, ty string) {
	if binding, ok := e.locals[name]; ok {
		binding.current = value
		if ty != "" {
			binding.ty = ty
			binding.funcSig = formalFuncSigForType(ty)
		}
		return
	}
	e.locals[name] = &formalBinding{current: value, ty: ty, funcSig: formalFuncSigForType(ty)}
}

func (e *formalEnv) use(name string) string {
	if binding, ok := e.locals[name]; ok {
		return binding.current
	}
	return e.define(name, formalOpaqueType("value"))
}

func (e *formalEnv) typeOf(name string) string {
	if binding, ok := e.locals[name]; ok && binding.ty != "" {
		return binding.ty
	}
	return formalOpaqueType("value")
}

func (e *formalEnv) temp(prefix string) string {
	e.tempID++
	return fmt.Sprintf("%%%s%d", sanitizeName(prefix), e.tempID)
}

func (e *formalEnv) clone() *formalEnv {
	cloned := &formalEnv{
		locals:      make(map[string]*formalBinding, len(e.locals)),
		tempID:      e.tempID,
		module:      e.module,
		resultTypes: append([]string(nil), e.resultTypes...),
		currentFunc: e.currentFunc,
	}
	for name, binding := range e.locals {
		copied := *binding
		if binding.funcSig != nil {
			copied.funcSig = cloneFormalFuncSig(*binding.funcSig)
		}
		cloned.locals[name] = &copied
	}
	return cloned
}

func formalFuncSigForType(ty string) *formalFuncSig {
	sig, ok := parseFormalFuncType(ty)
	if !ok {
		return nil
	}
	return cloneFormalFuncSig(sig)
}

func cloneFormalFuncSig(sig formalFuncSig) *formalFuncSig {
	return &formalFuncSig{
		params:  append([]string(nil), sig.params...),
		results: append([]string(nil), sig.results...),
	}
}

func syncFormalTempID(target *formalEnv, others ...*formalEnv) {
	for _, other := range others {
		if other != nil && other.tempID > target.tempID {
			target.tempID = other.tempID
		}
	}
}

// newFormalModuleContext collects module-level declarations before function lowering starts.
func newFormalModuleContext(file *ast.File, funcs []*ast.FuncDecl, typed *formalTypeContext) *formalModuleContext {
	module := &formalModuleContext{
		packageName:  sanitizeName(formalPackageName(file)),
		typed:        typed,
		definedFuncs: make(map[string]formalFuncSig, len(funcs)),
		namedTypes:   make(map[string]struct{}),
		externByKey:  make(map[string]formalExternDecl),
		funcLitByKey: make(map[string]int),
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

package gofrontend

func emitFormalGeneratedFuncs(module *formalModuleContext) string {
	if module == nil {
		return ""
	}
	return module.emitGeneratedFuncs()
}

func emitFormalExternDecls(module *formalModuleContext) string {
	if module == nil {
		return ""
	}
	return module.emitExternDecls()
}

func registerFormalExtern(module *formalModuleContext, base string, params []string, results []string) string {
	if module == nil {
		return sanitizeName(base)
	}
	return module.registerExtern(base, params, results)
}

func reserveFormalFuncLitSymbol(module *formalModuleContext, sig formalFuncSig, enclosing string) string {
	if module == nil {
		return sanitizeName(enclosing + ".__lit0")
	}
	return module.reserveFuncLitSymbol(sig, enclosing)
}

func addFormalGeneratedFunc(module *formalModuleContext, text string) {
	if module == nil {
		return
	}
	module.addGeneratedFunc(text)
}

func formalTopLevelSymbol(module *formalModuleContext, name string) string {
	if module == nil {
		return sanitizeName(name)
	}
	return module.topLevelSymbol(name)
}

func formalMethodBaseSymbol(module *formalModuleContext, name string) string {
	if module == nil {
		return sanitizeName(name)
	}
	return module.methodSymbol(name)
}

func formalModuleIsDefinedFunc(module *formalModuleContext, name string) bool {
	return module != nil && module.isDefinedFunc(name)
}

func formalModuleIsNamedType(module *formalModuleContext, name string) bool {
	return module != nil && module.isNamedType(name)
}

func lookupFormalDefinedFuncSig(module *formalModuleContext, symbol string) (formalFuncSig, bool) {
	if module == nil {
		return formalFuncSig{}, false
	}
	sig, ok := module.definedFuncs[symbol]
	if !ok {
		return formalFuncSig{}, false
	}
	return formalFuncSig{
		params:  append([]string(nil), sig.params...),
		results: append([]string(nil), sig.results...),
	}, true
}

package goirllvmexp

func LowerToLLVMDialectModule(input string) (string, error) {
	return LowerToLLVMDialectModuleWithOptions(input, LoweringOptions{})
}

func LowerToLLVMDialectModuleWithOptions(input string, opts LoweringOptions) (string, error) {
	opts, err := normalizeLoweringOptions(opts)
	if err != nil {
		return "", err
	}
	mod, err := parseModule(input)
	if err != nil {
		return "", err
	}
	return emitLLVMDialectModule(mod, opts)
}

func TranslateModule(input string) (string, error) {
	return TranslateModuleWithOptions(input, LoweringOptions{})
}

func TranslateModuleWithOptions(input string, opts LoweringOptions) (string, error) {
	llvmDialect, err := LowerToLLVMDialectModuleWithOptions(input, opts)
	if err != nil {
		return "", err
	}
	return translateLLVMDialectModule(llvmDialect)
}

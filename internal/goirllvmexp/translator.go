package goirllvmexp

func LowerToLLVMDialectModule(input string) (string, error) {
	mod, err := parseModule(input)
	if err != nil {
		return "", err
	}
	return emitLLVMDialectModule(mod)
}

func TranslateModule(input string) (string, error) {
	llvmDialect, err := LowerToLLVMDialectModule(input)
	if err != nil {
		return "", err
	}
	return translateLLVMDialectModule(llvmDialect)
}

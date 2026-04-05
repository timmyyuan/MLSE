package gofrontend

func formalTargetIntType(module *formalModuleContext) string {
	if module != nil && module.targetIntTy != "" {
		return module.targetIntTy
	}
	_, bits := detectFormalTarget()
	return formalIntegerTypeForBits(bits)
}

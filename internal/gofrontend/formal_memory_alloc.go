package gofrontend

type formalStaticAllocSpec struct {
	resultTy string
	size     int64
	align    int64
}

func emitFormalStaticAlloc(spec formalStaticAllocSpec, env *formalEnv) (string, string, bool) {
	return emitFormalRuntimeNewObject(spec.resultTy, spec.size, spec.align, env)
}

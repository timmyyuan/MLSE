package gofrontend

import (
	"fmt"
	"go/ast"
)

// formalEnv tracks local SSA names, inferred types and generated temporaries for one lowering scope.
type formalEnv struct {
	locals      map[string]*formalBinding
	tempID      int
	module      *formalModuleContext
	resultTypes []string
	currentFunc string
	currentNode ast.Node
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
		currentNode: e.currentNode,
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

func syncFormalTempID(target *formalEnv, others ...*formalEnv) {
	for _, other := range others {
		if other != nil && other.tempID > target.tempID {
			target.tempID = other.tempID
		}
	}
}

func (e *formalEnv) pushNode(node ast.Node) func() {
	if e == nil || node == nil {
		return func() {}
	}
	prev := e.currentNode
	e.currentNode = node
	return func() {
		e.currentNode = prev
	}
}

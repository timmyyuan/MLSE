package goirllvmexp

import (
	"errors"
	"fmt"
)

type sliceFieldKind int

const (
	sliceFieldData sliceFieldKind = iota
	sliceFieldLen
	sliceFieldCap
)

type sliceValueParts struct {
	Data     string
	Length   string
	Capacity string
}

type sliceLayout struct {
	model SliceModel
}

func newSliceLayout(model SliceModel) sliceLayout {
	return sliceLayout{model: normalizeSliceModel(model)}
}

func (l sliceLayout) llvmType() string {
	if l.hasCap() {
		return "!llvm.struct<(!llvm.ptr, i32, i32)>"
	}
	return "!llvm.struct<(!llvm.ptr, i32)>"
}

func (l sliceLayout) hasCap() bool {
	return l.model == SliceModelCap
}

func (l sliceLayout) fieldIndex(field sliceFieldKind) (int, bool) {
	switch field {
	case sliceFieldData:
		return 0, true
	case sliceFieldLen:
		return 1, true
	case sliceFieldCap:
		if l.hasCap() {
			return 2, true
		}
	}
	return 0, false
}

func (l sliceLayout) fieldName(field sliceFieldKind) string {
	switch field {
	case sliceFieldData:
		return "data"
	case sliceFieldLen:
		return "len"
	case sliceFieldCap:
		return "cap"
	default:
		return "field"
	}
}

func (l sliceLayout) extract(e *funcEmitter, value string, llvmTy string, field sliceFieldKind) (string, error) {
	if llvmTy != l.llvmType() {
		return "", fmt.Errorf("expected slice value, got %s", llvmTy)
	}
	index, ok := l.fieldIndex(field)
	if !ok {
		return "", fmt.Errorf("slice field %q is unavailable in %s mode", l.fieldName(field), l.model)
	}
	name := e.freshValue("slice." + l.fieldName(field))
	e.emitInstruction(fmt.Sprintf("%s = llvm.extractvalue %s[%d] : %s", name, value, index, llvmTy))
	return name, nil
}

func (l sliceLayout) build(e *funcEmitter, parts sliceValueParts) (string, string, error) {
	if parts.Data == "" || parts.Length == "" {
		return "", "", errors.New("cannot build slice from empty parts")
	}
	if l.hasCap() && parts.Capacity == "" {
		return "", "", errors.New("cannot build cap slice without capacity")
	}

	type fieldValue struct {
		kind  sliceFieldKind
		value string
	}
	values := []fieldValue{
		{kind: sliceFieldData, value: parts.Data},
		{kind: sliceFieldLen, value: parts.Length},
	}
	if l.hasCap() {
		values = append(values, fieldValue{kind: sliceFieldCap, value: parts.Capacity})
	}

	llvmTy := l.llvmType()
	aggregate := e.freshValue("slice")
	e.emitInstruction(fmt.Sprintf("%s = llvm.mlir.undef : %s", aggregate, llvmTy))
	for _, part := range values {
		index, _ := l.fieldIndex(part.kind)
		next := e.freshValue("slice")
		e.emitInstruction(fmt.Sprintf("%s = llvm.insertvalue %s, %s[%d] : %s", next, part.value, aggregate, index, llvmTy))
		aggregate = next
	}
	return aggregate, llvmTy, nil
}

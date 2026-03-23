package goirllvmexp

import "fmt"

type SliceModel string

const (
	SliceModelMin SliceModel = "min"
	SliceModelCap SliceModel = "cap"
)

type LoweringOptions struct {
	SliceModel SliceModel
}

func ParseSliceModel(raw string) (SliceModel, error) {
	switch SliceModel(raw) {
	case "", SliceModelMin:
		return SliceModelMin, nil
	case SliceModelCap:
		return SliceModelCap, nil
	default:
		return "", fmt.Errorf("unsupported slice model %q (want min or cap)", raw)
	}
}

func normalizeLoweringOptions(opts LoweringOptions) (LoweringOptions, error) {
	model, err := ParseSliceModel(string(opts.SliceModel))
	if err != nil {
		return LoweringOptions{}, err
	}
	opts.SliceModel = model
	return opts, nil
}

func normalizeSliceModel(model SliceModel) SliceModel {
	if model == SliceModelCap {
		return SliceModelCap
	}
	return SliceModelMin
}

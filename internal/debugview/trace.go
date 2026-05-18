package debugview

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type TraceSnapshot struct {
	SourcePath string      `json:"sourcePath,omitempty"`
	SourceName string      `json:"sourceName,omitempty"`
	Status     string      `json:"status,omitempty"`
	Paths      []TracePath `json:"paths"`
}

type TracePath struct {
	ID            string       `json:"id"`
	Case          string       `json:"case,omitempty"`
	Function      string       `json:"function,omitempty"`
	Status        string       `json:"status,omitempty"`
	Expected      string       `json:"expected,omitempty"`
	FirstBlocker  string       `json:"firstBlocker,omitempty"`
	ArtifactDir   string       `json:"artifactDir,omitempty"`
	Inputs        []TraceValue `json:"inputs,omitempty"`
	Frames        []TraceFrame `json:"frames,omitempty"`
	Events        []TraceEvent `json:"events,omitempty"`
	PathCondition []string     `json:"pathCondition,omitempty"`
}

type TraceValue struct {
	Name  string `json:"name,omitempty"`
	Type  string `json:"type,omitempty"`
	Value string `json:"value,omitempty"`
}

type TraceFrame struct {
	Index    int    `json:"index"`
	Side     string `json:"side,omitempty"`
	Function string `json:"function,omitempty"`
	Source   string `json:"source,omitempty"`
	Bitcode  string `json:"bitcode,omitempty"`
	Status   string `json:"status,omitempty"`
}

type TraceEvent struct {
	Index    int    `json:"index"`
	Kind     string `json:"kind,omitempty"`
	Side     string `json:"side,omitempty"`
	Stage    string `json:"stage,omitempty"`
	Status   string `json:"status,omitempty"`
	Detail   string `json:"detail,omitempty"`
	Artifact string `json:"artifact,omitempty"`
}

func LoadTrace(path string) (*TraceSnapshot, error) {
	if path == "" {
		return nil, nil
	}
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}
	var probe probeSummary
	if err := json.Unmarshal(content, &probe); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}
	if len(probe.Paths) != 0 {
		trace := TraceSnapshot{
			SourcePath: probe.SourcePath,
			SourceName: probe.SourceName,
			Status:     probe.Status,
			Paths:      probe.Paths,
		}
		if trace.SourcePath == "" {
			trace.SourcePath = path
		}
		if trace.SourceName == "" {
			trace.SourceName = filepath.Base(path)
		}
		return &trace, nil
	}
	return traceFromProbeSummary(path, probe), nil
}

type probeSummary struct {
	SourcePath string        `json:"sourcePath"`
	SourceName string        `json:"sourceName"`
	Status     string        `json:"status"`
	Paths      []TracePath   `json:"paths"`
	Results    []probeResult `json:"results"`
}

type probeResult struct {
	Case           string                   `json:"case"`
	Function       string                   `json:"function"`
	ExpectedStatus string                   `json:"expected_status"`
	ArtifactDir    string                   `json:"artifact_dir"`
	Status         string                   `json:"status"`
	FirstBlocker   string                   `json:"first_blocker"`
	MissingWork    []string                 `json:"missing_work"`
	Sides          []map[string]interface{} `json:"sides"`
	KleeDiff       map[string]interface{}   `json:"klee_diff"`
}

type probeCase struct {
	Inputs []TraceValue `json:"inputs"`
}

func traceFromProbeSummary(path string, probe probeSummary) *TraceSnapshot {
	trace := &TraceSnapshot{
		SourcePath: path,
		SourceName: filepath.Base(path),
		Status:     probe.Status,
		Paths:      make([]TracePath, 0, len(probe.Results)),
	}
	baseDir := filepath.Dir(path)
	for _, result := range probe.Results {
		trace.Paths = append(trace.Paths, tracePathFromProbeResult(baseDir, result))
	}
	return trace
}

func tracePathFromProbeResult(baseDir string, result probeResult) TracePath {
	path := TracePath{
		ID:           result.Case,
		Case:         result.Case,
		Function:     result.Function,
		Status:       result.Status,
		Expected:     result.ExpectedStatus,
		FirstBlocker: result.FirstBlocker,
		ArtifactDir:  result.ArtifactDir,
		Inputs:       loadProbeInputs(baseDir, result.ArtifactDir),
	}
	for _, side := range result.Sides {
		path.Frames = append(path.Frames, TraceFrame{
			Index:    len(path.Frames),
			Side:     stringField(side, "label"),
			Function: result.Function,
			Source:   stringField(side, "source"),
			Bitcode:  stringField(side, "bitcode"),
			Status:   sideStatus(side),
		})
		appendProbeStageEvents(&path.Events, side)
	}
	for _, item := range result.MissingWork {
		path.Events = append(path.Events, TraceEvent{
			Index:  len(path.Events),
			Kind:   "missing_work",
			Status: "blocked",
			Detail: item,
		})
	}
	if result.KleeDiff != nil {
		path.Events = append(path.Events, TraceEvent{
			Index:  len(path.Events),
			Kind:   "klee",
			Stage:  "klee_diff",
			Status: stringField(result.KleeDiff, "status"),
			Detail: stringField(result.KleeDiff, "reason"),
		})
		for _, detail := range stringSliceField(result.KleeDiff, "counterexample_errors") {
			path.Events = append(path.Events, TraceEvent{
				Index:  len(path.Events),
				Kind:   "counterexample",
				Stage:  "klee_diff",
				Status: "counterexample",
				Detail: detail,
			})
		}
	}
	if result.FirstBlocker != "" {
		path.Events = append(path.Events, TraceEvent{
			Index:  len(path.Events),
			Kind:   "blocker",
			Status: "blocked",
			Detail: result.FirstBlocker,
		})
	}
	if result.ExpectedStatus != "" {
		path.PathCondition = append(path.PathCondition, "expected_status = "+result.ExpectedStatus)
	}
	if result.FirstBlocker != "" {
		path.PathCondition = append(path.PathCondition, "first_blocker = "+result.FirstBlocker)
	}
	return path
}

func appendProbeStageEvents(events *[]TraceEvent, side map[string]interface{}) {
	for _, stage := range probeStageOrder {
		status := stringField(side, stage.key+"_status")
		if status == "" {
			continue
		}
		*events = append(*events, TraceEvent{
			Index:    len(*events),
			Kind:     "stage",
			Side:     stringField(side, "label"),
			Stage:    stage.label,
			Status:   status,
			Detail:   stringField(side, stage.key+"_reason"),
			Artifact: stageArtifact(side, stage.key),
		})
	}
}

var probeStageOrder = []struct {
	key   string
	label string
}{
	{"mlse_go", "mlse-go"},
	{"mlse_opt_roundtrip", "mlse-opt roundtrip"},
	{"go_bootstrap_lower", "go bootstrap lower"},
	{"mlir_opt_llvm", "mlir-opt llvm"},
	{"mlir_translate", "mlir-translate"},
	{"llvm_as", "llvm-as"},
}

func stageArtifact(side map[string]interface{}, stage string) string {
	switch stage {
	case "mlse_go":
		return stringField(side, "source")
	case "llvm_as":
		return stringField(side, "bitcode")
	default:
		return ""
	}
}

func sideStatus(side map[string]interface{}) string {
	status := "success"
	for _, stage := range probeStageOrder {
		stageStatus := stringField(side, stage.key+"_status")
		if stageStatus == "" {
			continue
		}
		if stageStatus != "success" {
			return stageStatus
		}
	}
	return status
}

func loadProbeInputs(baseDir string, artifactDir string) []TraceValue {
	if artifactDir == "" {
		return nil
	}
	casePath := filepath.Join(resolveTraceArtifactDir(baseDir, artifactDir), "case.json")
	content, err := os.ReadFile(casePath)
	if err != nil {
		return nil
	}
	var probe probeCase
	if err := json.Unmarshal(content, &probe); err != nil {
		return nil
	}
	return probe.Inputs
}

func resolveTraceArtifactDir(baseDir string, artifactDir string) string {
	if filepath.IsAbs(artifactDir) {
		return artifactDir
	}
	return filepath.Join(baseDir, artifactDir)
}

func stringField(value map[string]interface{}, key string) string {
	raw, ok := value[key]
	if !ok || raw == nil {
		return ""
	}
	switch typed := raw.(type) {
	case string:
		return typed
	default:
		return fmt.Sprint(typed)
	}
}

func stringSliceField(value map[string]interface{}, key string) []string {
	raw, ok := value[key]
	if !ok || raw == nil {
		return nil
	}
	items, ok := raw.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(items))
	for _, item := range items {
		if item != nil {
			out = append(out, fmt.Sprint(item))
		}
	}
	return out
}

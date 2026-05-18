package debugview

import (
	"encoding/json"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestBuildSnapshotMapsInstructionsToSourceLines(t *testing.T) {
	source := `package demo

func add(a int, b int) int {
	c := a + b
	return c
}
`
	path := writeTempSource(t, source)
	snapshot, err := BuildSnapshot(path)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if snapshot.SourceName != "input.go" {
		t.Fatalf("unexpected source name: %s", snapshot.SourceName)
	}
	if snapshot.Summary.TotalInstructions == 0 || snapshot.Summary.LocatedInstructions == 0 {
		t.Fatalf("expected located instructions, got %+v", snapshot.Summary)
	}
	if snapshot.SourceLines[3].InstructionCount == 0 {
		t.Fatalf("expected source line 4 to map to at least one instruction: %+v", snapshot.SourceLines)
	}
}

func TestBuildSnapshotParsesScopes(t *testing.T) {
	path := filepath.Join("..", "gofrontend", "testdata", "goeq_scope_locations.go")
	snapshot, err := BuildSnapshot(path)
	if err != nil {
		t.Fatalf("BuildSnapshot: %v", err)
	}
	if len(snapshot.Scopes) < 2 {
		t.Fatalf("expected function and if scopes, got %+v", snapshot.Scopes)
	}
	if snapshot.Summary.TotalScopes != len(snapshot.Scopes) {
		t.Fatalf("summary scope count mismatch: %+v", snapshot.Summary)
	}
	var sawIf bool
	for _, scope := range snapshot.Scopes {
		if scope.Kind == "if" && scope.Label == "scope1" {
			sawIf = true
			if scope.Parent != 0 || scope.Line == 0 || scope.InstructionCount == 0 {
				t.Fatalf("unexpected if scope: %+v", scope)
			}
		}
	}
	if !sawIf {
		t.Fatalf("missing if scope: %+v", snapshot.Scopes)
	}
}

func TestServerServesDebugJSON(t *testing.T) {
	snapshot := Snapshot{
		SourcePath:  "input.go",
		SourceName:  "input.go",
		SourceLines: []SourceLine{{Number: 1, Text: "package demo"}},
		Instructions: []Instruction{
			{Index: 0, Text: "module {", Kind: "module"},
		},
		Summary: Summary{TotalInstructions: 1},
	}
	server := httptest.NewServer(NewServer(snapshot))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/debug.json")
	if err != nil {
		t.Fatalf("GET debug.json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var got Snapshot
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.SourceName != snapshot.SourceName || got.Summary.TotalInstructions != 1 {
		t.Fatalf("unexpected snapshot: %+v", got)
	}
}

func TestServerServesTraceJSON(t *testing.T) {
	snapshot := Snapshot{
		SourcePath: "input.go",
		SourceName: "input.go",
		Trace: &TraceSnapshot{
			Status: "blocked",
			Paths:  []TracePath{{ID: "case-a", Status: "blocked"}},
		},
	}
	server := httptest.NewServer(NewServer(snapshot))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/trace.json")
	if err != nil {
		t.Fatalf("GET trace.json: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("unexpected status: %d", resp.StatusCode)
	}
	var got TraceSnapshot
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if got.Status != "blocked" || len(got.Paths) != 1 {
		t.Fatalf("unexpected trace: %+v", got)
	}
}

func TestLoadTraceFromProbeSummary(t *testing.T) {
	tmpDir := t.TempDir()
	caseDir := filepath.Join(tmpDir, "case-a")
	if err := os.MkdirAll(caseDir, 0o755); err != nil {
		t.Fatalf("mkdir case dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(caseDir, "case.json"), []byte(`{
  "inputs": [{"name": "x", "type": "int"}]
}`), 0o644); err != nil {
		t.Fatalf("write case json: %v", err)
	}
	summaryPath := filepath.Join(tmpDir, "summary.json")
	if err := os.WriteFile(summaryPath, []byte(`{
  "status": "blocked",
  "results": [{
    "case": "case-a",
    "function": "diffcase.F",
    "expected_status": "counterexample",
    "artifact_dir": "case-a",
    "status": "blocked",
    "first_blocker": "klee_run_not_requested",
    "missing_work": ["rerun with --run-klee"],
    "sides": [
      {
        "label": "old",
        "source": "old.go",
        "mlse_go_status": "success",
        "mlse_opt_roundtrip_status": "success",
        "llvm_as_status": "success",
        "bitcode": "old.bc"
      },
      {
        "label": "new",
        "source": "new.go",
        "mlse_go_status": "success",
        "mlse_opt_roundtrip_status": "failure",
        "mlse_opt_roundtrip_reason": "roundtrip failed"
      }
    ]
  }]
}`), 0o644); err != nil {
		t.Fatalf("write summary json: %v", err)
	}

	trace, err := LoadTrace(summaryPath)
	if err != nil {
		t.Fatalf("LoadTrace: %v", err)
	}
	if trace.Status != "blocked" || len(trace.Paths) != 1 {
		t.Fatalf("unexpected trace: %+v", trace)
	}
	path := trace.Paths[0]
	if path.Case != "case-a" || path.Expected != "counterexample" || len(path.Inputs) != 1 {
		t.Fatalf("unexpected path: %+v", path)
	}
	if len(path.Frames) != 2 || path.Frames[1].Status != "failure" {
		t.Fatalf("unexpected frames: %+v", path.Frames)
	}
	if len(path.Events) == 0 {
		t.Fatalf("expected trace events")
	}
}

func writeTempSource(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

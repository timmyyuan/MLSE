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

func writeTempSource(t *testing.T, source string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "input.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

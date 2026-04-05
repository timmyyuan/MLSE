package gofrontend

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

func TestCompileFileFormalEmitsScopeLocations(t *testing.T) {
	path := filepath.Join("testdata", "goeq_scope_locations.go")

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture %q: %v", path, err)
	}

	wantSubstrings := []string{
		`module attributes {go.scope_table = [`,
		`label = "scope0"`,
		`kind = "func"`,
		`name = "demo.check"`,
		`label = "scope1"`,
		`kind = "if"`,
		`attributes {go.scope = 0 : i64}`,
		`loc("scope0"("testdata/goeq_scope_locations.go":`,
		`loc("scope1"("testdata/goeq_scope_locations.go":`,
		`go.neq %err, %nil`,
		`go.neq %name, %str`,
	}
	for _, want := range wantSubstrings {
		if !strings.Contains(got, want) {
			t.Fatalf("compiled output missing %q\n%s", want, got)
		}
	}
}

func TestCompileFileFormalHonorsSourceDisplayOverride(t *testing.T) {
	tmpDir := t.TempDir()
	stagedPath := filepath.Join(tmpDir, "gopath", "src", "example.com", "demo", "origin.go")
	if err := os.MkdirAll(filepath.Dir(stagedPath), 0o755); err != nil {
		t.Fatalf("mkdir staged source: %v", err)
	}
	if err := os.WriteFile(stagedPath, []byte("package demo\n\nfunc Target(xs []int) []int {\n\treturn xs\n}\n"), 0o644); err != nil {
		t.Fatalf("write staged source: %v", err)
	}

	t.Setenv(formalSourceDisplayPathEnv, "../gobench-eq/dataset/cases/goeq-spec-9999/prog_a/origin.go")
	got, err := CompileFileFormal(stagedPath)
	if err != nil {
		t.Fatalf("compile staged fixture %q: %v", stagedPath, err)
	}

	if !strings.Contains(got, `../gobench-eq/dataset/cases/goeq-spec-9999/prog_a/origin.go`) {
		t.Fatalf("compiled output missing overridden source path\n%s", got)
	}
	if strings.Contains(got, `/gopath/src/`) {
		t.Fatalf("compiled output still contains staged GOPATH path\n%s", got)
	}
}

func TestCompileFileFormalEmitsEmptyScopeTableForNoFuncFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "nofunc.go")
	if err := os.WriteFile(path, []byte("package demo\n\nconst X = 1\n"), 0o644); err != nil {
		t.Fatalf("write nofunc fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile nofunc fixture %q: %v", path, err)
	}
	if !strings.Contains(got, `module attributes {go.scope_table = []} {`) {
		t.Fatalf("compiled output missing empty scope table\n%s", got)
	}
}

func TestCompileFileFormalTagsSliceBuiltinsWithLocations(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "slice_ops.go")
	source := `package demo

func Target(xs []int) []int {
	ys := make([]int, len(xs))
	ys = append(ys, xs[0])
	return ys
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write slice fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile slice fixture %q: %v", path, err)
	}

	for _, pattern := range []string{
		`(?m)^\s*%[A-Za-z0-9_]+ = go\.len .+ loc\(`,
		`(?m)^\s*%[A-Za-z0-9_]+ = go\.make_slice .+ loc\(`,
		`(?m)^\s*%[A-Za-z0-9_]+ = go\.append .+ loc\(`,
	} {
		if !regexp.MustCompile(pattern).MatchString(got) {
			t.Fatalf("compiled output missing location for pattern %q\n%s", pattern, got)
		}
	}
}

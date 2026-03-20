package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compareCompileWithGolden(t *testing.T, input string, golden string) {
	t.Helper()

	got, err := compileFile(input)
	if err != nil {
		t.Fatalf("compileFile(%q): %v", input, err)
	}

	wantBytes, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden %q: %v", golden, err)
	}

	if got != string(wantBytes) {
		t.Fatalf("unexpected MLIR-like output\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
	}
}

func TestCompileFileSimpleAddGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "simple_add.go")
	golden := filepath.Join(root, "testdata", "simple_add.mlir")

	compareCompileWithGolden(t, input, golden)
}

func TestCompileFileAcceptsPreviouslyUnsupportedStatement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unsupported.go")
	source := `package demo

func f() int {
	var x int
	if true {
		x = 1
	}
	return x
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := compileFile(path)
	if err != nil {
		t.Fatalf("compileFile returned error: %v", err)
	}
	if !strings.Contains(got, "mlse.if") {
		t.Fatalf("expected mlse.if placeholder in output:\n%s", got)
	}
}

func TestCompileFileEmitsCompareConditionForIf(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "sign_if.go")

	got, err := compileFile(input)
	if err != nil {
		t.Fatalf("compileFile(%q): %v", input, err)
	}
	if !strings.Contains(got, "mlse.if arith.cmpi_gt %x, 0 : i32 {") {
		t.Fatalf("expected compare-backed mlse.if output:\n%s", got)
	}
}

func TestCompileFileEmitsElseBlock(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "choose_if_else.go")

	got, err := compileFile(input)
	if err != nil {
		t.Fatalf("compileFile(%q): %v", input, err)
	}
	if !strings.Contains(got, "} else {") {
		t.Fatalf("expected else block in output:\n%s", got)
	}
	if strings.Contains(got, `mlse.unsupported_stmt "BlockStmt"`) {
		t.Fatalf("unexpected unsupported else block placeholder:\n%s", got)
	}
}

func TestCompileFileChooseMergeGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "choose_merge.go")
	golden := filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.mlir")

	compareCompileWithGolden(t, input, golden)
}

func TestCompileFileSumForGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "sum_for.go")
	golden := filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.mlir")

	compareCompileWithGolden(t, input, golden)
}

func TestCompileFileSwitchValueGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "switch_value.go")
	golden := filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.mlir")

	compareCompileWithGolden(t, input, golden)
}

func TestCompileFileAcceptsPointerAndSelectorTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "types.go")
	source := `package demo

import "context"

func f(ctx context.Context, s *string) error {
	return nil
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := compileFile(path)
	if err != nil {
		t.Fatalf("compileFile returned error: %v", err)
	}
	if !strings.Contains(got, "!go.sel<\"context.Context\">") {
		t.Fatalf("expected selector type placeholder in output:\n%s", got)
	}
	if !strings.Contains(got, "!go.ptr<!go.string>") {
		t.Fatalf("expected pointer type placeholder in output:\n%s", got)
	}
	if !strings.Contains(got, "!go.error") {
		t.Fatalf("expected error type placeholder in output:\n%s", got)
	}
}

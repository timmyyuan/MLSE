package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCompileFileSimpleAddGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "simple_add.go")
	golden := filepath.Join(root, "testdata", "simple_add.mlir")

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

func TestCompileFileRejectsUnsupportedStatement(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "unsupported.go")
	source := `package demo

func f() int {
	var x int
	return x
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	_, err := compileFile(path)
	if err == nil {
		t.Fatal("expected error for unsupported statement")
	}
}

package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckGoFilesFlagsSingleUseWrapper(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "demo", "wrapper.go")
	writeGoTestFile(t, path, `package demo

func use() string {
	return wrapper(7)
}

func wrapper(v int) string {
	return callee(v)
}

func callee(v int) string {
	if v == 0 {
		return "0"
	}
	return "1"
}
`)

	violations := runGoMetricsCheck(t, root, path)
	if len(violations) != 1 {
		t.Fatalf("expected 1 violation, got %d: %#v", len(violations), violations)
	}
	if !strings.Contains(violations[0].msg, `function "wrapper" is a single-use wrapper around single-use callee "callee"`) {
		t.Fatalf("unexpected violation message: %s", violations[0].msg)
	}
}

func TestCheckGoFilesAllowsReusedCallee(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, "internal", "demo", "wrapper.go")
	writeGoTestFile(t, path, `package demo

func use() string {
	return wrapper(7)
}

func also(v int) string {
	return callee(v)
}

func wrapper(v int) string {
	return callee(v)
}

func callee(v int) string {
	if v == 0 {
		return "0"
	}
	return "1"
}
`)

	violations := runGoMetricsCheck(t, root, path)
	if len(violations) != 0 {
		t.Fatalf("expected no violations, got %#v", violations)
	}
}

func runGoMetricsCheck(t *testing.T, root string, path string) []violation {
	t.Helper()

	violations, err := checkGoFiles(root, []string{path}, 5, 200, 2000)
	if err != nil {
		t.Fatalf("checkGoFiles failed: %v", err)
	}
	return violations
}

func writeGoTestFile(t *testing.T, path string, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

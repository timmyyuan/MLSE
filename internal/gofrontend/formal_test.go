package gofrontend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeFormalTestSource(t *testing.T, source string) string {
	t.Helper()

	dir := t.TempDir()
	path := filepath.Join(dir, "input.go")
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}
	return path
}

func TestCompileFileDefaultsToFormalOutput(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func add(a int, b int) int {
	c := a + b
	return c
}
`)

	got, err := CompileFile(path)
	if err != nil {
		t.Fatalf("CompileFile(%q): %v", path, err)
	}
	if strings.Contains(got, "mlse.") {
		t.Fatalf("expected default frontend output to avoid legacy mlse.* placeholders:\n%s", got)
	}
	if !strings.Contains(got, "func.func @add(%a: i32, %b: i32) -> i32") {
		t.Fatalf("expected formal function signature:\n%s", got)
	}
	if !strings.Contains(got, "arith.addi %a, %b : i32") {
		t.Fatalf("expected arithmetic lowering in formal output:\n%s", got)
	}
}

func TestCompileFileFormalEmitsStringConstantAndCall(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

import "fmt"

func greet(name string) string {
	return fmt.Sprintf("hi %s", name)
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, `go.string_constant "hi %s" : !go.string`) {
		t.Fatalf("expected go.string_constant in formal output:\n%s", got)
	}
	if !strings.Contains(got, "func.call @fmt.Sprintf(") {
		t.Fatalf("expected direct func.call lowering for selector call:\n%s", got)
	}
}

func TestCompileFileFormalEmitsMakeSliceAndNil(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func build(n int) []int {
	return make([]int, n)
}

func fail() error {
	return nil
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, "go.make_slice") {
		t.Fatalf("expected go.make_slice in formal output:\n%s", got)
	}
	if !strings.Contains(got, "go.nil : !go.error") {
		t.Fatalf("expected go.nil for typed nil return:\n%s", got)
	}
}

func TestCompileFileFormalRebindsAssignmentsWithoutLegacyCopies(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func bump(x int) int {
	x = x + 1
	return x
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if strings.Contains(got, "%x = %") {
		t.Fatalf("expected formal output to avoid non-op SSA copy assignments:\n%s", got)
	}
	if !strings.Contains(got, "arith.addi %x, ") {
		t.Fatalf("expected arithmetic result to be returned through rebound variable state:\n%s", got)
	}
}

func TestCompileFileFormalLowersTerminatingIfToSCF(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, "scf.if") {
		t.Fatalf("expected terminating if to lower to scf.if:\n%s", got)
	}
	if strings.Contains(got, `go.todo "IfStmt"`) {
		t.Fatalf("expected terminating if to avoid generic go.todo fallback:\n%s", got)
	}
}

func TestCompileFileFormalAppendsFallbackReturnWhenNeeded(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, `go.todo "implicit_return_placeholder"`) {
		t.Fatalf("expected fallback return after unsupported control flow:\n%s", got)
	}
}

func TestCompileFileFormalMergesSimpleIfAssignments(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func choose(b bool) int {
	var x int
	if b {
		x = 1
	} else {
		x = 2
	}
	return x
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, "scf.if") {
		t.Fatalf("expected merge if to lower to scf.if:\n%s", got)
	}
	if !strings.Contains(got, "scf.yield") {
		t.Fatalf("expected merge if to yield merged value:\n%s", got)
	}
	if strings.Contains(got, `go.todo "IfStmt"`) {
		t.Fatalf("expected merge if to avoid generic go.todo fallback:\n%s", got)
	}
}

func TestCompileFileFormalLowersSimpleCountedLoopToSCFFor(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func sumTo(n int) int {
	sum := 0
	i := 0
	for i < n {
		sum = sum + i
		i = i + 1
	}
	return sum
}
`)

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("CompileFileFormal(%q): %v", path, err)
	}
	if !strings.Contains(got, "scf.for") {
		t.Fatalf("expected simple counted loop to lower to scf.for:\n%s", got)
	}
	if !strings.Contains(got, "iter_args(") {
		t.Fatalf("expected loop-carried variable to use iter_args:\n%s", got)
	}
	if strings.Contains(got, `go.todo "ForStmt"`) {
		t.Fatalf("expected simple counted loop to avoid generic go.todo fallback:\n%s", got)
	}
}

func TestCompileFileGoIRLikeRemainsAvailable(t *testing.T) {
	path := writeFormalTestSource(t, `package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return -1
}
`)

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFileGoIRLike(%q): %v", path, err)
	}
	if !strings.Contains(got, "mlse.if") {
		t.Fatalf("expected legacy GoIR-like path to remain available:\n%s", got)
	}
}

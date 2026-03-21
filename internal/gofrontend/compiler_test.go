package gofrontend

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func compareCompileGoIRLikeWithGolden(t *testing.T, input string, golden string) {
	t.Helper()

	got, err := CompileFileGoIRLike(input)
	if err != nil {
		t.Fatalf("CompileFileGoIRLike(%q): %v", input, err)
	}

	wantBytes, err := os.ReadFile(golden)
	if err != nil {
		t.Fatalf("read golden %q: %v", golden, err)
	}

	if got != string(wantBytes) {
		t.Fatalf("unexpected MLIR-like output\n--- got ---\n%s\n--- want ---\n%s", got, string(wantBytes))
	}
}

func parseFirstFunc(t *testing.T, source string) *ast.FuncDecl {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "test.go", source, parser.ParseComments)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			return fn
		}
	}
	t.Fatal("no function declaration found")
	return nil
}

func TestCompileFileSimpleAddGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "simple_add.go")
	golden := filepath.Join(root, "testdata", "simple_add.mlir")

	compareCompileGoIRLikeWithGolden(t, input, golden)
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

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if !strings.Contains(got, "mlse.if") {
		t.Fatalf("expected mlse.if placeholder in output:\n%s", got)
	}
}

func TestCompileFileEmitsCompareConditionForIf(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "sign_if.go")

	got, err := CompileFileGoIRLike(input)
	if err != nil {
		t.Fatalf("CompileFileGoIRLike(%q): %v", input, err)
	}
	if !strings.Contains(got, "mlse.if arith.cmpi_gt %x, 0 : i32 {") {
		t.Fatalf("expected compare-backed mlse.if output:\n%s", got)
	}
}

func TestCompileFileEmitsElseBlock(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "choose_if_else.go")

	got, err := CompileFileGoIRLike(input)
	if err != nil {
		t.Fatalf("CompileFileGoIRLike(%q): %v", input, err)
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

	compareCompileGoIRLikeWithGolden(t, input, golden)
}

func TestCompileFileSumForGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "sum_for.go")
	golden := filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.mlir")

	compareCompileGoIRLikeWithGolden(t, input, golden)
}

func TestCompileFileSwitchValueGolden(t *testing.T) {
	root := filepath.Join("..", "..")
	input := filepath.Join(root, "examples", "go", "switch_value.go")
	golden := filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.mlir")

	compareCompileGoIRLikeWithGolden(t, input, golden)
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

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
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

func TestCompileFileLowersRangeToFor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "range.go")
	source := `package demo

func f(xs []int) []int {
	var out []int
	for _, x := range xs {
		out = append(out, x)
	}
	return out
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if strings.Contains(got, "mlse.range") {
		t.Fatalf("expected range to be lowered to mlse.for:\n%s", got)
	}
	if !strings.Contains(got, "mlse.for arith.cmpi_lt") {
		t.Fatalf("expected lowered mlse.for loop:\n%s", got)
	}
}

func TestCompileFileHandlesSingleRhsMultiAssign(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "multi_assign.go")
	source := `package demo

func g() int { return 1 }

func f() {
	value, err := g()
	_, _ = value, err
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if strings.Contains(got, `mlse.unsupported_stmt "AssignStmt"`) {
		t.Fatalf("unexpected multi-assign placeholder:\n%s", got)
	}
	if !strings.Contains(got, "%err = mlse.zero : !go.error") {
		t.Fatalf("expected synthetic zeroing for secondary assignment result:\n%s", got)
	}
}

func TestCompileFileEmitsGotoLabelTargets(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "goto.go")
	source := `package demo

func f(flag bool) int {
label:
	if flag {
		goto label
	}
	return 0
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if !strings.Contains(got, "mlse.label @label") {
		t.Fatalf("expected label in output:\n%s", got)
	}
	if !strings.Contains(got, `mlse.branch "goto" @label`) {
		t.Fatalf("expected goto target in output:\n%s", got)
	}
}

func TestCompileFileTypeSwitchNoLongerEmitsUnsupportedStmt(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "typeswitch.go")
	source := `package demo

func f(v any) int {
	switch v.(type) {
	default:
		panic("unsupported")
	}
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if strings.Contains(got, `mlse.unsupported_stmt "TypeSwitchStmt"`) {
		t.Fatalf("unexpected TypeSwitchStmt placeholder:\n%s", got)
	}
	if !strings.Contains(got, `mlse.expr "TypeSwitchStmt" : !go.any`) {
		t.Fatalf("expected type-switch marker expr:\n%s", got)
	}
}

func TestCompileFilePreservesIndexAndSliceTypes(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "index_slice.go")
	source := `package demo

func f(xs []int, s string) (int, []int, string) {
	v := xs[0]
	a := xs[1:]
	b := s[1:]
	return v, a, b
}
`
	if err := os.WriteFile(path, []byte(source), 0o644); err != nil {
		t.Fatalf("write source: %v", err)
	}

	got, err := CompileFileGoIRLike(path)
	if err != nil {
		t.Fatalf("CompileFile returned error: %v", err)
	}
	if !strings.Contains(got, `mlse.index %xs[0] : i32`) {
		t.Fatalf("expected typed slice index result:\n%s", got)
	}
	if !strings.Contains(got, `mlse.slice %xs : !go.slice<i32>`) {
		t.Fatalf("expected typed slice expression:\n%s", got)
	}
	if !strings.Contains(got, `mlse.slice %s : !go.string`) {
		t.Fatalf("expected typed string slice expression:\n%s", got)
	}
}

func TestEmitParamsAssignsSyntheticNamesForUnnamedParams(t *testing.T) {
	fn := parseFirstFunc(t, `package demo

func f(int, string) {}
`)
	env := newEnv()
	got := emitParams(fn.Type.Params, env)

	if len(got) != 2 {
		t.Fatalf("expected 2 params, got %d (%v)", len(got), got)
	}
	if got[0] != "%arg1: i32" {
		t.Fatalf("unexpected first param: %q", got[0])
	}
	if got[1] != "%arg2: !go.string" {
		t.Fatalf("unexpected second param: %q", got[1])
	}
}

func TestEmitResultTypesExpandsNamedResults(t *testing.T) {
	fn := parseFirstFunc(t, `package demo

func f() (value int, err error) {
	return 0, nil
}
`)
	got := emitResultTypes(fn.Type.Results)

	if len(got) != 2 {
		t.Fatalf("expected 2 result types, got %d (%v)", len(got), got)
	}
	if got[0] != "i32" {
		t.Fatalf("unexpected first result type: %q", got[0])
	}
	if got[1] != "!go.error" {
		t.Fatalf("unexpected second result type: %q", got[1])
	}
}

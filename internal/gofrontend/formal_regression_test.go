package gofrontend

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompileFileFormalFunctionTypedSingleResultParses(t *testing.T) {
	const src = `package demo

func Make() func(int) bool {
	return func(x int) bool {
		return x > 0
	}
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "func_result.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	formal, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if !strings.Contains(formal, `func.func @demo.Make() -> ((i64) -> i1)`) {
		t.Fatalf("expected wrapped function result type in header:\n%s", formal)
	}

	repoRoot := findRepoRoot(t)
	tools := discoverLLVMTestTools(t, repoRoot)
	tmpDir := t.TempDir()
	formalPath := filepath.Join(tmpDir, "input.mlir")
	if err := os.WriteFile(formalPath, []byte(formal), 0o644); err != nil {
		t.Fatalf("write formal mlir: %v", err)
	}
	if _, err := runTool("", tools.mlseOpt, formalPath); err != nil {
		t.Fatalf("mlse-opt parse failed: %v\n%s", err, formal)
	}
}

func TestCompileFileFormalDuplicateInitGetsUniqueSymbols(t *testing.T) {
	const src = `package demo

func init() {}
func init() {}`

	dir := t.TempDir()
	path := filepath.Join(dir, "dup_init.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	formal, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if !strings.Contains(formal, "@demo.init(") || !strings.Contains(formal, "@demo.init__decl2(") {
		t.Fatalf("expected unique init symbols:\n%s", formal)
	}
}

func TestCompileFileFormalInitPanicIfHasNoReturningRegionTodo(t *testing.T) {
	const src = `package demo

func load() error { return nil }

func boot() {
	if err := load(); err != nil {
		panic(err)
	}
	load()
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "panic_if.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	formal, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if strings.Contains(formal, `go.todo "IfStmt_returning_region"`) {
		t.Fatalf("unexpected IfStmt_returning_region todo:\n%s", formal)
	}
}

func TestCompileFileFormalImportedPackageMultiResultFallback(t *testing.T) {
	root := t.TempDir()
	mainDir := filepath.Join(root, "src", "example.com", "demo")
	subDir := filepath.Join(mainDir, "subpkg")
	if err := os.MkdirAll(subDir, 0o755); err != nil {
		t.Fatalf("mkdir subpkg: %v", err)
	}

	subSrc := `package subpkg

type RespError int

const SUCCESS RespError = 0

func Stream() (string, RespError) {
	return "", SUCCESS
}`
	mainSrc := `package demo

import "example.com/demo/subpkg"

func Run() string {
	resp, err := subpkg.Stream()
	if err != subpkg.SUCCESS {
		return ""
	}
	return resp
}`

	subPath := filepath.Join(subDir, "subpkg.go")
	mainPath := filepath.Join(mainDir, "main.go")
	if err := os.WriteFile(subPath, []byte(subSrc), 0o644); err != nil {
		t.Fatalf("write subpkg fixture: %v", err)
	}
	if err := os.WriteFile(mainPath, []byte(mainSrc), 0o644); err != nil {
		t.Fatalf("write main fixture: %v", err)
	}

	formal, err := CompileFileFormal(mainPath)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if !strings.Contains(formal, `func.call @example.com.demo.subpkg.Stream() : () -> (!go.string, !go.named<"RespError">)`) {
		t.Fatalf("expected imported package multi-result signature in formal output:\n%s", formal)
	}
}

func TestCompileFileFormalSingleBranchReturnIfStillParses(t *testing.T) {
	const src = `package demo

func F(i any) string {
	if i == nil {
		return ""
	}
	switch i.(type) {
	default:
	}
	return ""
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "single_branch_return_if.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	formal, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if strings.Contains(formal, "scf.if %cmp") && strings.Contains(formal, "return %str") {
		t.Fatalf("unexpected direct return inside scf.if:\n%s", formal)
	}

	repoRoot := findRepoRoot(t)
	tools := discoverLLVMTestTools(t, repoRoot)
	tmpDir := t.TempDir()
	formalPath := filepath.Join(tmpDir, "input.mlir")
	if err := os.WriteFile(formalPath, []byte(formal), 0o644); err != nil {
		t.Fatalf("write formal mlir: %v", err)
	}
	if _, err := runTool("", tools.mlseOpt, formalPath); err != nil {
		t.Fatalf("mlse-opt parse failed: %v\n%s", err, formal)
	}
}

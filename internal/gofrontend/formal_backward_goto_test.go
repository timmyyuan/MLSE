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

func TestCompileFileFormalBackwardGotoTailHasNoImplicitPlaceholder(t *testing.T) {
	const src = `package demo
func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "backward_goto.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if strings.Contains(got, `go.todo "implicit_return_placeholder"`) {
		t.Fatalf("unexpected implicit return placeholder in backward goto lowering:\n%s", got)
	}
}

func TestCompileFileFormalBackwardGotoRestartFlagAvoidsUserNameCollision(t *testing.T) {
	const src = `package demo
func pick(x int) bool {
	var __mlse_goto_restart_lbl bool
	__mlse_goto_restart_lbl = true
lbl:
	x++
	if x < 2 {
		goto lbl
	}
	return __mlse_goto_restart_lbl
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "backward_goto_collision.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if !strings.Contains(got, "__mlse_goto_restart_lbl_1") {
		t.Fatalf("expected fresh synthetic restart flag name, got:\n%s", got)
	}
	if strings.Contains(got, "return %loop") {
		t.Fatalf("return unexpectedly uses synthetic loop value:\n%s", got)
	}
}

func TestCompileFileFormalMultipleBackwardGotosHaveNoTodo(t *testing.T) {
	const src = `package demo
func pick(x int) int {
l1:
	x++
	if x < 2 {
		goto l1
	}
l2:
	x++
	if x < 4 {
		goto l2
	}
	return x
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "backward_goto_multiple.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if strings.Contains(got, `go.todo "LabeledStmt"`) || strings.Contains(got, `go.todo "BranchStmt"`) {
		t.Fatalf("multiple backward gotos still left unresolved:\n%s", got)
	}
	if !strings.Contains(got, "__mlse_goto_restart_l1") || !strings.Contains(got, "__mlse_goto_restart_l2") {
		t.Fatalf("expected both restart loops to be materialized:\n%s", got)
	}
}

func TestCompileFileFormalSingleBackwardGotoLoopHasNoTodo(t *testing.T) {
	const src = `package demo
func spin() {
lbl:
	goto lbl
}`

	dir := t.TempDir()
	path := filepath.Join(dir, "backward_goto_single.go")
	if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}

	got, err := CompileFileFormal(path)
	if err != nil {
		t.Fatalf("compile formal fixture: %v", err)
	}
	if strings.Contains(got, `go.todo "LabeledStmt"`) || strings.Contains(got, `go.todo "BranchStmt"`) {
		t.Fatalf("single backward goto loop still left unresolved:\n%s", got)
	}
	if !strings.Contains(got, "__mlse_goto_restart_lbl") {
		t.Fatalf("expected restart loop to be materialized:\n%s", got)
	}
}

func TestCollectAssignedOuterNamesDeepTracksRestartFlag(t *testing.T) {
	const src = `package demo
func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "backward_goto.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	funcs := collectFormalFuncs(file)
	module := newFormalModuleContext(file, funcs, buildFormalTypeContext("backward_goto.go", fset, file))
	env := newFormalEnv(module)
	env.resultTypes = []string{"i64"}
	env.define("x", "i64")

	body := normalizeFormalTopLevelLabels(funcs[0].Body.List)
	if len(body) != 3 {
		t.Fatalf("expected restart decl + loop + exit tail, got %d statements", len(body))
	}
	if _, term := emitFormalStmt(body[0], env, nil); term {
		t.Fatal("restart decl unexpectedly terminated")
	}
	loop, ok := body[1].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected synthetic restart loop, got %T", body[1])
	}

	names := collectAssignedOuterNamesDeep(loop.Body.List, env, "")
	got := strings.Join(names, ",")
	if !strings.Contains(got, "__mlse_goto_restart_lbl") {
		t.Fatalf("restart flag not carried: %v", names)
	}
	if !strings.Contains(got, "x") {
		t.Fatalf("mutated value x not carried: %v", names)
	}
}

func TestEmitFormalLoopBodyWithBreakPreservesBackwardGotoTail(t *testing.T) {
	const src = `package demo
func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "backward_goto.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	funcs := collectFormalFuncs(file)
	module := newFormalModuleContext(file, funcs, buildFormalTypeContext("backward_goto.go", fset, file))
	env := newFormalEnv(module)
	env.resultTypes = []string{"i64"}
	env.define("x", "i64")

	body := normalizeFormalTopLevelLabels(funcs[0].Body.List)
	if _, term := emitFormalStmt(body[0], env, nil); term {
		t.Fatal("restart decl unexpectedly terminated")
	}
	if ret, ok := body[2].(*ast.ReturnStmt); !ok || len(ret.Results) != 1 {
		t.Fatalf("expected trailing return outside restart loop, got %T", body[2])
	}
	loop, ok := body[1].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected synthetic restart loop, got %T", body[1])
	}
	flagName, ok := formalSyntheticGotoRestartFlagName(loop)
	if !ok {
		t.Fatal("expected synthetic restart loop flag")
	}

	bodyEnv := env.clone()
	bodyEnv.bindValue("loop_stop_body", "%loop_stop_body", "i1")
	bodyEnv.bindValue(flagName, "%flag_body", "i1")
	bodyEnv.bindValue("x", "%x_body", "i64")
	text, _, _, term := emitFormalLoopBodyWithBreak(loop.Body.List, bodyEnv, []string{flagName, "x"}, []string{"i1", "i64"})
	if term {
		t.Fatalf("loop body unexpectedly terminated:\nbody:\n%s\nlowered:\n%s", printStmtList(t, loop.Body.List), text)
	}
	if !strings.Contains(text, "cmpi slt") {
		t.Fatalf("loop body lost goto guard comparison:\nbody:\n%s\nlowered:\n%s", printStmtList(t, loop.Body.List), text)
	}
	if !strings.Contains(text, "scf.if") {
		t.Fatalf("loop body lost structured if lowering:\nbody:\n%s\nlowered:\n%s", printStmtList(t, loop.Body.List), text)
	}
}

func TestEmitFormalLeadingIfWithBreakHandlesBackwardGotoTail(t *testing.T) {
	const src = `package demo
func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}`

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "backward_goto.go", src, 0)
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	funcs := collectFormalFuncs(file)
	module := newFormalModuleContext(file, funcs, buildFormalTypeContext("backward_goto.go", fset, file))
	env := newFormalEnv(module)
	env.resultTypes = []string{"i64"}
	env.define("x", "i64")

	body := normalizeFormalTopLevelLabels(funcs[0].Body.List)
	if _, term := emitFormalStmt(body[0], env, nil); term {
		t.Fatal("restart decl unexpectedly terminated")
	}
	if ret, ok := body[2].(*ast.ReturnStmt); !ok || len(ret.Results) != 1 {
		t.Fatalf("expected trailing return outside restart loop, got %T", body[2])
	}
	loop, ok := body[1].(*ast.ForStmt)
	if !ok {
		t.Fatalf("expected synthetic restart loop, got %T", body[1])
	}
	flagName, ok := formalSyntheticGotoRestartFlagName(loop)
	if !ok {
		t.Fatal("expected synthetic restart loop flag")
	}

	env.bindValue(flagName, "%flag_iter", "i1")
	env.bindValue("x", "%x_iter", "i64")
	if prefix, term := emitFormalStmt(loop.Body.List[0], env, nil); term || !strings.Contains(prefix, "arith.constant false") {
		t.Fatalf("unexpected flag reset lowering:\n%s", prefix)
	}
	prefix, term := emitFormalStmt(loop.Body.List[1], env, nil)
	if term || !strings.Contains(prefix, "addi") {
		t.Fatalf("unexpected prefix lowering:\n%s", prefix)
	}
	text, _, _, bodyTerm, ok := emitFormalLeadingIfWithBreak(loop.Body.List[2:], env, []string{flagName, "x"}, []string{"i1", "i64"})
	if !ok {
		t.Fatalf("leading if matcher rejected body:\n%s", printStmtList(t, loop.Body.List[2:]))
	}
	if bodyTerm {
		t.Fatalf("leading if unexpectedly terminated:\nbody:\n%s\nlowered:\n%s", printStmtList(t, loop.Body.List[2:]), text)
	}
	if !strings.Contains(text, "cmpi slt") {
		t.Fatalf("leading if lost comparison:\nbody:\n%s\nlowered:\n%s", printStmtList(t, loop.Body.List[2:]), text)
	}
}

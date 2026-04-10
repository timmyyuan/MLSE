package gofrontend

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"strings"
	"testing"
)

func TestNormalizeFormalTopLevelLabelsRewritesBackwardGotoLoop(t *testing.T) {
	const src = `package demo
func Target(x int) int {
lbl_1:
	for x = 0; x < 3; x++ {
		return x
	}
	if x != 0 {
		goto lbl_1
	}
	return x
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)

	assertNoTopLevelLabelOrGoto(t, normalized, "lbl_1")
	if got := printStmtList(t, normalized); !strings.Contains(got, "__mlse_goto_restart_lbl_1") {
		t.Fatalf("normalized body missing restart flag:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsRewritesBackwardGotoAssign(t *testing.T) {
	const src = `package demo
func Target(x int) int {
lbl_2:
	x++
	if x < 5 {
		for i := 0; i < 2; i++ {
			if i != 0 {
				goto lbl_2
			}
		}
	}
	return x
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)

	assertNoTopLevelLabelOrGoto(t, normalized, "lbl_2")
	if got := printStmtList(t, normalized); !strings.Contains(got, "__mlse_goto_restart_lbl_2") {
		t.Fatalf("normalized body missing restart flag:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsKeepsBackwardGotoTail(t *testing.T) {
	const src = `package demo
func Target(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)
	got := printStmtList(t, normalized)

	assertNoTopLevelLabelOrGoto(t, normalized, "lbl")
	if len(normalized) != 3 {
		t.Fatalf("expected restart decl + loop + exit tail, got %d statements:\n%s", len(normalized), got)
	}
	if !strings.Contains(got, "__mlse_goto_restart_lbl") {
		t.Fatalf("normalized body missing restart flag:\n%s", got)
	}
	if !strings.Contains(got, "if x < 4") {
		t.Fatalf("normalized body lost goto guard:\n%s", got)
	}
	if !strings.Contains(got, "return x") {
		t.Fatalf("normalized body lost trailing return:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsRewritesNestedBackwardGoto(t *testing.T) {
	const src = `package demo
func Target(x int) int {
	if x >= 0 {
	lbl:
		x++
		if x < 4 {
			goto lbl
		}
		return x
	}
	return 0
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)
	got := printStmtList(t, normalized)

	if strings.Contains(got, "goto lbl") {
		t.Fatalf("nested goto still present after normalization:\n%s", got)
	}
	if strings.Contains(got, "lbl:") {
		t.Fatalf("nested label still present after normalization:\n%s", got)
	}
	if !strings.Contains(got, "__mlse_goto_restart_lbl") {
		t.Fatalf("normalized nested block missing restart flag:\n%s", got)
	}
	if !strings.Contains(got, "return x") {
		t.Fatalf("normalized nested block lost trailing return:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsAvoidsUserNameCollision(t *testing.T) {
	const src = `package demo
func Target(x int) bool {
	var __mlse_goto_restart_lbl bool
	__mlse_goto_restart_lbl = true
lbl:
	x++
	if x < 2 {
		goto lbl
	}
	return __mlse_goto_restart_lbl
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)
	got := printStmtList(t, normalized)

	if !strings.Contains(got, "__mlse_goto_restart_lbl_1") {
		t.Fatalf("normalized body did not allocate fresh restart name:\n%s", got)
	}
	if strings.Contains(got, "var __mlse_goto_restart_lbl bool\nvar __mlse_goto_restart_lbl bool") {
		t.Fatalf("normalized body duplicated colliding user name:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsRewritesMultipleBackwardGotoLoops(t *testing.T) {
	const src = `package demo
func Target(x int) int {
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

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)
	got := printStmtList(t, normalized)

	assertNoTopLevelLabelOrGoto(t, normalized, "l1")
	assertNoTopLevelLabelOrGoto(t, normalized, "l2")
	if !strings.Contains(got, "__mlse_goto_restart_l1") || !strings.Contains(got, "__mlse_goto_restart_l2") {
		t.Fatalf("normalized body missing one of the restart flags:\n%s", got)
	}
}

func TestNormalizeFormalTopLevelLabelsRewritesSingleBackwardGotoLoop(t *testing.T) {
	const src = `package demo
func Target() {
lbl:
	goto lbl
}`

	body := parseNormalizedTargetBody(t, src)
	normalized := normalizeFormalTopLevelLabels(body)
	got := printStmtList(t, normalized)

	assertNoTopLevelLabelOrGoto(t, normalized, "lbl")
	if !strings.Contains(got, "__mlse_goto_restart_lbl") {
		t.Fatalf("normalized body missing restart flag:\n%s", got)
	}
}

func parseNormalizedTargetBody(t *testing.T, src string) []ast.Stmt {
	t.Helper()

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "normalize.go", src, 0)
	if err != nil {
		t.Fatalf("parse source: %v", err)
	}
	if len(file.Decls) != 1 {
		t.Fatalf("expected 1 decl, got %d", len(file.Decls))
	}
	fn, ok := file.Decls[0].(*ast.FuncDecl)
	if !ok || fn.Body == nil {
		t.Fatalf("expected function decl with body")
	}
	return fn.Body.List
}

func assertNoTopLevelLabelOrGoto(t *testing.T, stmts []ast.Stmt, label string) {
	t.Helper()

	for _, stmt := range stmts {
		if labeled, ok := stmt.(*ast.LabeledStmt); ok {
			t.Fatalf("top-level label %q still present: %T", label, labeled)
		}
		if formalStmtContainsGotoLabel(stmt, label) {
			t.Fatalf("goto %q still present after normalization", label)
		}
	}
}

func printStmtList(t *testing.T, stmts []ast.Stmt) string {
	t.Helper()

	var buf bytes.Buffer
	fset := token.NewFileSet()
	for _, stmt := range stmts {
		if err := printer.Fprint(&buf, fset, stmt); err != nil {
			t.Fatalf("print stmt: %v", err)
		}
		buf.WriteByte('\n')
	}
	return buf.String()
}

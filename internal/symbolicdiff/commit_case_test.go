package symbolicdiff

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestPrepareCaseForScalarBodyChange(t *testing.T) {
	repo := newGitRepo(t)
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return x + 1 }\n")
	oldCommit := commitAll(t, repo, "old")
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return x + 2 }\n")
	newCommit := commitAll(t, repo, "new")

	result, metadata := prepareCaseForTest(t, repo, oldCommit, newCommit, PrepareOptions{
		CaseName:       "scalar-body-change",
		ExpectedStatus: "counterexample",
	})

	if result.Function != "diffcase.F" {
		t.Fatalf("function = %q, want diffcase.F", result.Function)
	}
	if metadata.ExpectedStatus != "counterexample" || metadata.CModel == nil {
		t.Fatalf("metadata did not keep expected scalar model: %+v", metadata)
	}
	if len(result.ChangedFiles) != 1 || result.ChangedFiles[0] != "calc.go" {
		t.Fatalf("changed files = %#v, want calc.go", result.ChangedFiles)
	}
	oldSource := readCaseFile(t, result.CaseDir, "old.go")
	if !strings.Contains(oldSource, "package diffcase") || strings.Contains(oldSource, "MLSEDiffEntry") {
		t.Fatalf("unexpected old.go source:\n%s", oldSource)
	}
}

func TestPrepareCaseKeepsSplitHelper(t *testing.T) {
	repo := newGitRepo(t)
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return x + 1 }\n")
	oldCommit := commitAll(t, repo, "old")
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return inc(x) }\n\nfunc inc(x int) int { return x + 1 }\n")
	newCommit := commitAll(t, repo, "new")

	result, metadata := prepareCaseForTest(t, repo, oldCommit, newCommit, PrepareOptions{
		CaseName: "split-helper",
	})

	if metadata.Function != "diffcase.F" {
		t.Fatalf("metadata function = %q, want diffcase.F", metadata.Function)
	}
	newSource := readCaseFile(t, result.CaseDir, "new.go")
	if !strings.Contains(newSource, "func inc(x int) int") {
		t.Fatalf("new.go lost split helper:\n%s", newSource)
	}
}

func TestPrepareCaseIncludesChangedPackageHelperFile(t *testing.T) {
	repo := newGitRepo(t)
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return x + 1 }\n")
	oldCommit := commitAll(t, repo, "old")
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc F(x int) int { return inc(x) }\n")
	writeRepoFile(t, repo, "helper.go", "package sample\n\nfunc inc(x int) int { return x + 1 }\n")
	newCommit := commitAll(t, repo, "new")

	result, _ := prepareCaseForTest(t, repo, oldCommit, newCommit, PrepareOptions{
		CaseName: "split-helper-file",
	})

	newSource := readCaseFile(t, result.CaseDir, "new.go")
	if !strings.Contains(newSource, "func inc(x int) int") {
		t.Fatalf("new.go lost helper from changed package file:\n%s", newSource)
	}
}

func TestPrepareCaseAddsWrapperForRenamedEntry(t *testing.T) {
	repo := newGitRepo(t)
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc Old(x int) int { return x + 1 }\n")
	oldCommit := commitAll(t, repo, "old")
	writeRepoFile(t, repo, "calc.go", "package sample\n\nfunc New(x int) int { return x + 1 }\n")
	newCommit := commitAll(t, repo, "new")

	result, metadata := prepareCaseForTest(t, repo, oldCommit, newCommit, PrepareOptions{
		CaseName:    "renamed-entry",
		OldFunction: "Old",
		NewFunction: "New",
	})

	if metadata.Function != "diffcase.MLSEDiffEntry" {
		t.Fatalf("metadata function = %q, want wrapper", metadata.Function)
	}
	for _, name := range []string{"old.go", "new.go"} {
		source := readCaseFile(t, result.CaseDir, name)
		if !strings.Contains(source, "func MLSEDiffEntry(x int) int") {
			t.Fatalf("%s missing wrapper:\n%s", name, source)
		}
	}
}

func prepareCaseForTest(t *testing.T, repo string, oldCommit string, newCommit string, opts PrepareOptions) (CaseResult, caseMetadata) {
	t.Helper()
	opts.Repo = repo
	opts.OldCommit = oldCommit
	opts.NewCommit = newCommit
	opts.EmitDir = filepath.Join(t.TempDir(), "out")
	result, err := PrepareCase(context.Background(), opts)
	if err != nil {
		t.Fatalf("PrepareCase() error = %v", err)
	}
	var metadata caseMetadata
	data := []byte(readCaseFile(t, result.CaseDir, "case.json"))
	if err := json.Unmarshal(data, &metadata); err != nil {
		t.Fatalf("unmarshal case metadata: %v", err)
	}
	return result, metadata
}

func newGitRepo(t *testing.T) string {
	t.Helper()
	repo := t.TempDir()
	gitCmd(t, repo, "init")
	gitCmd(t, repo, "config", "user.email", "mlse@example.test")
	gitCmd(t, repo, "config", "user.name", "MLSE Test")
	return repo
}

func writeRepoFile(t *testing.T, repo string, path string, source string) {
	t.Helper()
	fullPath := filepath.Join(repo, path)
	if err := os.MkdirAll(filepath.Dir(fullPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(fullPath, []byte(source), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func commitAll(t *testing.T, repo string, message string) string {
	t.Helper()
	gitCmd(t, repo, "add", ".")
	gitCmd(t, repo, "commit", "-m", message)
	return strings.TrimSpace(gitCmd(t, repo, "rev-parse", "HEAD"))
}

func gitCmd(t *testing.T, repo string, args ...string) string {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = repo
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, strings.TrimSpace(string(out)))
	}
	return string(out)
}

func readCaseFile(t *testing.T, caseDir string, name string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(caseDir, name))
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(data)
}

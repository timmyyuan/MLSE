package gofrontend

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

func TestCompileFileWithFileCheck(t *testing.T) {
	fileCheck := discoverFileCheck(t)

	matches, err := filepath.Glob(filepath.Join("testdata", "*.go"))
	if err != nil {
		t.Fatalf("glob testdata: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one gofrontend FileCheck fixture")
	}
	sort.Strings(matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			mode := parseFixtureCompileMode(t, path)

			var (
				got string
				err error
			)
			switch mode {
			case "default":
				got, err = CompileFile(path)
			case "formal":
				got, err = CompileFileFormal(path)
			default:
				t.Fatalf("unsupported fixture compile mode %q in %s", mode, path)
			}
			if err != nil {
				t.Fatalf("compile %s fixture %q: %v", mode, path, err)
			}

			cmd := exec.Command(fileCheck, path, "--input-file=-", "--dump-input=fail")
			cmd.Stdin = strings.NewReader(got)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("FileCheck failed for %s:\n%s\ncompiled output:\n%s", path, out, got)
			}
		})
	}
}

func discoverFileCheck(t *testing.T) string {
	t.Helper()

	candidates := []string{"FileCheck"}
	candidates = append(candidates, []string{
		"/opt/homebrew/opt/llvm@20/bin/FileCheck",
		"/usr/local/opt/llvm/bin/FileCheck",
	}...)

	for _, candidate := range candidates {
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path
		}
	}

	t.Skip("FileCheck not found on PATH; skipping gofrontend FileCheck fixtures")
	return ""
}

func parseFixtureCompileMode(t *testing.T, path string) string {
	t.Helper()

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read fixture %q: %v", path, err)
	}

	mode := "formal"
	for _, line := range strings.Split(string(data), "\n") {
		trimmed := strings.TrimSpace(line)
		if !strings.HasPrefix(trimmed, "//") {
			continue
		}
		trimmed = strings.TrimSpace(strings.TrimPrefix(trimmed, "//"))
		if !strings.HasPrefix(trimmed, "MLSE-COMPILE:") {
			continue
		}
		mode = strings.TrimSpace(strings.TrimPrefix(trimmed, "MLSE-COMPILE:"))
		break
	}

	switch mode {
	case "default", "formal":
		return mode
	default:
		t.Fatalf("unsupported MLSE-COMPILE mode %q in %s", mode, path)
		return ""
	}
}

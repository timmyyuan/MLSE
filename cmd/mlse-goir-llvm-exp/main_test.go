package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureCase struct {
	name   string
	input  string
	output string
}

func translationFixtureCases(root string) []fixtureCase {
	return []fixtureCase{
		{
			name:   "simple_add",
			input:  filepath.Join(root, "testdata", "simple_add.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "simple_add.ll"),
		},
		{
			name:   "sign_if",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "sign_if.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "sign_if.ll"),
		},
		{
			name:   "choose_if_else",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "choose_if_else.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "choose_if_else.ll"),
		},
		{
			name:   "choose_merge",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.ll"),
		},
		{
			name:   "sum_for",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.ll"),
		},
		{
			name:   "switch_value",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.ll"),
		},
		{
			name:   "mmap_size",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "mmap_size.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "mmap_size.ll"),
		},
		{
			name:   "preallocate_unsupported",
			input:  filepath.Join(root, "testdata", "goir-llvm-exp", "preallocate_unsupported.mlir"),
			output: filepath.Join(root, "testdata", "goir-llvm-exp", "preallocate_unsupported.ll"),
		},
	}
}

type failureFixtureCase struct {
	name      string
	input     string
	wantError string
}

func translationFailureFixtureCases(root string) []failureFixtureCase {
	return []failureFixtureCase{
		{
			name:      "unsupported_if_condition_expression",
			input:     filepath.Join(root, "testdata", "goir-llvm-exp", "byte_order_if.mlir"),
			wantError: "unsupported if condition expression",
		},
		{
			name:      "if_branch_local_merge_requires_predeclared_local",
			input:     filepath.Join(root, "testdata", "goir-llvm-exp", "if_branch_local_merge_fail.mlir"),
			wantError: "new locals inside control flow are not supported",
		},
		{
			name:      "for_body_requires_predeclared_local",
			input:     filepath.Join(root, "testdata", "goir-llvm-exp", "for_new_local_fail.mlir"),
			wantError: "new locals inside control flow are not supported",
		},
		{
			name:      "switch_multi_case_rejected",
			input:     filepath.Join(root, "testdata", "goir-llvm-exp", "switch_multi_case_fail.mlir"),
			wantError: "only single-value switch cases are supported",
		},
	}
}

func translateFixture(t *testing.T, inputPath string) string {
	t.Helper()

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	got, err := translateModule(string(input))
	if err != nil {
		t.Fatalf("translateModule returned error: %v", err)
	}
	return got
}

func writeTempLLVMIR(t *testing.T, name string, text string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name+".ll")
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write temp LLVM IR: %v", err)
	}
	return path
}

func findTool(base string) string {
	if path, err := exec.LookPath(base); err == nil {
		return path
	}

	out, err := exec.Command("xcrun", "--find", base).CombinedOutput()
	if err == nil {
		path := strings.TrimSpace(string(out))
		if path != "" {
			return path
		}
	}

	dirs := append(
		strings.Split(os.Getenv("PATH"), string(os.PathListSeparator)),
		"/opt/homebrew/opt/llvm/bin",
		"/usr/local/opt/llvm/bin",
		"/opt/homebrew/bin",
		"/usr/local/bin",
	)
	for _, dir := range dirs {
		if path := searchToolDir(dir, base); path != "" {
			return path
		}
	}
	return ""
}

func searchToolDir(dir string, base string) string {
	if dir == "" {
		return ""
	}
	patterns := []string{
		filepath.Join(dir, base),
		filepath.Join(dir, base+"-*"),
	}
	for _, pattern := range patterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}
		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil || info.IsDir() || info.Mode()&0o111 == 0 {
				continue
			}
			return match
		}
	}
	return ""
}

func runOptVerify(optPath string, input string) ([]byte, error) {
	cmd := exec.Command(optPath, "-passes=verify", "-disable-output", input)
	out, err := cmd.CombinedOutput()
	if err == nil {
		return out, nil
	}
	if bytes.Contains(out, []byte("Unknown command line argument")) ||
		bytes.Contains(out, []byte("unknown pass name")) ||
		bytes.Contains(out, []byte("for the --passes option")) {
		cmd = exec.Command(optPath, "-verify", "-disable-output", input)
		return cmd.CombinedOutput()
	}
	return out, err
}

func TestTranslateModuleFixtures(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tc := range translationFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			got := translateFixture(t, tc.input)
			want, err := os.ReadFile(tc.output)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			if got != string(want) {
				t.Fatalf("unexpected LLVM IR\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
			}
		})
	}
}

func TestTranslatedFixturesPassDedicatedVerifierIfAvailable(t *testing.T) {
	root := filepath.Join("..", "..")
	optPath := findTool("opt")
	llvmAsPath := findTool("llvm-as")
	if optPath == "" && llvmAsPath == "" {
		t.Skip("no dedicated LLVM verifier tool available")
	}

	for _, tc := range translationFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			got := translateFixture(t, tc.input)
			llvmIR := writeTempLLVMIR(t, tc.name, got)

			if optPath != "" {
				out, err := runOptVerify(optPath, llvmIR)
				if err != nil {
					t.Fatalf("opt verification failed: %v\n%s", err, string(out))
				}
				return
			}

			cmd := exec.Command(llvmAsPath, "-o", os.DevNull, llvmIR)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("llvm-as verification failed: %v\n%s", err, string(out))
			}
		})
	}
}

func TestTranslatedFixturesCompileWithClangIfAvailable(t *testing.T) {
	root := filepath.Join("..", "..")
	clangPath := findTool("clang")
	if clangPath == "" {
		t.Skip("no clang available")
	}

	for _, tc := range translationFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			got := translateFixture(t, tc.input)
			llvmIR := writeTempLLVMIR(t, tc.name, got)
			objectPath := filepath.Join(t.TempDir(), tc.name+".o")

			cmd := exec.Command(clangPath, "-Wno-override-module", "-c", llvmIR, "-o", objectPath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("clang compile failed: %v\n%s", err, string(out))
			}
		})
	}
}

func TestTranslateModuleRejectsUnsupportedFixtures(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tc := range translationFailureFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			input, err := os.ReadFile(tc.input)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}

			_, err = translateModule(string(input))
			if err == nil {
				t.Fatal("expected translation to fail")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

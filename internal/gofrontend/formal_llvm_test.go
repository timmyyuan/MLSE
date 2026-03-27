package gofrontend

import (
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

var llvmLoweringPasses = []string{
	"--convert-scf-to-cf",
	"--convert-cf-to-llvm",
	"--convert-arith-to-llvm",
	"--convert-func-to-llvm",
	"--convert-index-to-llvm",
	"--reconcile-unrealized-casts",
}

func TestCompileFileToLLVMIRWithFileCheck(t *testing.T) {
	fileCheck := discoverFileCheck(t)
	repoRoot := findRepoRoot(t)
	tools := discoverLLVMTestTools(t, repoRoot)

	matches, err := filepath.Glob(filepath.Join("testdata", "llvm", "*.go"))
	if err != nil {
		t.Fatalf("glob llvm testdata: %v", err)
	}
	if len(matches) == 0 {
		t.Fatal("expected at least one gofrontend LLVM FileCheck fixture")
	}
	sort.Strings(matches)

	for _, path := range matches {
		path := path
		t.Run(filepath.Base(path), func(t *testing.T) {
			mode := parseFixtureCompileMode(t, path)

			var (
				formal string
				err    error
			)
			switch mode {
			case "default":
				formal, err = CompileFile(path)
			case "formal":
				formal, err = CompileFileFormal(path)
			default:
				t.Fatalf("unsupported fixture compile mode %q in %s", mode, path)
			}
			if err != nil {
				t.Fatalf("compile %s fixture %q: %v", mode, path, err)
			}

			tmpDir := t.TempDir()
			formalPath := filepath.Join(tmpDir, "input.formal.mlir")
			if err := os.WriteFile(formalPath, []byte(formal), 0o644); err != nil {
				t.Fatalf("write formal mlir: %v", err)
			}

			roundtrip, err := runTool("", tools.mlseOpt, formalPath)
			if err != nil {
				t.Fatalf("mlse-opt failed for %s: %v", path, err)
			}

			roundtripPath := filepath.Join(tmpDir, "input.roundtrip.mlir")
			if err := os.WriteFile(roundtripPath, []byte(roundtrip), 0o644); err != nil {
				t.Fatalf("write roundtrip mlir: %v", err)
			}

			lowered, err := runTool("", tools.mlseOpt, "--lower-go-bootstrap", roundtripPath)
			if err != nil {
				t.Fatalf("mlse-opt --lower-go-bootstrap failed for %s: %v", path, err)
			}

			features := classifyGoFeatures(lowered)
			if len(features) != 0 {
				t.Fatalf("fixture %s still contains unresolved go dialect syntax after go bootstrap lowering: %s\n%s", path, strings.Join(features, ", "), lowered)
			}

			lowerInputPath := filepath.Join(tmpDir, "input.lower.mlir")
			if err := os.WriteFile(lowerInputPath, []byte(lowered), 0o644); err != nil {
				t.Fatalf("write lowering input: %v", err)
			}

			args := append([]string{lowerInputPath}, llvmLoweringPasses...)
			llvmDialect, err := runTool("", tools.mlirOpt, args...)
			if err != nil {
				t.Fatalf("mlir-opt lowering failed for %s: %v", path, err)
			}

			llvmDialectPath := filepath.Join(tmpDir, "input.llvm.mlir")
			if err := os.WriteFile(llvmDialectPath, []byte(llvmDialect), 0o644); err != nil {
				t.Fatalf("write llvm dialect mlir: %v", err)
			}

			llvmIR, err := runTool("", tools.mlirTranslate, "--mlir-to-llvmir", llvmDialectPath)
			if err != nil {
				t.Fatalf("mlir-translate failed for %s: %v", path, err)
			}

			if err := verifyLLVMIR(t, tools, tmpDir, llvmIR); err != nil {
				t.Fatalf("llvm opt -Oz failed for %s: %v\n%s", path, err, llvmIR)
			}

			cmd := exec.Command(fileCheck, path, "--check-prefix=LLVM", "--input-file=-", "--dump-input=fail")
			cmd.Stdin = strings.NewReader(llvmIR)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("LLVM FileCheck failed for %s:\n%s\nLLVM IR:\n%s", path, out, llvmIR)
			}
		})
	}
}

type llvmTestTools struct {
	mlseOpt       string
	mlirOpt       string
	mlirTranslate string
	opt           string
	llvmAs        string
}

func discoverLLVMTestTools(t *testing.T, repoRoot string) llvmTestTools {
	t.Helper()

	mlseOptCandidates := []string{
		filepath.Join(repoRoot, "tmp", "cmake-mlir-build", "tools", "mlse-opt", "mlse-opt"),
		"mlse-opt",
	}

	tools := llvmTestTools{
		mlseOpt:       discoverRequiredExecutable(t, mlseOptCandidates...),
		mlirOpt:       discoverRequiredExecutable(t, "mlir-opt", "/opt/homebrew/opt/llvm@20/bin/mlir-opt", "/usr/local/opt/llvm/bin/mlir-opt"),
		mlirTranslate: discoverRequiredExecutable(t, "mlir-translate", "/opt/homebrew/opt/llvm@20/bin/mlir-translate", "/usr/local/opt/llvm/bin/mlir-translate"),
		opt:           discoverOptionalExecutable("opt", "/opt/homebrew/opt/llvm@20/bin/opt", "/usr/local/opt/llvm/bin/opt"),
		llvmAs:        discoverOptionalExecutable("llvm-as", "/opt/homebrew/opt/llvm@20/bin/llvm-as", "/usr/local/opt/llvm/bin/llvm-as"),
	}
	return tools
}

func discoverRequiredExecutable(t *testing.T, candidates ...string) string {
	t.Helper()

	if path := discoverOptionalExecutable(candidates...); path != "" {
		return path
	}
	t.Skipf("required LLVM tool not found; skipping gofrontend LLVM fixtures: %s", strings.Join(candidates, ", "))
	return ""
}

func discoverOptionalExecutable(candidates ...string) string {
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		path, err := exec.LookPath(candidate)
		if err == nil {
			return path
		}
		if filepath.IsAbs(candidate) {
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
	}
	return ""
}

func findRepoRoot(t *testing.T) string {
	t.Helper()

	dir, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			t.Fatalf("failed to locate repository root from %s", dir)
		}
		dir = parent
	}
}

func runTool(stdinText string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	if stdinText != "" {
		cmd.Stdin = strings.NewReader(stdinText)
	}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", &toolError{name: name, args: append([]string(nil), args...), output: string(out), err: err}
	}
	return string(out), nil
}

type toolError struct {
	name   string
	args   []string
	output string
	err    error
}

func (e *toolError) Error() string {
	return e.name + " " + strings.Join(e.args, " ") + ": " + e.err.Error() + "\n" + e.output
}

func verifyLLVMIR(t *testing.T, tools llvmTestTools, tmpDir string, llvmIR string) error {
	t.Helper()

	llvmPath := filepath.Join(tmpDir, "module.ll")
	if err := os.WriteFile(llvmPath, []byte(llvmIR), 0o644); err != nil {
		return err
	}

	if tools.opt != "" {
		if _, err := runTool("", tools.opt, "-Oz", "-disable-output", llvmPath); err == nil {
			return nil
		}
	}
	if tools.llvmAs != "" {
		_, err := runTool("", tools.llvmAs, "-o", os.DevNull, llvmPath)
		return err
	}
	return nil
}

func classifyGoFeatures(text string) []string {
	features := make([]string, 0, 6)
	if strings.Contains(text, "go.todo ") {
		features = append(features, "go.todo")
	}
	if strings.Contains(text, "go.todo_value") {
		features = append(features, "go.todo_value")
	}
	if strings.Contains(text, "go.make_slice") {
		features = append(features, "go.make_slice")
	}
	if strings.Contains(text, "go.string_constant") {
		features = append(features, "go.string_constant")
	}
	if strings.Contains(text, "go.nil") {
		features = append(features, "go.nil")
	}
	if strings.Contains(text, "!go.") {
		features = append(features, "!go.type")
	}
	return features
}

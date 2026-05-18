package symbolicdiff

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
)

type PipelineOptions struct {
	CasesRoot      string
	CaseName       string
	EmitDir        string
	Python         string
	PipelineScript string
	RunKLEE        bool
	ExpectStatus   string
	Tools          ToolPaths
}

type ToolPaths struct {
	MLSEGo        string
	MLSEOpt       string
	MLIROpt       string
	MLIRTranslate string
	LLVMAs        string
	LLVMLink      string
	Clang         string
	KLEE          string
}

func RunPipeline(ctx context.Context, opts PipelineOptions, stdout io.Writer, stderr io.Writer) error {
	if opts.CasesRoot == "" || opts.CaseName == "" || opts.EmitDir == "" {
		return errors.New("cases root, case name and emit dir are required")
	}
	if opts.Python == "" {
		opts.Python = "python3"
	}
	script, cwd, err := resolvePipelineScript(opts.PipelineScript)
	if err != nil {
		return err
	}
	args := []string{
		script,
		"--cases-root", opts.CasesRoot,
		"--case", opts.CaseName,
		"--emit", opts.EmitDir,
	}
	args = appendPipelineFlags(args, opts)
	cmd := exec.CommandContext(ctx, opts.Python, args...)
	cmd.Dir = cwd
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("run symbolic diff probe: %w", err)
	}
	return nil
}

func appendPipelineFlags(args []string, opts PipelineOptions) []string {
	if opts.RunKLEE {
		args = append(args, "--run-klee")
	}
	if opts.ExpectStatus != "" {
		args = append(args, "--expect-status", opts.ExpectStatus)
	}
	toolFlags := []struct {
		name  string
		value string
	}{
		{"--mlse-go-bin", opts.Tools.MLSEGo},
		{"--mlse-opt-bin", opts.Tools.MLSEOpt},
		{"--mlir-opt-bin", opts.Tools.MLIROpt},
		{"--mlir-translate-bin", opts.Tools.MLIRTranslate},
		{"--llvm-as-bin", opts.Tools.LLVMAs},
		{"--llvm-link-bin", opts.Tools.LLVMLink},
		{"--clang-bin", opts.Tools.Clang},
		{"--klee-bin", opts.Tools.KLEE},
	}
	for _, flag := range toolFlags {
		if flag.value != "" {
			args = append(args, flag.name, flag.value)
		}
	}
	return args
}

func resolvePipelineScript(configured string) (string, string, error) {
	if configured != "" {
		abs, err := filepath.Abs(configured)
		if err != nil {
			return "", "", err
		}
		return abs, filepath.Dir(filepath.Dir(abs)), nil
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "", "", err
	}
	root, err := findMLSECheckout(cwd)
	if err != nil {
		return "", "", err
	}
	return filepath.Join(root, "scripts", "mlse-diff-go-pipeline-probe.py"), root, nil
}

func findMLSECheckout(start string) (string, error) {
	dir, err := filepath.Abs(start)
	if err != nil {
		return "", err
	}
	for {
		script := filepath.Join(dir, "scripts", "mlse-diff-go-pipeline-probe.py")
		if _, err := os.Stat(script); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", errors.New("could not find MLSE checkout containing scripts/mlse-diff-go-pipeline-probe.py")
		}
		dir = parent
	}
}

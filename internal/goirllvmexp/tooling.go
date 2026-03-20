package goirllvmexp

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func translateLLVMDialectModule(input string) (string, error) {
	tool := findTool("mlir-translate")
	if tool == "" {
		return "", fmt.Errorf("required tool mlir-translate not found in PATH or common LLVM install directories")
	}

	cmd := exec.Command(tool, "--mlir-to-llvmir")
	cmd.Stdin = strings.NewReader(input)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("mlir-translate failed: %w\n%s", err, strings.TrimSpace(string(out)))
	}
	return string(out), nil
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
		"/opt/homebrew/opt/llvm@20/bin",
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

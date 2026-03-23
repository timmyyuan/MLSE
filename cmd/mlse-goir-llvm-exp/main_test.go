package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunDefaultsToMinimalSliceModel(t *testing.T) {
	input := `module {
  func.func @Target(%xs: !go.slice<i32>) -> i32 {
    %n = mlse.call %len(%xs) : i32
    return %n : i32
  }
}
`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-emit=llvm-dialect"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `!llvm.struct<(!llvm.ptr, i32)>`) {
		t.Fatalf("missing minimal slice struct in output:\n%s", stdout.String())
	}
}

func TestRunSupportsCapSliceModel(t *testing.T) {
	input := `module {
  func.func @Target(%xs: !go.slice<i32>) -> i32 {
    %n = mlse.call %cap(%xs) : i32
    return %n : i32
  }
}
`

	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-emit=llvm-dialect", "-slice-model=cap"}, strings.NewReader(input), &stdout, &stderr)
	if code != 0 {
		t.Fatalf("run returned %d: %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), `!llvm.struct<(!llvm.ptr, i32, i32)>`) {
		t.Fatalf("missing cap slice struct in output:\n%s", stdout.String())
	}
	if strings.Contains(stdout.String(), `llvm.call @cap`) {
		t.Fatalf("unexpected external cap fallback in output:\n%s", stdout.String())
	}
}

func TestRunRejectsInvalidSliceModel(t *testing.T) {
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	code := run([]string{"-slice-model=weird"}, strings.NewReader("module {\n}\n"), &stdout, &stderr)
	if code != 2 {
		t.Fatalf("run returned %d, want 2", code)
	}
	if !strings.Contains(stderr.String(), `unsupported slice model "weird"`) {
		t.Fatalf("missing invalid slice model error:\n%s", stderr.String())
	}
}

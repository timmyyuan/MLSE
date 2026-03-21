package goirllvmexp

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

type fixtureCase struct {
	name              string
	input             string
	output            string
	llvmDialectOutput string
}

func translationFixtureCases(root string) []fixtureCase {
	return []fixtureCase{
		{
			name:              "simple_add",
			input:             filepath.Join(root, "testdata", "simple_add.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "simple_add.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "simple_add.llvm.mlir"),
		},
		{
			name:              "sign_if",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "sign_if.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "sign_if.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "sign_if.llvm.mlir"),
		},
		{
			name:              "choose_if_else",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "choose_if_else.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "choose_if_else.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "choose_if_else.llvm.mlir"),
		},
		{
			name:              "choose_merge",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "choose_merge.llvm.mlir"),
		},
		{
			name:              "sum_for",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "sum_for.llvm.mlir"),
		},
		{
			name:              "switch_value",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "switch_value.llvm.mlir"),
		},
		{
			name:              "mmap_size",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "mmap_size.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "mmap_size.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "mmap_size.llvm.mlir"),
		},
		{
			name:              "preallocate_unsupported",
			input:             filepath.Join(root, "testdata", "goir-llvm-exp", "preallocate_unsupported.mlir"),
			output:            filepath.Join(root, "testdata", "goir-llvm-exp", "preallocate_unsupported.ll"),
			llvmDialectOutput: filepath.Join(root, "testdata", "goir-llvm-exp", "preallocate_unsupported.llvm.mlir"),
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
			name:      "switch_multi_case_rejected",
			input:     filepath.Join(root, "testdata", "goir-llvm-exp", "switch_multi_case_fail.mlir"),
			wantError: "only single-value switch cases are supported",
		},
	}
}

func newlySupportedFixturePaths(root string) []string {
	return []string{
		filepath.Join(root, "testdata", "goir-llvm-exp", "byte_order_if.mlir"),
		filepath.Join(root, "testdata", "goir-llvm-exp", "if_branch_local_merge_fail.mlir"),
		filepath.Join(root, "testdata", "goir-llvm-exp", "for_new_local_fail.mlir"),
	}
}

func translateFixture(t *testing.T, inputPath string) string {
	t.Helper()

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	got, err := TranslateModule(string(input))
	if err != nil {
		t.Fatalf("TranslateModule returned error: %v", err)
	}
	return got
}

func lowerFixture(t *testing.T, inputPath string) string {
	t.Helper()

	input, err := os.ReadFile(inputPath)
	if err != nil {
		t.Fatalf("read input: %v", err)
	}
	got, err := LowerToLLVMDialectModule(string(input))
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	return got
}

func writeTempText(t *testing.T, name string, ext string, text string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), name+ext)
	if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	return path
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

func TestLowerToLLVMDialectFixtures(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, tc := range translationFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			got := lowerFixture(t, tc.input)
			want, err := os.ReadFile(tc.llvmDialectOutput)
			if err != nil {
				t.Fatalf("read output: %v", err)
			}
			if got != string(want) {
				t.Fatalf("unexpected LLVM dialect MLIR\n--- got ---\n%s\n--- want ---\n%s", got, string(want))
			}
		})
	}
}

func TestLoweredFixturesParseWithMlirOptIfAvailable(t *testing.T) {
	root := filepath.Join("..", "..")
	mlirOptPath := findTool("mlir-opt")
	if mlirOptPath == "" {
		t.Skip("no mlir-opt available")
	}

	for _, tc := range translationFixtureCases(root) {
		t.Run(tc.name, func(t *testing.T) {
			got := lowerFixture(t, tc.input)
			mlirPath := writeTempText(t, tc.name, ".mlir", got)

			cmd := exec.Command(mlirOptPath, mlirPath)
			out, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("mlir-opt parse failed: %v\n%s", err, string(out))
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
			llvmIR := writeTempText(t, tc.name, ".ll", got)

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
			llvmIR := writeTempText(t, tc.name, ".ll", got)
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

			_, err = TranslateModule(string(input))
			if err == nil {
				t.Fatal("expected translation to fail")
			}
			if !strings.Contains(err.Error(), tc.wantError) {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestTranslateModuleAcceptsOpaqueFallbackFixtures(t *testing.T) {
	root := filepath.Join("..", "..")
	for _, path := range newlySupportedFixturePaths(root) {
		t.Run(filepath.Base(path), func(t *testing.T) {
			input, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read input: %v", err)
			}
			if _, err := TranslateModule(string(input)); err != nil {
				t.Fatalf("TranslateModule returned error: %v", err)
			}
		})
	}
}

func TestParseFunctionHeaderParsesMultiResult(t *testing.T) {
	line := sourceLine{
		number: 2,
		text:   `func.func @f(%ctx: !go.sel<"context.Context">) -> (i32, !go.error) {`,
	}

	fn, err := parseFunctionHeader(line)
	if err != nil {
		t.Fatalf("parseFunctionHeader returned error: %v", err)
	}
	if len(fn.results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(fn.results))
	}
	if fn.results[0] != "i32" || fn.results[1] != "!go.error" {
		t.Fatalf("unexpected results: %#v", fn.results)
	}
}

func TestSplitTopLevelHandlesNestedSelectors(t *testing.T) {
	input := `%ctx: !go.sel<"context.Context">, %repoInfo: !go.ptr<!go.sel<"commonpkg.CreateRepoResp">>, %owners: !go.slice<!go.string>`

	got, err := splitTopLevel(input)
	if err != nil {
		t.Fatalf("splitTopLevel returned error: %v", err)
	}
	if len(got) != 3 {
		t.Fatalf("expected 3 parts, got %d (%v)", len(got), got)
	}
	if got[1] != `%repoInfo: !go.ptr<!go.sel<"commonpkg.CreateRepoResp">>` {
		t.Fatalf("unexpected second part: %q", got[1])
	}
}

func TestSearchToolDirFindsVersionedExecutable(t *testing.T) {
	dir := t.TempDir()
	want := filepath.Join(dir, "mlir-opt-20")
	if err := os.WriteFile(want, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write tool: %v", err)
	}

	got := searchToolDir(dir, "mlir-opt")
	if got != want {
		t.Fatalf("expected %q, got %q", want, got)
	}
}

func TestSplitCallArgsMergesInlineArithmeticExpressions(t *testing.T) {
	got, err := splitCallArgs(`"%.1fK", arith.divsi %n, 1000`)
	if err != nil {
		t.Fatalf("splitCallArgs returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 args, got %d (%v)", len(got), got)
	}
	if got[1] != "arith.divsi %n, 1000" {
		t.Fatalf("unexpected second arg: %q", got[1])
	}
}

func TestSplitCallArgsKeepsIndexedExpressionsTogether(t *testing.T) {
	got, err := splitCallArgs(`mlse.index %w[arith.addi %i, 1], %suffix`)
	if err != nil {
		t.Fatalf("splitCallArgs returned error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 args, got %d (%v)", len(got), got)
	}
	if got[0] != "mlse.index %w[arith.addi %i, 1]" {
		t.Fatalf("unexpected first arg: %q", got[0])
	}
}

func TestLowerToLLVMDialectSupportsMultiResultFunctions(t *testing.T) {
	input := `module {
  func.func @Target(%x: i32) -> (i32, !go.error) {
    return %x, mlse.nil : i32, !go.nil
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, `llvm.func @Target(%x: i32) -> !llvm.struct<(i32, !llvm.ptr)>`) {
		t.Fatalf("missing packed struct return:\n%s", got)
	}
	if !strings.Contains(got, "llvm.insertvalue") {
		t.Fatalf("missing struct packing insertvalue:\n%s", got)
	}
}

func TestLowerToLLVMDialectCoercesOpaqueValuesToExpectedType(t *testing.T) {
	input := `module {
  func.func @Target(%x: i32) -> i32 {
    %bin1 = mlse.select %commonpkg.GlobalInput : !go.any
    %ret2 = arith.addi %x, %bin1 : i32
    return %ret2 : i32
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if strings.Contains(got, "llvm.add %load4, %load5 : i32") {
		t.Fatalf("unexpected pointer-typed add in output:\n%s", got)
	}
	if !strings.Contains(got, "llvm.add") {
		t.Fatalf("missing lowered add:\n%s", got)
	}
}

func TestLowerToLLVMDialectSplitsConflictingExternalSignatures(t *testing.T) {
	input := `module {
  func.func @Target(%n: i32) -> !go.string {
    %call1 = mlse.call mlse.select %fmt.Sprintf("%.1fK", arith.divsi %n, 1000) : !go.string
    %call2 = mlse.call mlse.select %fmt.Sprintf("%d", %n) : !go.string
    return %call2 : !go.string
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if strings.Count(got, "llvm.func @fmt.Sprintf__") < 2 {
		t.Fatalf("expected per-signature fmt.Sprintf externs:\n%s", got)
	}
}

func TestLowerToLLVMDialectAcceptsFunclitAssignments(t *testing.T) {
	input := `module {
  func.func @Target() -> !go.any {
    %funclit1 = mlse.funclit
    %call2 = mlse.call %funclit1() : !go.any
    return %call2 : !go.any
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.func @funclit1__void__ret__llvm.ptr() -> !llvm.ptr") {
		t.Fatalf("missing funclit extern declaration:\n%s", got)
	}
}

func TestLowerToLLVMDialectLowersLocalIncDec(t *testing.T) {
	input := `module {
  func.func @Target() -> i32 {
    %x = 1 : i32
    mlse.++ %x : i32
    mlse.-- %x : i32
    return %x : i32
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.add") || !strings.Contains(got, "llvm.sub") {
		t.Fatalf("missing inc/dec lowering:\n%s", got)
	}
}

func TestLowerToLLVMDialectAcceptsCallWithColonInsideString(t *testing.T) {
	input := `module {
  func.func @Target(%x: !go.named<"uint64">, %vname: !go.ptr<!go.named<"byte">>) {
    %call1 = mlse.call mlse.select %stdio.Printf("...checksum after hashing %s : %lX\n", %vname, mlse.binop__ %x, 0xFFFFFFFF) : !go.any
    mlse.expr %call1 : !go.any
    return
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.func @stdio.Printf__") {
		t.Fatalf("missing lowered external call:\n%s", got)
	}
}

func TestLowerToLLVMDialectSupportsGotoLabels(t *testing.T) {
	input := `module {
  func.func @Target(%flag: i1) -> i32 {
    mlse.label @label
    mlse.if %flag : i1 {
        mlse.branch "goto" @label
    }
    return 0 : i32
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "^label:") {
		t.Fatalf("missing named label block:\n%s", got)
	}
	if !strings.Contains(got, "llvm.br ^label") {
		t.Fatalf("missing goto branch:\n%s", got)
	}
}

func TestLowerToLLVMDialectManglesVariantInternalCallFallbacks(t *testing.T) {
	input := `module {
  func.func @platform_main_begin() {
    return
  }
  func.func @Target() {
    %call1 = mlse.call %platform_main_begin() : !go.any
    mlse.expr %call1 : !go.any
    return
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.func @platform_main_begin__void__ret__llvm.ptr() -> !llvm.ptr") {
		t.Fatalf("missing mangled extern fallback:\n%s", got)
	}
	if !strings.Contains(got, "llvm.call @platform_main_begin__void__ret__llvm.ptr()") {
		t.Fatalf("missing mangled call fallback:\n%s", got)
	}
}

func TestLowerToLLVMDialectFallsBackForUnknownValues(t *testing.T) {
	input := `module {
  func.func @Target() -> i32 {
    %x = %missing : i32
    return %x : i32
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.mlir.zero : i32") {
		t.Fatalf("missing zero fallback for unknown value:\n%s", got)
	}
}

func TestLowerToLLVMDialectAcceptsDiscardTypeChanges(t *testing.T) {
	input := `module {
  func.func @Target() {
    %_ = 0 : i32
    %_ = mlse.nil : !go.nil
    return
  }
}
`

	if _, err := LowerToLLVMDialectModule(input); err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
}

func TestLowerToLLVMDialectDropsPointerArithmeticToZero(t *testing.T) {
	input := `module {
  func.func @Target(%x: !go.any) -> !go.any {
    %ret = arith.muli %x, 24 : !go.any
    return %ret : !go.any
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if strings.Contains(got, "llvm.mul") {
		t.Fatalf("unexpected pointer multiply in output:\n%s", got)
	}
}

func TestLowerToLLVMDialectKeepsMlseLoadBasePointerTyped(t *testing.T) {
	input := `module {
  func.func @Target(%dataList: !go.ptr<!go.slice<!go.named<"byte">>>) -> !go.nil {
    %v = mlse.load %dataList : !go.named<"len">
    mlse.expr %v : !go.named<"len">
    return mlse.nil : !go.nil
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if strings.Contains(got, "llvm.load %zero") && strings.Contains(got, "-> i32") {
		t.Fatalf("unexpected integer zero used as load base:\n%s", got)
	}
}

func TestLowerToLLVMDialectNormalizesOutOfRangeIntegerLiterals(t *testing.T) {
	input := `module {
  func.func @Target() -> i32 {
    %x = 57915251962860314 : i32
    return %x : i32
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if strings.Contains(got, "57915251962860314 : i32") {
		t.Fatalf("expected out-of-range literal to be normalized:\n%s", got)
	}
	if !strings.Contains(got, "llvm.mlir.constant(") {
		t.Fatalf("missing lowered constant:\n%s", got)
	}
}

func TestLowerToLLVMDialectAddsImplicitZeroReturnForMissingTerminator(t *testing.T) {
	input := `module {
  func.func @Target(%flag: i1) -> i32 {
    mlse.if %flag : i1 {
        return 7 : i32
    }
  }
}
`

	got, err := LowerToLLVMDialectModule(input)
	if err != nil {
		t.Fatalf("LowerToLLVMDialectModule returned error: %v", err)
	}
	if !strings.Contains(got, "llvm.return %zero") && !strings.Contains(got, "llvm.return %c") {
		t.Fatalf("expected implicit fallback return in output:\n%s", got)
	}
}

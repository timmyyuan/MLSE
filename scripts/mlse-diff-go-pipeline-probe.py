#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import re
import shutil
import subprocess
import time
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CASES_ROOT = REPO_ROOT / "test" / "SymbolicDiff" / "cases"
DEFAULT_ARTIFACT_DIR = REPO_ROOT / "artifacts" / "symbolic-diff-go-pipeline-probe"
LOWERING_PASSES = [
    "--convert-scf-to-cf",
    "--convert-cf-to-llvm",
    "--convert-arith-to-llvm",
    "--convert-func-to-llvm",
    "--convert-index-to-llvm",
    "--reconcile-unrealized-casts",
]


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description="Probe current Go symbolic-diff readiness through the real MLSE pipeline."
    )
    parser.add_argument("--cases-root", default=str(DEFAULT_CASES_ROOT))
    parser.add_argument("--case", action="append", default=[])
    parser.add_argument(
        "--require-case-list",
        action="append",
        default=[],
        help="Text file containing case names that must finish without blockers and match expected_status.",
    )
    parser.add_argument("--emit", default=str(DEFAULT_ARTIFACT_DIR))
    parser.add_argument("--mlse-go-bin", default="")
    parser.add_argument("--mlse-opt-bin", default="")
    parser.add_argument("--mlir-opt-bin", default="")
    parser.add_argument("--mlir-translate-bin", default="")
    parser.add_argument("--llvm-as-bin", default="")
    parser.add_argument("--llvm-link-bin", default="")
    parser.add_argument("--clang-bin", default="")
    parser.add_argument("--klee-bin", default="")
    parser.add_argument("--run-klee", action="store_true")
    parser.add_argument("--expect-blocker", default="")
    parser.add_argument("--expect-status", default="")
    return parser.parse_args()


def resolve_path(text: str) -> Path:
    path = Path(text)
    if path.is_absolute():
        return path
    return (REPO_ROOT / path).resolve()


def discover(configured: str, candidates: list[str]) -> str | None:
    if configured:
        path = Path(configured)
        if path.is_file():
            return str(path)
        resolved = shutil.which(configured)
        return resolved
    for candidate in candidates:
        path = REPO_ROOT / candidate
        if path.is_file():
            return str(path)
        resolved = shutil.which(candidate)
        if resolved:
            return resolved
    return None


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def display_path(path: Path) -> str:
    try:
        return str(path.relative_to(REPO_ROOT))
    except ValueError:
        return str(path)


def run(cmd: list[str]) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(REPO_ROOT),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )


def write_stage(out_dir: Path, stem: str, proc: subprocess.CompletedProcess[str]) -> None:
    (out_dir / f"{stem}.stdout").write_text(proc.stdout, encoding="utf-8")
    (out_dir / f"{stem}.stderr").write_text(proc.stderr, encoding="utf-8")


def extract_reason(proc: subprocess.CompletedProcess[str]) -> str:
    text = "\n".join(part for part in (proc.stderr, proc.stdout) if part)
    lines = [line.strip() for line in text.splitlines() if line.strip()]
    if not lines:
        return f"exit status {proc.returncode}"
    interesting = [line for line in lines if "error" in line.lower() or "failed" in line.lower()]
    return (interesting or lines)[0][:500]


def collect_case_dirs(cases_root: Path, selected: list[str]) -> list[Path]:
    if selected:
        return [cases_root / name for name in selected]
    return sorted(path for path in cases_root.iterdir() if path.is_dir())


def load_case_list(path: Path) -> list[str]:
    names: list[str] = []
    for line in path.read_text(encoding="utf-8").splitlines():
        text = line.split("#", 1)[0].strip()
        if text:
            names.append(text)
    return names


def collect_required_cases(case_list_paths: list[str]) -> list[str]:
    required: list[str] = []
    seen: set[str] = set()
    for item in case_list_paths:
        for name in load_case_list(resolve_path(item)):
            if name not in seen:
                required.append(name)
                seen.add(name)
    return required


def required_case_failures(results: list[dict[str, Any]], required: list[str]) -> list[dict[str, str]]:
    by_case = {item["case"]: item for item in results}
    failures: list[dict[str, str]] = []
    for name in required:
        result = by_case.get(name)
        if result is None:
            failures.append({"case": name, "reason": "missing_from_probe"})
            continue
        blocker = result.get("first_blocker")
        if blocker:
            failures.append({"case": name, "reason": blocker})
            continue
        if result["status"] != result["expected_status"]:
            failures.append({"case": name, "reason": f"status:{result['status']}"})
    return failures


def has_unresolved_go_dialect(text: str) -> bool:
    if re.search(r"!go\.", text):
        return True
    for line in text.splitlines():
        stripped = line.strip()
        if stripped.startswith("go."):
            return True
        if re.search(r"=\s+go\.", stripped):
            return True
    return False


def run_stage(
    record: dict[str, Any],
    stage: str,
    cmd: list[str],
    out_dir: Path,
    artifact: Path | None,
) -> str | None:
    proc = run(cmd)
    write_stage(out_dir, stage, proc)
    record[f"{stage}_returncode"] = proc.returncode
    if proc.returncode != 0:
        record[f"{stage}_status"] = "failure"
        record[f"{stage}_reason"] = extract_reason(proc)
        return f"{stage}_failed"
    record[f"{stage}_status"] = "success"
    if artifact is not None:
        artifact.write_text(proc.stdout, encoding="utf-8")
    return None


def sanitize_symbol(text: str) -> str:
    return re.sub(r"[^A-Za-z0-9_]", "_", text)


def rename_llvm_module_symbols(llvm_ir: str, entry_symbol: str, entry_replacement: str, side: str) -> str:
    def replacement(match: re.Match[str]) -> str:
        quoted_symbol = match.group(1)
        plain_symbol = match.group(2)
        symbol = quoted_symbol or plain_symbol
        if symbol == entry_symbol:
            return f"@{entry_replacement}"
        return f"@__mlse_{side}_{sanitize_symbol(symbol)}"

    return re.sub(r'@"(diffcase\.[^"]+)"|@(diffcase\.[A-Za-z0-9_.$]+)', replacement, llvm_ir)


GO_ABI_RUNTIME_IR = """@.file = private unnamed_addr constant [5 x i8] c"mlse\\00"
@.mismatch = private unnamed_addr constant [9 x i8] c"mismatch\\00"
@.panic = private unnamed_addr constant [6 x i8] c"panic\\00"
@.unsupported = private unnamed_addr constant [12 x i8] c"unsupported\\00"
@.assert_suffix = private unnamed_addr constant [11 x i8] c"assert.err\\00"
@.panic_suffix = private unnamed_addr constant [10 x i8] c"panic.err\\00"
@.model_suffix = private unnamed_addr constant [10 x i8] c"model.err\\00"

declare void @klee_make_symbolic(ptr, i64, ptr)
declare void @klee_report_error(ptr, i32, ptr, ptr)
declare ptr @malloc(i64)

define i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %alen = extractvalue { ptr, i64 } %a, 1
  %blen = extractvalue { ptr, i64 } %b, 1
  %same_len = icmp eq i64 %alen, %blen
  br i1 %same_len, label %loop, label %not_equal

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %done = icmp eq i64 %i, %alen
  br i1 %done, label %equal, label %body

body:
  %adata = extractvalue { ptr, i64 } %a, 0
  %bdata = extractvalue { ptr, i64 } %b, 0
  %aptr = getelementptr i8, ptr %adata, i64 %i
  %bptr = getelementptr i8, ptr %bdata, i64 %i
  %aval = load i8, ptr %aptr, align 1
  %bval = load i8, ptr %bptr, align 1
  %same_value = icmp eq i8 %aval, %bval
  br i1 %same_value, label %continue, label %not_equal

continue:
  %next = add i64 %i, 1
  br label %loop

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_slice_string_equal({ ptr, i64, i64 } %a, { ptr, i64, i64 } %b) {
entry:
  %alen = extractvalue { ptr, i64, i64 } %a, 1
  %blen = extractvalue { ptr, i64, i64 } %b, 1
  %same_len = icmp eq i64 %alen, %blen
  br i1 %same_len, label %loop, label %not_equal

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %done = icmp eq i64 %i, %alen
  br i1 %done, label %equal, label %body

body:
  %adata = extractvalue { ptr, i64, i64 } %a, 0
  %bdata = extractvalue { ptr, i64, i64 } %b, 0
  %aptr = getelementptr { ptr, i64 }, ptr %adata, i64 %i
  %bptr = getelementptr { ptr, i64 }, ptr %bdata, i64 %i
  %aval = load { ptr, i64 }, ptr %aptr, align 8
  %bval = load { ptr, i64 }, ptr %bptr, align 8
  %same_value = call i1 @__mlse_string_equal({ ptr, i64 } %aval, { ptr, i64 } %bval)
  br i1 %same_value, label %continue, label %not_equal

continue:
  %next = add i64 %i, 1
  br label %loop

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_error_equal(ptr %a, ptr %b) {
entry:
  %a_nil = icmp eq ptr %a, null
  %b_nil = icmp eq ptr %b, null
  %both_nil = and i1 %a_nil, %b_nil
  %same_nil = icmp eq i1 %a_nil, %b_nil
  br i1 %both_nil, label %equal, label %check_non_nil

check_non_nil:
  br i1 %same_nil, label %compare_message, label %not_equal

compare_message:
  %aval = load { ptr, i64 }, ptr %a, align 8
  %bval = load { ptr, i64 }, ptr %b, align 8
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %aval, { ptr, i64 } %bval)
  br i1 %same, label %equal, label %not_equal

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define i1 @__mlse_ptr_i64_equal(ptr %a, ptr %b) {
entry:
  %a_nil = icmp eq ptr %a, null
  %b_nil = icmp eq ptr %b, null
  %both_nil = and i1 %a_nil, %b_nil
  %same_nil = icmp eq i1 %a_nil, %b_nil
  br i1 %both_nil, label %equal, label %check_non_nil

check_non_nil:
  br i1 %same_nil, label %compare_value, label %not_equal

compare_value:
  %aval = load i64, ptr %a, align 8
  %bval = load i64, ptr %b, align 8
  %same = icmp eq i64 %aval, %bval
  br i1 %same, label %equal, label %not_equal

equal:
  ret i1 true

not_equal:
  ret i1 false
}

define { ptr, i64 } @runtime.add.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %adata = extractvalue { ptr, i64 } %a, 0
  %alen = extractvalue { ptr, i64 } %a, 1
  %bdata = extractvalue { ptr, i64 } %b, 0
  %blen = extractvalue { ptr, i64 } %b, 1
  %len = add i64 %alen, %blen
  %empty = icmp eq i64 %len, 0
  %alloc_len = select i1 %empty, i64 1, i64 %len
  %buf = call ptr @malloc(i64 %alloc_len)
  br label %copy_a

copy_a:
  %ai = phi i64 [ 0, %entry ], [ %anext, %copy_a_body ]
  %a_done = icmp eq i64 %ai, %alen
  br i1 %a_done, label %copy_b, label %copy_a_body

copy_a_body:
  %asrc = getelementptr i8, ptr %adata, i64 %ai
  %aval = load i8, ptr %asrc, align 1
  %adst = getelementptr i8, ptr %buf, i64 %ai
  store i8 %aval, ptr %adst, align 1
  %anext = add i64 %ai, 1
  br label %copy_a

copy_b:
  %bi = phi i64 [ 0, %copy_a ], [ %bnext, %copy_b_body ]
  %b_done = icmp eq i64 %bi, %blen
  br i1 %b_done, label %done, label %copy_b_body

copy_b_body:
  %bsrc = getelementptr i8, ptr %bdata, i64 %bi
  %bval = load i8, ptr %bsrc, align 1
  %offset = add i64 %alen, %bi
  %bdst = getelementptr i8, ptr %buf, i64 %offset
  store i8 %bval, ptr %bdst, align 1
  %bnext = add i64 %bi, 1
  br label %copy_b

done:
  %out0 = insertvalue { ptr, i64 } undef, ptr %buf, 0
  %out1 = insertvalue { ptr, i64 } %out0, i64 %len, 1
  ret { ptr, i64 } %out1
}

define i1 @runtime.eq.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b)
  ret i1 %same
}

define i1 @runtime.neq.string({ ptr, i64 } %a, { ptr, i64 } %b) {
entry:
  %same = call i1 @__mlse_string_equal({ ptr, i64 } %a, { ptr, i64 } %b)
  %not_same = xor i1 %same, true
  ret i1 %not_same
}

define ptr @runtime.any.box.string({ ptr, i64 } %value) {
entry:
  %box = call ptr @malloc(i64 16)
  store { ptr, i64 } %value, ptr %box, align 8
  ret ptr %box
}

define ptr @runtime.any.box.i64(i64 %value) {
entry:
  %box = call ptr @malloc(i64 8)
  store i64 %value, ptr %box, align 8
  ret ptr %box
}

define ptr @runtime.any.box.f64(double %value) {
entry:
  %box = call ptr @malloc(i64 8)
  store double %value, ptr %box, align 8
  ret ptr %box
}

define { ptr, i64 } @runtime.fmt.Sprintf({ ptr, i64 } %format, { ptr, i64, i64 } %args) {
entry:
  %fmt_ptr = extractvalue { ptr, i64 } %format, 0
  %fmt_len = extractvalue { ptr, i64 } %format, 1
  br label %scan

scan:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %after = add i64 %i, 1
  %has_next = icmp ult i64 %after, %fmt_len
  br i1 %has_next, label %check, label %fallback

check:
  %ch_ptr = getelementptr i8, ptr %fmt_ptr, i64 %i
  %next_ptr = getelementptr i8, ptr %fmt_ptr, i64 %after
  %ch = load i8, ptr %ch_ptr, align 1
  %next_ch = load i8, ptr %next_ptr, align 1
  %is_percent = icmp eq i8 %ch, 37
  %is_string = icmp eq i8 %next_ch, 115
  %found = and i1 %is_percent, %is_string
  br i1 %found, label %format_string, label %maybe_unsupported

maybe_unsupported:
  %not_string = xor i1 %is_string, true
  %is_unsupported = and i1 %is_percent, %not_string
  br i1 %is_unsupported, label %unsupported, label %continue

continue:
  %next = add i64 %i, 1
  br label %scan

format_string:
  %args_len = extractvalue { ptr, i64, i64 } %args, 1
  %has_arg = icmp ugt i64 %args_len, 0
  br i1 %has_arg, label %arg_ok, label %fallback

arg_ok:
  %args_data = extractvalue { ptr, i64, i64 } %args, 0
  %arg_slot = getelementptr ptr, ptr %args_data, i64 0
  %boxed_arg = load ptr, ptr %arg_slot, align 8
  %arg = load { ptr, i64 }, ptr %boxed_arg, align 8
  %prefix0 = insertvalue { ptr, i64 } undef, ptr %fmt_ptr, 0
  %prefix1 = insertvalue { ptr, i64 } %prefix0, i64 %i, 1
  %prefix_arg = call { ptr, i64 } @runtime.add.string({ ptr, i64 } %prefix1, { ptr, i64 } %arg)
  %suffix_index = add i64 %i, 2
  %suffix_ptr = getelementptr i8, ptr %fmt_ptr, i64 %suffix_index
  %suffix_len = sub i64 %fmt_len, %suffix_index
  %suffix0 = insertvalue { ptr, i64 } undef, ptr %suffix_ptr, 0
  %suffix1 = insertvalue { ptr, i64 } %suffix0, i64 %suffix_len, 1
  %full = call { ptr, i64 } @runtime.add.string({ ptr, i64 } %prefix_arg, { ptr, i64 } %suffix1)
  ret { ptr, i64 } %full

fallback:
  ret { ptr, i64 } %format

unsupported:
  call void @klee_report_error(ptr @.file, i32 3, ptr @.unsupported, ptr @.model_suffix)
  unreachable
}

define ptr @runtime.errors.New({ ptr, i64 } %message) {
entry:
  %err = call ptr @malloc(i64 16)
  store { ptr, i64 } %message, ptr %err, align 8
  ret ptr %err
}

define ptr @runtime.fmt.Errorf({ ptr, i64 } %format, { ptr, i64, i64 } %args) {
entry:
  %args_len = extractvalue { ptr, i64, i64 } %args, 1
  %no_args = icmp eq i64 %args_len, 0
  br i1 %no_args, label %constant_error, label %unsupported

constant_error:
  %err = call ptr @runtime.errors.New({ ptr, i64 } %format)
  ret ptr %err

unsupported:
  call void @klee_report_error(ptr @.file, i32 4, ptr @.unsupported, ptr @.model_suffix)
  unreachable
}

define { ptr, i64, i64 } @runtime.makeslice(i64 %len, i64 %cap) {
entry:
  %bytes = mul i64 %cap, 8
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue { ptr, i64, i64 } undef, ptr %buf, 0
  %slice1 = insertvalue { ptr, i64, i64 } %slice0, i64 %len, 1
  %slice2 = insertvalue { ptr, i64, i64 } %slice1, i64 %cap, 2
  ret { ptr, i64, i64 } %slice2
}

define { ptr, i64, i64 } @runtime.growslice(ptr %data, i64 %new_len, i64 %old_cap, i64 %count, i64 %elem_size) {
entry:
  %bytes = mul i64 %new_len, %elem_size
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue { ptr, i64, i64 } undef, ptr %buf, 0
  %slice1 = insertvalue { ptr, i64, i64 } %slice0, i64 %new_len, 1
  %slice2 = insertvalue { ptr, i64, i64 } %slice1, i64 %new_len, 2
  ret { ptr, i64, i64 } %slice2
}

define ptr @runtime.newobject(i64 %size, i64 %align) {
entry:
  %empty = icmp eq i64 %size, 0
  %alloc_len = select i1 %empty, i64 1, i64 %size
  %obj = call ptr @malloc(i64 %alloc_len)
  ret ptr %obj
}

define ptr @runtime.composite.map({ ptr, i64 } %first, { ptr, i64 } %second, { ptr, i64 } %third) {
entry:
  %map = call ptr @malloc(i64 8)
  ret ptr %map
}

define void @runtime.panic.index(i64 %index, i64 %len) {
entry:
  call void @klee_report_error(ptr @.file, i32 1, ptr @.panic, ptr @.panic_suffix)
  unreachable
}
"""


def c_param_decl(params: list[dict[str, Any]]) -> str:
    return ", ".join(f"{item['ctype']} {item['name']}" for item in params) or "void"


def c_param_names(params: list[dict[str, Any]]) -> str:
    return ", ".join(item["name"] for item in params)


def build_scalar_klee_harness(metadata: dict[str, Any], old_symbol: str, new_symbol: str) -> str:
    model = metadata["c_model"]
    params = model["params"]
    ret = model["return_type"]
    decl = c_param_decl(params)
    names = c_param_names(params)
    declarations = "\n".join(f"  {item['ctype']} {item['name']} = 0;" for item in params)
    symbolic = "\n".join(
        f'  klee_make_symbolic(&{item["name"]}, sizeof({item["name"]}), "{item["name"]}");'
        for item in params
    )
    return f"""extern void klee_make_symbolic(void *addr, unsigned long nbytes, const char *name);
extern void klee_report_error(const char *file, int line, const char *message, const char *suffix) __attribute__((noreturn));

extern {ret} {old_symbol}({decl});
extern {ret} {new_symbol}({decl});

int main(void) {{
{declarations}
{symbolic}
  {ret} old_result = {old_symbol}({names});
  {ret} new_result = {new_symbol}({names});
  if (old_result != new_result) {{
    klee_report_error("mlse-diff-go-pipeline-probe", __LINE__, "symbolic diff mismatch", "assert.err");
  }}
  return 0;
}}
"""


def build_slice_i64_klee_harness(metadata: dict[str, Any], old_symbol: str, new_symbol: str) -> str:
    model = metadata["klee_model"]
    params = model["params"]
    if len(params) != 1 or params[0]["type"] != "slice_i64":
        raise ValueError("slice_i64 KLEE model currently supports exactly one slice_i64 parameter")
    name = params[0]["name"]
    length = int(params[0].get("length", 1))
    if length <= 0:
        raise ValueError("slice_i64 KLEE model requires a positive concrete input length")
    bytes_len = length * 8
    input_mode = params[0].get("mode", "fixed")
    if input_mode not in {"fixed", "nil_empty_full"}:
        raise ValueError(f"unsupported slice_i64 input mode {input_mode!r}")
    compare_mode = model.get("return", {}).get("compare", "logical")
    if compare_mode not in {"logical", "strict"}:
        raise ValueError(f"unsupported slice_i64 compare mode {compare_mode!r}")
    compare_func = "__mlse_slice_i64_strict_equal" if compare_mode == "strict" else "__mlse_slice_i64_equal"
    input_lines = slice_i64_input_lines(name, length, bytes_len, input_mode)
    input_text = "\n".join(input_lines)
    symbolic_name = f"{name}_data"
    symbolic_name_len = len(symbolic_name) + 1
    mode_name = f"{name}_mode"
    mode_name_len = len(mode_name) + 1
    return f"""@.name_{name} = private unnamed_addr constant [{symbolic_name_len} x i8] c"{symbolic_name}\\00"
@.name_{name}_mode = private unnamed_addr constant [{mode_name_len} x i8] c"{mode_name}\\00"
@.file = private unnamed_addr constant [5 x i8] c"mlse\\00"
@.mismatch = private unnamed_addr constant [9 x i8] c"mismatch\\00"
@.panic = private unnamed_addr constant [6 x i8] c"panic\\00"
@.assert_suffix = private unnamed_addr constant [11 x i8] c"assert.err\\00"
@.panic_suffix = private unnamed_addr constant [10 x i8] c"panic.err\\00"

declare void @klee_make_symbolic(ptr, i64, ptr)
declare void @klee_report_error(ptr, i32, ptr, ptr)
declare ptr @malloc(i64)
declare {{ ptr, i64, i64 }} @{old_symbol}({{ ptr, i64, i64 }})
declare {{ ptr, i64, i64 }} @{new_symbol}({{ ptr, i64, i64 }})

define void @runtime.panic.index(i64 %index, i64 %len) {{
entry:
  call void @klee_report_error(ptr @.file, i32 1, ptr @.panic, ptr @.panic_suffix)
  unreachable
}}

define {{ ptr, i64, i64 }} @runtime.growslice(ptr %data, i64 %new_len, i64 %old_cap, i64 %count, i64 %elem_size) {{
entry:
  %bytes = mul i64 %new_len, %elem_size
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue {{ ptr, i64, i64 }} undef, ptr %buf, 0
  %slice1 = insertvalue {{ ptr, i64, i64 }} %slice0, i64 %new_len, 1
  %slice2 = insertvalue {{ ptr, i64, i64 }} %slice1, i64 %new_len, 2
  ret {{ ptr, i64, i64 }} %slice2
}}

define {{ ptr, i64, i64 }} @runtime.makeslice(i64 %len, i64 %cap) {{
entry:
  %bytes = mul i64 %cap, 8
  %empty = icmp eq i64 %bytes, 0
  %alloc_len = select i1 %empty, i64 1, i64 %bytes
  %buf = call ptr @malloc(i64 %alloc_len)
  %slice0 = insertvalue {{ ptr, i64, i64 }} undef, ptr %buf, 0
  %slice1 = insertvalue {{ ptr, i64, i64 }} %slice0, i64 %len, 1
  %slice2 = insertvalue {{ ptr, i64, i64 }} %slice1, i64 %cap, 2
  ret {{ ptr, i64, i64 }} %slice2
}}

define i1 @__mlse_slice_i64_equal({{ ptr, i64, i64 }} %a, {{ ptr, i64, i64 }} %b) {{
entry:
  %alen = extractvalue {{ ptr, i64, i64 }} %a, 1
  %blen = extractvalue {{ ptr, i64, i64 }} %b, 1
  %same_len = icmp eq i64 %alen, %blen
  br i1 %same_len, label %loop, label %not_equal

loop:
  %i = phi i64 [ 0, %entry ], [ %next, %continue ]
  %done = icmp eq i64 %i, %alen
  br i1 %done, label %equal, label %body

body:
  %adata = extractvalue {{ ptr, i64, i64 }} %a, 0
  %bdata = extractvalue {{ ptr, i64, i64 }} %b, 0
  %aptr = getelementptr i64, ptr %adata, i64 %i
  %bptr = getelementptr i64, ptr %bdata, i64 %i
  %aval = load i64, ptr %aptr, align 8
  %bval = load i64, ptr %bptr, align 8
  %same_value = icmp eq i64 %aval, %bval
  br i1 %same_value, label %continue, label %not_equal

continue:
  %next = add i64 %i, 1
  br label %loop

equal:
  ret i1 true

not_equal:
  ret i1 false
}}

define i1 @__mlse_slice_i64_strict_equal({{ ptr, i64, i64 }} %a, {{ ptr, i64, i64 }} %b) {{
entry:
  %aptr = extractvalue {{ ptr, i64, i64 }} %a, 0
  %bptr = extractvalue {{ ptr, i64, i64 }} %b, 0
  %alen = extractvalue {{ ptr, i64, i64 }} %a, 1
  %blen = extractvalue {{ ptr, i64, i64 }} %b, 1
  %acap = extractvalue {{ ptr, i64, i64 }} %a, 2
  %bcap = extractvalue {{ ptr, i64, i64 }} %b, 2
  %anil = icmp eq ptr %aptr, null
  %bnil = icmp eq ptr %bptr, null
  %same_nil = icmp eq i1 %anil, %bnil
  %same_len = icmp eq i64 %alen, %blen
  %same_cap = icmp eq i64 %acap, %bcap
  %same_shape0 = and i1 %same_nil, %same_len
  %same_shape = and i1 %same_shape0, %same_cap
  br i1 %same_shape, label %compare_values, label %not_equal

compare_values:
  %same_values = call i1 @__mlse_slice_i64_equal({{ ptr, i64, i64 }} %a, {{ ptr, i64, i64 }} %b)
  ret i1 %same_values

not_equal:
  ret i1 false
}}

define i32 @main() {{
entry:
{input_text}
  %old_result = call {{ ptr, i64, i64 }} @{old_symbol}({{ ptr, i64, i64 }} %{name}_slice2)
  %new_result = call {{ ptr, i64, i64 }} @{new_symbol}({{ ptr, i64, i64 }} %{name}_slice2)
  %same = call i1 @{compare_func}({{ ptr, i64, i64 }} %old_result, {{ ptr, i64, i64 }} %new_result)
  br i1 %same, label %ok, label %mismatch

mismatch:
  call void @klee_report_error(ptr @.file, i32 2, ptr @.mismatch, ptr @.assert_suffix)
  unreachable

ok:
  ret i32 0
	}}
	"""


def slice_i64_input_lines(name: str, length: int, bytes_len: int, mode: str) -> list[str]:
    common = [
        f"  %{name}_data = alloca [{length} x i64], align 8",
        f"  %{name}_ptr = getelementptr [{length} x i64], ptr %{name}_data, i64 0, i64 0",
        f"  call void @klee_make_symbolic(ptr %{name}_ptr, i64 {bytes_len}, ptr @.name_{name})",
    ]
    if mode == "fixed":
        return common + [
            f"  %{name}_slice0 = insertvalue {{ ptr, i64, i64 }} undef, ptr %{name}_ptr, 0",
            f"  %{name}_slice1 = insertvalue {{ ptr, i64, i64 }} %{name}_slice0, i64 {length}, 1",
            f"  %{name}_slice2 = insertvalue {{ ptr, i64, i64 }} %{name}_slice1, i64 {length}, 2",
        ]
    return common + [
        f"  %{name}_mode_addr = alloca i8, align 1",
        f"  call void @klee_make_symbolic(ptr %{name}_mode_addr, i64 1, ptr @.name_{name}_mode)",
        f"  %{name}_mode_raw = load i8, ptr %{name}_mode_addr, align 1",
        f"  %{name}_mode = and i8 %{name}_mode_raw, 3",
        f"  %{name}_is_nil = icmp eq i8 %{name}_mode, 0",
        f"  %{name}_is_zero_len = icmp ult i8 %{name}_mode, 2",
        f"  %{name}_len = select i1 %{name}_is_zero_len, i64 0, i64 {length}",
        f"  %{name}_cap = select i1 %{name}_is_nil, i64 0, i64 {length}",
        f"  %{name}_data_or_null = select i1 %{name}_is_nil, ptr null, ptr %{name}_ptr",
        f"  %{name}_slice0 = insertvalue {{ ptr, i64, i64 }} undef, ptr %{name}_data_or_null, 0",
        f"  %{name}_slice1 = insertvalue {{ ptr, i64, i64 }} %{name}_slice0, i64 %{name}_len, 1",
        f"  %{name}_slice2 = insertvalue {{ ptr, i64, i64 }} %{name}_slice1, i64 %{name}_cap, 2",
    ]


def go_abi_llvm_type(model_type: str) -> str:
    mapping = {
        "bool": "i1",
        "error": "ptr",
        "i64": "i64",
        "ptr_i64": "ptr",
        "string": "{ ptr, i64 }",
        "slice_string": "{ ptr, i64, i64 }",
    }
    if model_type not in mapping:
        raise ValueError(f"unsupported go_llvm KLEE type {model_type!r}")
    return mapping[model_type]


def go_abi_c_string_global(name: str) -> str:
    return f'@.name_{name} = private unnamed_addr constant [{len(name) + 1} x i8] c"{name}\\00"'


def go_abi_scalar_setup(name: str, model_type: str) -> tuple[list[str], str]:
    if model_type == "i64":
        return [
            f"  %{name}_addr = alloca i64, align 8",
            f"  call void @klee_make_symbolic(ptr %{name}_addr, i64 8, ptr @.name_{name})",
            f"  %{name}_value = load i64, ptr %{name}_addr, align 8",
        ], f"%{name}_value"
    if model_type == "bool":
        return [
            f"  %{name}_addr = alloca i8, align 1",
            f"  call void @klee_make_symbolic(ptr %{name}_addr, i64 1, ptr @.name_{name})",
            f"  %{name}_raw = load i8, ptr %{name}_addr, align 1",
            f"  %{name}_bit = and i8 %{name}_raw, 1",
            f"  %{name}_value = icmp ne i8 %{name}_bit, 0",
        ], f"%{name}_value"
    raise ValueError(f"unsupported scalar go_llvm KLEE type {model_type!r}")


def go_abi_string_setup(name: str, length: int) -> tuple[list[str], str]:
    if length <= 0:
        raise ValueError("go_llvm string parameters require a positive symbolic length")
    return [
        f"  %{name}_data = alloca [{length} x i8], align 1",
        f"  %{name}_ptr = getelementptr [{length} x i8], ptr %{name}_data, i64 0, i64 0",
        f"  call void @klee_make_symbolic(ptr %{name}_ptr, i64 {length}, ptr @.name_{name})",
        f"  %{name}_str0 = insertvalue {{ ptr, i64 }} undef, ptr %{name}_ptr, 0",
        f"  %{name}_str1 = insertvalue {{ ptr, i64 }} %{name}_str0, i64 {length}, 1",
    ], f"%{name}_str1"


def go_abi_slice_string_setup(name: str, length: int) -> tuple[list[str], str]:
    if length <= 0:
        raise ValueError("go_llvm slice_string parameters require a positive symbolic length")
    lines = [
        f"  %{name}_data = alloca [{length} x {{ ptr, i64 }}], align 8",
        f"  %{name}_ptr = getelementptr [{length} x {{ ptr, i64 }}], ptr %{name}_data, i64 0, i64 0",
    ]
    for index in range(length):
        elem = f"{name}_elem{index}"
        elem_lines, elem_value = go_abi_string_setup(elem, length)
        lines.extend(elem_lines)
        lines.extend(
            [
                f"  %{elem}_slot = getelementptr [{length} x {{ ptr, i64 }}], ptr %{name}_data, i64 0, i64 {index}",
                f"  store {{ ptr, i64 }} {elem_value}, ptr %{elem}_slot, align 8",
            ]
        )
    lines.extend(
        [
            f"  %{name}_mode_addr = alloca i8, align 1",
            f"  call void @klee_make_symbolic(ptr %{name}_mode_addr, i64 1, ptr @.name_{name})",
            f"  %{name}_mode_raw = load i8, ptr %{name}_mode_addr, align 1",
            f"  %{name}_mode = and i8 %{name}_mode_raw, 3",
            f"  %{name}_is_nil = icmp eq i8 %{name}_mode, 0",
            f"  %{name}_is_zero_len = icmp ult i8 %{name}_mode, 2",
            f"  %{name}_len = select i1 %{name}_is_zero_len, i64 0, i64 {length}",
            f"  %{name}_cap = select i1 %{name}_is_nil, i64 0, i64 {length}",
            f"  %{name}_data_or_null = select i1 %{name}_is_nil, ptr null, ptr %{name}_ptr",
            f"  %{name}_slice0 = insertvalue {{ ptr, i64, i64 }} undef, ptr %{name}_data_or_null, 0",
            f"  %{name}_slice1 = insertvalue {{ ptr, i64, i64 }} %{name}_slice0, i64 %{name}_len, 1",
            f"  %{name}_slice2 = insertvalue {{ ptr, i64, i64 }} %{name}_slice1, i64 %{name}_cap, 2",
        ]
    )
    return lines, f"%{name}_slice2"


def go_abi_input_setup(param: dict[str, Any]) -> tuple[list[str], str]:
    name = sanitize_symbol(param["name"])
    model_type = param["type"]
    length = int(param.get("length", 1))
    if model_type in {"i64", "bool"}:
        return go_abi_scalar_setup(name, model_type)
    if model_type == "string":
        return go_abi_string_setup(name, length)
    if model_type == "slice_string":
        return go_abi_slice_string_setup(name, length)
    raise ValueError(f"unsupported go_llvm KLEE parameter type {model_type!r}")


def go_abi_input_globals(params: list[dict[str, Any]]) -> list[str]:
    globals_list: list[str] = []
    for param in params:
        name = sanitize_symbol(param["name"])
        globals_list.append(go_abi_c_string_global(name))
        if param["type"] == "slice_string":
            length = int(param.get("length", 1))
            for index in range(length):
                globals_list.append(go_abi_c_string_global(f"{name}_elem{index}"))
    return globals_list


def go_abi_compare_lines(return_type: str) -> list[str]:
    if return_type == "string":
        return [
            "  %same = call i1 @__mlse_string_equal({ ptr, i64 } %old_result, { ptr, i64 } %new_result)"
        ]
    if return_type == "slice_string":
        return [
            "  %same = call i1 @__mlse_slice_string_equal({ ptr, i64, i64 } %old_result, { ptr, i64, i64 } %new_result)"
        ]
    if return_type == "error":
        return [
            "  %same = call i1 @__mlse_error_equal(ptr %old_result, ptr %new_result)"
        ]
    if return_type == "ptr_i64":
        return [
            "  %same = call i1 @__mlse_ptr_i64_equal(ptr %old_result, ptr %new_result)"
        ]
    raise ValueError(f"unsupported go_llvm KLEE return type {return_type!r}")


def build_go_abi_klee_harness(metadata: dict[str, Any], old_symbol: str, new_symbol: str) -> str:
    model = metadata["klee_model"]
    params = model.get("params", [])
    return_type = model["return"]["type"]
    ret_decl = go_abi_llvm_type(return_type)
    param_decl = ", ".join(go_abi_llvm_type(item["type"]) for item in params)
    setup_lines: list[str] = []
    arg_values: list[str] = []
    for param in params:
        lines, value = go_abi_input_setup(param)
        setup_lines.extend(lines)
        arg_values.append(value)
    args = ", ".join(
        f"{go_abi_llvm_type(param['type'])} {value}" for param, value in zip(params, arg_values)
    )
    compare_lines = go_abi_compare_lines(return_type)
    globals_text = "\n".join(go_abi_input_globals(params))
    setup_text = "\n".join(setup_lines)
    compare_text = "\n".join(compare_lines)
    return f"""{globals_text}
{GO_ABI_RUNTIME_IR}

declare {ret_decl} @{old_symbol}({param_decl})
declare {ret_decl} @{new_symbol}({param_decl})

define i32 @main() {{
entry:
{setup_text}
  %old_result = call {ret_decl} @{old_symbol}({args})
  %new_result = call {ret_decl} @{new_symbol}({args})
{compare_text}
  br i1 %same, label %ok, label %mismatch

mismatch:
  call void @klee_report_error(ptr @.file, i32 2, ptr @.mismatch, ptr @.assert_suffix)
  unreachable

ok:
  ret i32 0
}}
"""


def build_klee_harness(metadata: dict[str, Any], old_symbol: str, new_symbol: str) -> tuple[str, str]:
    model = metadata.get("klee_model", {})
    if model.get("abi") == "go_llvm":
        return "llvm_ir", build_go_abi_klee_harness(metadata, old_symbol, new_symbol)
    if model.get("abi") == "slice_i64":
        return "llvm_ir", build_slice_i64_klee_harness(metadata, old_symbol, new_symbol)
    if "c_model" not in metadata:
        raise ValueError("missing c_model or supported klee_model")
    return "c", build_scalar_klee_harness(metadata, old_symbol, new_symbol)


def classify_klee_result(expected: str, klee_out: Path, proc: subprocess.CompletedProcess[str]) -> str:
    assert_errors = list(klee_out.glob("*.assert.err"))
    all_errors = list(klee_out.glob("*.err"))
    if expected == "counterexample" and assert_errors:
        return "counterexample"
    if expected == "equivalent" and not all_errors and proc.returncode == 0:
        return "equivalent"
    return "inconclusive"


def run_klee_diff(
    metadata: dict[str, Any],
    out_dir: Path,
    tools: dict[str, str | None],
) -> tuple[dict[str, Any], str | None]:
    required = ["clang", "llvm_as", "llvm_link", "klee"]
    missing = [name for name in required if not tools[name]]
    record: dict[str, Any] = {"status": "not_run"}
    if missing:
        record["missing_tools"] = missing
        return record, "klee_tool_unavailable"

    old_symbol = "__mlse_old_" + sanitize_symbol(metadata["function"])
    new_symbol = "__mlse_new_" + sanitize_symbol(metadata["function"])
    record["old_symbol"] = old_symbol
    record["new_symbol"] = new_symbol

    renamed_bitcodes = []
    for label, symbol in (("old", old_symbol), ("new", new_symbol)):
        llvm_ir_path = out_dir / f"05-{label}.ll"
        renamed_ll = out_dir / f"07-{label}.renamed.ll"
        renamed_bc = out_dir / f"08-{label}.renamed.bc"
        renamed_ll.write_text(
            rename_llvm_module_symbols(
                llvm_ir_path.read_text(encoding="utf-8"),
                metadata["function"],
                symbol,
                label,
            ),
            encoding="utf-8",
        )
        proc = run([tools["llvm_as"], str(renamed_ll), "-o", str(renamed_bc)])
        write_stage(out_dir, f"rename_{label}_llvm_as", proc)
        record[f"{label}_rename_llvm_as_returncode"] = proc.returncode
        if proc.returncode != 0:
            record["status"] = "blocked"
            record["reason"] = extract_reason(proc)
            return record, f"{label}_rename_llvm_as_failed"
        renamed_bitcodes.append(renamed_bc)

    harness_bc = out_dir / "10-klee-harness.bc"
    linked_bc = out_dir / "11-linked.bc"
    klee_out = out_dir / "klee-out"
    try:
        harness_kind, harness_text = build_klee_harness(metadata, old_symbol, new_symbol)
    except ValueError as exc:
        record["status"] = "blocked"
        record["reason"] = str(exc)
        return record, "klee_model_unavailable"
    if harness_kind == "llvm_ir":
        harness_source = out_dir / "09-klee-harness.ll"
        harness_source.write_text(harness_text, encoding="utf-8")
        proc = run([tools["llvm_as"], str(harness_source), "-o", str(harness_bc)])
        stage = "harness_llvm_as"
    else:
        harness_source = out_dir / "09-klee-harness.c"
        harness_source.write_text(harness_text, encoding="utf-8")
        proc = run([tools["clang"], "-emit-llvm", "-g", "-O0", "-c", str(harness_source), "-o", str(harness_bc)])
        stage = "harness_clang"
    write_stage(out_dir, stage, proc)
    record["harness_kind"] = harness_kind
    record[f"{stage}_returncode"] = proc.returncode
    if proc.returncode != 0:
        record["status"] = "blocked"
        record["reason"] = extract_reason(proc)
        return record, f"{stage}_failed"

    proc = run([tools["llvm_link"], str(harness_bc), *map(str, renamed_bitcodes), "-o", str(linked_bc)])
    write_stage(out_dir, "llvm_link", proc)
    record["llvm_link_returncode"] = proc.returncode
    if proc.returncode != 0:
        record["status"] = "blocked"
        record["reason"] = extract_reason(proc)
        return record, "llvm_link_failed"

    shutil.rmtree(klee_out, ignore_errors=True)
    proc = run([tools["klee"], "--output-dir", str(klee_out), str(linked_bc)])
    write_stage(out_dir, "klee", proc)
    actual = classify_klee_result(metadata["expected_status"], klee_out, proc)
    record.update(
        {
            "status": actual,
            "klee_returncode": proc.returncode,
            "klee_output_dir": str(klee_out),
            "linked_bitcode": str(linked_bc),
        }
    )
    all_errors = sorted(str(path) for path in klee_out.glob("*.err"))
    assert_errors = [path for path in all_errors if path.endswith(".assert.err")]
    if all_errors:
        record["klee_error_files"] = all_errors
    if assert_errors:
        record["counterexample_errors"] = assert_errors
    if actual != metadata["expected_status"]:
        return record, "klee_result_mismatch"
    return record, None


def probe_side(
    label: str,
    source: Path,
    out_dir: Path,
    tools: dict[str, str | None],
) -> tuple[dict[str, Any], str | None]:
    record: dict[str, Any] = {"label": label, "source": display_path(source)}
    formal = out_dir / f"01-{label}.formal.mlir"
    roundtrip = out_dir / f"02-{label}.roundtrip.mlir"
    go_lowered = out_dir / f"03-{label}.go-lowered.mlir"
    llvm_dialect = out_dir / f"04-{label}.llvm-dialect.mlir"
    llvm_ir = out_dir / f"05-{label}.ll"
    bitcode = out_dir / f"06-{label}.bc"

    required = ["mlse_go", "mlse_opt", "mlir_opt", "mlir_translate", "llvm_as"]
    missing = [name for name in required if not tools[name]]
    if missing:
        record["missing_tools"] = missing
        return record, "tool_unavailable"

    blocker = run_stage(record, "mlse_go", [tools["mlse_go"], str(source)], out_dir, formal)
    if blocker:
        return record, blocker
    blocker = run_stage(record, "mlse_opt_roundtrip", [tools["mlse_opt"], str(formal)], out_dir, roundtrip)
    if blocker:
        return record, blocker
    blocker = run_stage(
        record,
        "go_bootstrap_lower",
        [tools["mlse_opt"], "--lower-go-bootstrap", str(roundtrip)],
        out_dir,
        go_lowered,
    )
    if blocker:
        return record, blocker
    lowered_text = go_lowered.read_text(encoding="utf-8")
    if has_unresolved_go_dialect(lowered_text):
        record["go_bootstrap_lower_status"] = "blocked_unresolved_go_dialect"
        return record, "unresolved_go_dialect"
    blocker = run_stage(
        record,
        "mlir_opt_llvm",
        [tools["mlir_opt"], str(go_lowered), *LOWERING_PASSES],
        out_dir,
        llvm_dialect,
    )
    if blocker:
        return record, blocker
    blocker = run_stage(
        record,
        "mlir_translate",
        [tools["mlir_translate"], "--mlir-to-llvmir", str(llvm_dialect)],
        out_dir,
        llvm_ir,
    )
    if blocker:
        return record, blocker
    blocker = run_stage(record, "llvm_as", [tools["llvm_as"], str(llvm_ir), "-o", str(bitcode)], out_dir, None)
    if blocker:
        return record, blocker
    record["bitcode"] = str(bitcode)
    return record, None


def probe_case(case_dir: Path, emit_root: Path, tools: dict[str, str | None]) -> dict[str, Any]:
    metadata = load_json(case_dir / "case.json")
    out_dir = emit_root / metadata["name"]
    out_dir.mkdir(parents=True, exist_ok=True)
    shutil.copy2(case_dir / "case.json", out_dir / "case.json")

    result: dict[str, Any] = {
        "case": metadata["name"],
        "function": metadata["function"],
        "expected_status": metadata["expected_status"],
        "artifact_dir": str(out_dir),
        "sides": [],
    }
    expected_blocker = metadata.get("expected_blocker", "")
    if expected_blocker:
        result["expected_blocker"] = expected_blocker
    first_blocker: str | None = None
    for label in ("old", "new"):
        source = case_dir / f"{label}.go"
        shutil.copy2(source, out_dir / f"{label}.go")
        side_record, blocker = probe_side(label, source, out_dir, tools)
        result["sides"].append(side_record)
        if blocker and first_blocker is None:
            first_blocker = f"{label}_{blocker}"

    if first_blocker:
        result["status"] = "blocked"
        result["first_blocker"] = first_blocker
        result["expectation_met"] = first_blocker == expected_blocker
        return result

    if not tools["run_klee"]:
        result["status"] = "blocked"
        result["first_blocker"] = "klee_run_not_requested"
        result["missing_work"] = ["rerun with --run-klee to execute the linked same-input harness"]
        result["expectation_met"] = result["first_blocker"] == expected_blocker
        return result

    klee_record, blocker = run_klee_diff(metadata, out_dir, tools)
    result["klee_diff"] = klee_record
    if blocker:
        result["status"] = "blocked"
        result["first_blocker"] = blocker
        result["expectation_met"] = blocker == expected_blocker
        return result

    result["status"] = klee_record["status"]
    result["expectation_met"] = result["status"] == result["expected_status"]
    return result


def main() -> int:
    args = parse_args()
    started = time.time()
    emit_root = resolve_path(args.emit)
    shutil.rmtree(emit_root, ignore_errors=True)
    emit_root.mkdir(parents=True, exist_ok=True)
    tools = {
        "mlse_go": discover(args.mlse_go_bin, ["artifacts/bin/mlse-go"]),
        "mlse_opt": discover(args.mlse_opt_bin, ["tmp/cmake-mlir-build/tools/mlse-opt/mlse-opt"]),
        "mlir_opt": discover(args.mlir_opt_bin, ["mlir-opt"]),
        "mlir_translate": discover(args.mlir_translate_bin, ["mlir-translate"]),
        "llvm_as": discover(args.llvm_as_bin, ["llvm-as-16", "llvm-as"]),
        "llvm_link": discover(args.llvm_link_bin, ["llvm-link-16", "llvm-link"]),
        "clang": discover(args.clang_bin, ["clang-16", "clang"]),
        "klee": discover(args.klee_bin, ["klee"]),
        "run_klee": args.run_klee,
    }
    cases = collect_case_dirs(resolve_path(args.cases_root), args.case)
    results = [probe_case(case_dir, emit_root, tools) for case_dir in cases]
    required_cases = collect_required_cases(args.require_case_list)
    required_failures = required_case_failures(results, required_cases)
    blockers = sorted(
        {
            item["first_blocker"]
            for item in results
            if item.get("first_blocker") and item.get("first_blocker") != item.get("expected_blocker")
        }
    )
    expected_blockers = sorted(
        {
            item["first_blocker"]
            for item in results
            if item.get("first_blocker") and item.get("first_blocker") == item.get("expected_blocker")
        }
    )
    failures = [
        item["case"]
        for item in results
        if not item.get("first_blocker") and not item.get("expectation_met", item["status"] == item["expected_status"])
    ]
    status = "ok"
    if blockers:
        status = "blocked"
    elif failures or required_failures:
        status = "failure"
    summary = {
        "status": status,
        "elapsed_seconds": round(time.time() - started, 3),
        "tools": tools,
        "first_blockers": blockers,
        "expected_blockers": expected_blockers,
        "failures": failures,
        "required_cases": required_cases,
        "required_case_failures": required_failures,
        "results": results,
    }
    write_json(emit_root / "summary.json", summary)
    print(json.dumps(summary, indent=2, ensure_ascii=False))
    if args.expect_blocker and blockers != [args.expect_blocker]:
        return 1
    if args.expect_status and summary["status"] != args.expect_status:
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

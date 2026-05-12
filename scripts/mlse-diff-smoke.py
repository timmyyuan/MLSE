#!/usr/bin/env python3
from __future__ import annotations

import argparse
import json
import os
import shutil
import subprocess
import time
from pathlib import Path
from typing import Any

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CASES_ROOT = REPO_ROOT / "test" / "SymbolicDiff" / "cases"
DEFAULT_ARTIFACT_DIR = REPO_ROOT / "artifacts" / "symbolic-diff-smoke"


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Prepare MLSE symbolic-diff fixtures and optionally run a tiny KLEE "
            "toolchain smoke test for each scalar case."
        )
    )
    parser.add_argument("--cases-root", default=str(DEFAULT_CASES_ROOT))
    parser.add_argument("--case", action="append", default=[])
    parser.add_argument("--emit", default=str(DEFAULT_ARTIFACT_DIR))
    parser.add_argument("--run-klee-toolchain-smoke", action="store_true")
    parser.add_argument("--clang", default="")
    parser.add_argument("--klee", default="")
    return parser.parse_args()


def resolve_path(text: str) -> Path:
    path = Path(text)
    if path.is_absolute():
        return path
    return (REPO_ROOT / path).resolve()


def load_json(path: Path) -> dict[str, Any]:
    return json.loads(path.read_text(encoding="utf-8"))


def write_json(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(json.dumps(value, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")


def discover_tool(configured: str, names: list[str]) -> str | None:
    if configured:
        resolved = shutil.which(configured)
        if resolved:
            return resolved
        path = Path(configured)
        if path.is_file() and os.access(path, os.X_OK):
            return str(path)
        return None
    for name in names:
        resolved = shutil.which(name)
        if resolved:
            return resolved
    return None


def collect_case_dirs(cases_root: Path, selected: list[str]) -> list[Path]:
    if selected:
        return [cases_root / name for name in selected]
    return sorted(path for path in cases_root.iterdir() if path.is_dir())


def validate_case_dir(case_dir: Path) -> dict[str, Any]:
    for name in ("case.json", "old.go", "new.go"):
        if not (case_dir / name).is_file():
            raise FileNotFoundError(f"missing {name} in {case_dir}")
    return load_json(case_dir / "case.json")


def copy_case_inputs(case_dir: Path, out_dir: Path) -> None:
    out_dir.mkdir(parents=True, exist_ok=True)
    for name in ("case.json", "old.go", "new.go"):
        shutil.copy2(case_dir / name, out_dir / name)


def c_param_decl(params: list[dict[str, Any]]) -> str:
    return ", ".join(f"{item['ctype']} {item['name']}" for item in params) or "void"


def c_param_names(params: list[dict[str, Any]]) -> str:
    return ", ".join(item["name"] for item in params)


def build_c_smoke_source(metadata: dict[str, Any]) -> str:
    model = metadata["c_model"]
    params = model["params"]
    ret = model["return_type"]
    decl = c_param_decl(params)
    names = c_param_names(params)
    symbolic = []
    for item in params:
        symbolic.append(
            f'  klee_make_symbolic(&{item["name"]}, sizeof({item["name"]}), "{item["name"]}");'
        )
    declarations = "\n".join(f"  {item['ctype']} {item['name']} = 0;" for item in params)
    symbolic_block = "\n".join(symbolic)
    return f"""extern void klee_make_symbolic(void *addr, unsigned long nbytes, const char *name);
extern void klee_report_error(const char *file, int line, const char *message, const char *suffix) __attribute__((noreturn));

static {ret} oldF({decl}) {{
  return {model["old_return"]};
}}

static {ret} newF({decl}) {{
  return {model["new_return"]};
}}

int main(void) {{
{declarations}
{symbolic_block}
  {ret} old_result = oldF({names});
  {ret} new_result = newF({names});
  if (old_result != new_result) {{
    klee_report_error("mlse-diff-smoke", __LINE__, "symbolic diff mismatch", "assert.err");
  }}
  return 0;
}}
"""


def run_command(cmd: list[str], cwd: Path) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd),
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )


def classify_klee_result(expected: str, out_dir: Path, proc: subprocess.CompletedProcess[str]) -> str:
    assert_errors = list(out_dir.glob("*.assert.err"))
    all_errors = list(out_dir.glob("*.err"))
    if expected == "counterexample" and assert_errors:
        return "counterexample"
    if expected == "equivalent" and not all_errors and proc.returncode == 0:
        return "equivalent"
    return "inconclusive"


def run_klee_smoke(metadata: dict[str, Any], out_dir: Path, clang: str, klee: str) -> dict[str, Any]:
    c_path = out_dir / "klee-smoke.c"
    bc_path = out_dir / "klee-smoke.bc"
    klee_out = out_dir / "klee-out"
    c_path.write_text(build_c_smoke_source(metadata), encoding="utf-8")

    clang_proc = run_command([clang, "-emit-llvm", "-g", "-O0", "-c", str(c_path), "-o", str(bc_path)], REPO_ROOT)
    (out_dir / "clang.stdout").write_text(clang_proc.stdout, encoding="utf-8")
    (out_dir / "clang.stderr").write_text(clang_proc.stderr, encoding="utf-8")
    if clang_proc.returncode != 0:
        return {"status": "inconclusive", "reason": "clang_bitcode_failed"}

    klee_proc = run_command([klee, "--output-dir", str(klee_out), str(bc_path)], REPO_ROOT)
    (out_dir / "klee.stdout").write_text(klee_proc.stdout, encoding="utf-8")
    (out_dir / "klee.stderr").write_text(klee_proc.stderr, encoding="utf-8")
    status = classify_klee_result(metadata["expected_status"], klee_out, klee_proc)
    result: dict[str, Any] = {"status": status, "klee_output_dir": str(klee_out)}
    all_errors = sorted(str(path) for path in klee_out.glob("*.err"))
    assert_errors = [path for path in all_errors if path.endswith(".assert.err")]
    if all_errors:
        result["klee_error_files"] = all_errors
    if assert_errors:
        result["counterexample_errors"] = assert_errors
    if status == "inconclusive":
        result["reason"] = "klee_result_did_not_match_expected_smoke_status"
    return result


def prepare_case(case_dir: Path, emit_root: Path, run_klee: bool, tools: dict[str, str | None]) -> dict[str, Any]:
    metadata = validate_case_dir(case_dir)
    name = metadata["name"]
    out_dir = emit_root / name
    copy_case_inputs(case_dir, out_dir)

    result: dict[str, Any] = {
        "case": name,
        "function": metadata["function"],
        "expected_status": metadata["expected_status"],
        "artifact_dir": str(out_dir),
        "status": "prepared",
    }
    if not run_klee:
        return result
    if not tools["clang"]:
        result.update({"status": "inconclusive", "reason": "clang_not_found"})
        return result
    if not tools["klee"]:
        result.update({"status": "inconclusive", "reason": "klee_not_found"})
        return result
    if "c_model" not in metadata:
        result.update({"status": "skipped", "reason": "c_model_not_available"})
        return result
    smoke = run_klee_smoke(metadata, out_dir, tools["clang"], tools["klee"])
    result.update(smoke)
    return result


def main() -> int:
    args = parse_args()
    cases_root = resolve_path(args.cases_root)
    emit_root = resolve_path(args.emit)
    started = time.time()
    shutil.rmtree(emit_root, ignore_errors=True)
    emit_root.mkdir(parents=True, exist_ok=True)

    tools = {
        "clang": discover_tool(args.clang, ["clang-16", "clang"]),
        "klee": discover_tool(args.klee, ["klee"]),
    }
    results = [
        prepare_case(path, emit_root, args.run_klee_toolchain_smoke, tools)
        for path in collect_case_dirs(cases_root, args.case)
    ]
    summary = {
        "status": "ok" if all(item["status"] != "inconclusive" for item in results) else "inconclusive",
        "elapsed_seconds": round(time.time() - started, 3),
        "tools": tools,
        "run_klee_toolchain_smoke": args.run_klee_toolchain_smoke,
        "results": results,
    }
    write_json(emit_root / "summary.json", summary)
    print(json.dumps(summary, indent=2, ensure_ascii=False))
    if args.run_klee_toolchain_smoke and summary["status"] != "ok":
        return 1
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

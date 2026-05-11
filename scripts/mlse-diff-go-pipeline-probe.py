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


def rename_llvm_symbol(llvm_ir: str, old_symbol: str, new_symbol: str) -> str:
    return llvm_ir.replace(f'@"{old_symbol}"', f"@{new_symbol}").replace(
        f"@{old_symbol}", f"@{new_symbol}"
    )


def c_param_decl(params: list[dict[str, Any]]) -> str:
    return ", ".join(f"{item['ctype']} {item['name']}" for item in params) or "void"


def c_param_names(params: list[dict[str, Any]]) -> str:
    return ", ".join(item["name"] for item in params)


def build_klee_harness(metadata: dict[str, Any], old_symbol: str, new_symbol: str) -> str:
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
extern void klee_assert(int condition);

extern {ret} {old_symbol}({decl});
extern {ret} {new_symbol}({decl});

int main(void) {{
{declarations}
{symbolic}
  klee_assert({old_symbol}({names}) == {new_symbol}({names}));
  return 0;
}}
"""


def classify_klee_result(expected: str, klee_out: Path, proc: subprocess.CompletedProcess[str]) -> str:
    assert_errors = list(klee_out.glob("*.assert.err"))
    if expected == "counterexample" and assert_errors:
        return "counterexample"
    if expected == "equivalent" and not assert_errors and proc.returncode == 0:
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
            rename_llvm_symbol(llvm_ir_path.read_text(encoding="utf-8"), metadata["function"], symbol),
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

    harness_c = out_dir / "09-klee-harness.c"
    harness_bc = out_dir / "10-klee-harness.bc"
    linked_bc = out_dir / "11-linked.bc"
    klee_out = out_dir / "klee-out"
    harness_c.write_text(build_klee_harness(metadata, old_symbol, new_symbol), encoding="utf-8")

    proc = run([tools["clang"], "-emit-llvm", "-g", "-O0", "-c", str(harness_c), "-o", str(harness_bc)])
    write_stage(out_dir, "harness_clang", proc)
    record["harness_clang_returncode"] = proc.returncode
    if proc.returncode != 0:
        record["status"] = "blocked"
        record["reason"] = extract_reason(proc)
        return record, "harness_clang_failed"

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
    assert_errors = sorted(str(path) for path in klee_out.glob("*.assert.err"))
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
    record: dict[str, Any] = {"label": label, "source": str(source.relative_to(REPO_ROOT))}
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
        return result

    if not tools["run_klee"]:
        result["status"] = "blocked"
        result["first_blocker"] = "klee_run_not_requested"
        result["missing_work"] = ["rerun with --run-klee to execute the linked same-input harness"]
        return result

    klee_record, blocker = run_klee_diff(metadata, out_dir, tools)
    result["klee_diff"] = klee_record
    if blocker:
        result["status"] = "blocked"
        result["first_blocker"] = blocker
        return result

    result["status"] = klee_record["status"]
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
    blockers = sorted({item["first_blocker"] for item in results if item.get("first_blocker")})
    failures = [
        item["case"]
        for item in results
        if not item.get("first_blocker") and item["status"] != item["expected_status"]
    ]
    status = "ok"
    if blockers:
        status = "blocked"
    elif failures:
        status = "failure"
    summary = {
        "status": status,
        "elapsed_seconds": round(time.time() - started, 3),
        "tools": tools,
        "first_blockers": blockers,
        "failures": failures,
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

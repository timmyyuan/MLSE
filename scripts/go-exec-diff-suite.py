#!/usr/bin/env python3
from __future__ import annotations

import argparse
import difflib
import json
import os
import shutil
import subprocess
import sys
from dataclasses import dataclass
from pathlib import Path

REPO_ROOT = Path(__file__).resolve().parent.parent
DEFAULT_CASE_GLOB = "goexec-spec-*"
DEFAULT_ARTIFACT_DIR = "artifacts/go-exec-diff-suite"
DEFAULT_TMP_DIR = "tmp/go-exec-diff-suite"

LOWERING_PASSES = [
    "--convert-scf-to-cf",
    "--convert-cf-to-llvm",
    "--convert-arith-to-llvm",
    "--convert-func-to-llvm",
    "--convert-index-to-llvm",
    "--reconcile-unrealized-casts",
]


@dataclass(frozen=True)
class Toolchain:
    mlse_go: str
    mlse_opt: str
    mlse_run: str
    mlir_opt: str


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Run repo-owned Go execution differential cases by comparing "
            "native `go run` output against `mlse-go -> lower-go-bootstrap -> mlse-run`."
        )
    )
    parser.add_argument(
        "--case-glob",
        action="append",
        default=[],
        help="Case directory glob under test/GoExec/cases; can be repeated",
    )
    parser.add_argument(
        "--artifact-dir",
        default=DEFAULT_ARTIFACT_DIR,
        help="Artifact directory relative to the repo root unless absolute",
    )
    parser.add_argument(
        "--tmp-dir",
        default=DEFAULT_TMP_DIR,
        help="Temporary directory relative to the repo root unless absolute",
    )
    parser.add_argument(
        "--skip-build",
        action="store_true",
        help="Skip rebuilding mlse-go/mlse-run before running the suite",
    )
    parser.add_argument("--mlse-go-bin", default="", help="Override mlse-go binary path")
    parser.add_argument("--mlse-opt-bin", default="", help="Override mlse-opt binary path")
    parser.add_argument("--mlse-run-bin", default="", help="Override mlse-run binary path")
    return parser.parse_args()


def resolve_path(root: Path, text: str) -> Path:
    path = Path(text)
    if path.is_absolute():
        return path
    return (root / path).resolve()


def resolve_executable(text: str) -> str | None:
    if not text:
        return None
    path = Path(text)
    if path.is_absolute() or "/" in text:
        if path.is_file() and os.access(path, os.X_OK):
            return str(path)
        return None
    return shutil.which(text)


def discover_tool(configured: str, candidates: list[str]) -> str:
    resolved = resolve_executable(configured)
    if resolved:
        return resolved
    for candidate in candidates:
        resolved = resolve_executable(candidate)
        if resolved:
            return resolved
    raise RuntimeError(f"required executable not found: {', '.join(candidates)}")


def run(cmd: list[str], *, cwd: Path, capture: bool = True) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd),
        stdout=subprocess.PIPE if capture else None,
        stderr=subprocess.PIPE if capture else None,
        text=True,
        check=False,
    )


def collect_program_dirs(root: Path, patterns: list[str]) -> list[Path]:
    cases_root = root / "test" / "GoExec" / "cases"
    out: list[Path] = []
    seen: set[Path] = set()
    for pattern in patterns or [DEFAULT_CASE_GLOB]:
        for case_dir in sorted(cases_root.glob(pattern)):
            if not case_dir.is_dir():
                continue
            for prog_dir in sorted(case_dir.glob("prog_*")):
                if prog_dir.is_dir() and (prog_dir / "main.go").is_file() and prog_dir not in seen:
                    out.append(prog_dir)
                    seen.add(prog_dir)
    return out


def build_tools(root: Path) -> None:
    for script_name in ("build.sh", "build-mlir.sh"):
        proc = run([str(root / "scripts" / script_name)], cwd=root)
        if proc.returncode != 0:
            raise RuntimeError(
                f"{script_name} failed with code {proc.returncode}\n{proc.stdout}{proc.stderr}"
            )


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def diff_text(expected: str, actual: str, from_label: str, to_label: str) -> str:
    diff = difflib.unified_diff(
        expected.splitlines(keepends=True),
        actual.splitlines(keepends=True),
        fromfile=from_label,
        tofile=to_label,
    )
    return "".join(diff)


def lower_to_llvm_dialect(formal: str, tools: Toolchain, work_dir: Path) -> tuple[int, str, str]:
    formal_path = work_dir / "02-formal.mlir"
    lowered_path = work_dir / "03-go-bootstrap-lowered.mlir"
    llvm_path = work_dir / "04-llvm-dialect.mlir"
    write_text(formal_path, formal)

    proc = run([tools.mlse_opt, "--lower-go-bootstrap", str(formal_path)], cwd=REPO_ROOT)
    if proc.returncode != 0:
        return proc.returncode, proc.stdout, proc.stderr
    write_text(lowered_path, proc.stdout)

    proc2 = run([tools.mlir_opt, str(lowered_path), *LOWERING_PASSES], cwd=REPO_ROOT)
    if proc2.returncode != 0:
        return proc2.returncode, proc2.stdout, proc2.stderr
    write_text(llvm_path, proc2.stdout)
    return 0, proc2.stdout, proc2.stderr


def main() -> int:
    args = parse_args()
    root = REPO_ROOT
    artifact_dir = resolve_path(root, args.artifact_dir)
    tmp_dir = resolve_path(root, args.tmp_dir)

    if not args.skip_build:
        build_tools(root)

    tools = Toolchain(
        mlse_go=discover_tool(args.mlse_go_bin, [str(root / "artifacts" / "bin" / "mlse-go"), "mlse-go"]),
        mlse_opt=discover_tool(
            args.mlse_opt_bin,
            [str(root / "tmp" / "cmake-mlir-build" / "tools" / "mlse-opt" / "mlse-opt"), "mlse-opt"],
        ),
        mlse_run=discover_tool(
            args.mlse_run_bin,
            [str(root / "tmp" / "cmake-mlir-build" / "tools" / "mlse-run" / "mlse-run"), "mlse-run"],
        ),
        mlir_opt=discover_tool("", ["mlir-opt", "/opt/homebrew/opt/llvm@20/bin/mlir-opt"]),
    )

    shutil.rmtree(artifact_dir, ignore_errors=True)
    shutil.rmtree(tmp_dir, ignore_errors=True)
    artifact_dir.mkdir(parents=True, exist_ok=True)
    tmp_dir.mkdir(parents=True, exist_ok=True)

    program_dirs = collect_program_dirs(root, args.case_glob)
    if not program_dirs:
        print("no GoExec programs found", file=sys.stderr)
        return 1

    summary: list[dict[str, object]] = []
    failures = 0
    for prog_dir in program_dirs:
        rel_prog = prog_dir.relative_to(root / "test" / "GoExec" / "cases")
        program_name = rel_prog.as_posix()
        work_dir = tmp_dir / rel_prog
        out_dir = artifact_dir / "files" / rel_prog
        work_dir.mkdir(parents=True, exist_ok=True)
        out_dir.mkdir(parents=True, exist_ok=True)

        source_path = prog_dir / "main.go"
        shutil.copy2(source_path, out_dir / "00-source__main.go")

        native = run(["go", "run", "."], cwd=prog_dir)
        write_text(out_dir / "01-native.stdout", native.stdout)
        write_text(out_dir / "01-native.stderr", native.stderr)

        formal = run([tools.mlse_go, str(source_path)], cwd=root)
        write_text(out_dir / "02-formal.stdout", formal.stdout)
        write_text(out_dir / "02-formal.stderr", formal.stderr)

        case_result: dict[str, object] = {
            "program": program_name,
            "native_rc": native.returncode,
            "mlse_rc": None,
            "ok": False,
        }

        if native.returncode != 0 or formal.returncode != 0:
            failures += 1
            case_result["reason"] = "native go run or mlse-go failed"
            summary.append(case_result)
            print(f"FAIL {program_name}: {case_result['reason']}", file=sys.stderr)
            continue

        lower_rc, llvm_mlir, lower_stderr = lower_to_llvm_dialect(formal.stdout, tools, work_dir)
        write_text(out_dir / "03-go-bootstrap.stderr", lower_stderr)
        if lower_rc != 0:
            failures += 1
            case_result["reason"] = "lowering to llvm dialect failed"
            summary.append(case_result)
            print(f"FAIL {program_name}: {case_result['reason']}", file=sys.stderr)
            continue
        write_text(out_dir / "04-llvm-dialect.mlir", llvm_mlir)

        mlse = run([tools.mlse_run, str(work_dir / "04-llvm-dialect.mlir")], cwd=root)
        write_text(out_dir / "05-mlse-run.stdout", mlse.stdout)
        write_text(out_dir / "05-mlse-run.stderr", mlse.stderr)
        case_result["mlse_rc"] = mlse.returncode

        stdout_match = native.stdout == mlse.stdout
        stderr_match = native.stderr == mlse.stderr
        rc_match = native.returncode == mlse.returncode
        if stdout_match and stderr_match and rc_match:
            case_result["ok"] = True
            summary.append(case_result)
            print(f"PASS {program_name}")
            continue

        failures += 1
        case_result["reason"] = "stdout/stderr/returncode mismatch"
        summary.append(case_result)
        print(f"FAIL {program_name}: {case_result['reason']}", file=sys.stderr)
        if not stdout_match:
            write_text(
                out_dir / "06-stdout.diff",
                diff_text(native.stdout, mlse.stdout, "native", "mlse-run"),
            )
        if not stderr_match:
            write_text(
                out_dir / "06-stderr.diff",
                diff_text(native.stderr, mlse.stderr, "native", "mlse-run"),
            )

    passed = sum(1 for item in summary if item["ok"])
    write_text(artifact_dir / "summary.json", json.dumps(summary, indent=2, ensure_ascii=False) + "\n")
    print(f"summary: {passed}/{len(summary)} passed")
    return 1 if failures else 0


if __name__ == "__main__":
    raise SystemExit(main())

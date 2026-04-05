#!/usr/bin/env python3
from __future__ import annotations

import argparse
import collections
import concurrent.futures
from dataclasses import dataclass
import json
import os
import re
import shutil
import subprocess
import sys
import time
from pathlib import Path


LOWERING_PASSES = [
    "--convert-scf-to-cf",
    "--convert-cf-to-llvm",
    "--convert-arith-to-llvm",
    "--convert-func-to-llvm",
    "--convert-index-to-llvm",
    "--reconcile-unrealized-casts",
]


@dataclass(frozen=True)
class ProbeContext:
    root: Path
    dataset_root: Path
    artifact_dir: Path
    tmp_dir: Path
    env: dict[str, str]
    mlse_go_bin: str
    mlse_go_ssa_dump_bin: str | None
    mlse_opt_bin: str
    mlir_opt_bin: str | None
    mlir_translate_bin: str | None
    opt_bin: str | None
    llvm_as_bin: str | None


@dataclass(frozen=True)
class SummaryContext:
    root: Path
    dataset_root: Path
    artifact_dir: Path
    args: argparse.Namespace
    tool_paths: dict[str, str | None]


@dataclass(frozen=True)
class SSADumpSpec:
    source_path: Path
    source_copy_path: Path
    case_source_dir: Path
    source_dir: Path
    rel_source: Path
    case_name: str


def parse_args() -> argparse.Namespace:
    root = Path(__file__).resolve().parent.parent
    parser = argparse.ArgumentParser(
        description=(
            "Probe gobench-eq Go files through the current MLSE Go frontend, "
            "mlse-opt parse/round-trip, and the LLVM-legal subset of the MLIR pipeline."
        )
    )
    parser.add_argument(
        "--root",
        default=str(root),
        help="MLSE repository root",
    )
    parser.add_argument(
        "--dataset-root",
        default="../gobench-eq/dataset/cases",
        help="Path to gobench-eq dataset/cases relative to --root unless absolute",
    )
    parser.add_argument(
        "--case-glob",
        action="append",
        default=[],
        help="Case directory glob under dataset root; can be repeated",
    )
    parser.add_argument(
        "--artifact-dir",
        default="artifacts/go-gobench-mlir-suite",
        help="Artifact directory relative to --root unless absolute",
    )
    parser.add_argument(
        "--tmp-dir",
        default="tmp/go-gobench-mlir-suite",
        help="Scratch directory relative to --root unless absolute",
    )
    parser.add_argument(
        "--jobs",
        type=int,
        default=max(1, min(8, (os.cpu_count() or 4))),
        help="Worker count",
    )
    parser.add_argument(
        "--limit",
        type=int,
        default=0,
        help="Optional file limit after collection",
    )
    parser.add_argument(
        "--skip-build",
        action="store_true",
        help="Skip rebuilding mlse-go and mlse-opt before probing",
    )
    parser.add_argument("--mlse-go-bin", default="", help="Override mlse-go binary path")
    parser.add_argument(
        "--mlse-go-ssa-dump-bin",
        default="",
        help="Override mlse-go-ssa-dump binary path",
    )
    parser.add_argument("--mlse-opt-bin", default="", help="Override mlse-opt binary path")
    parser.add_argument("--mlir-opt-bin", default="", help="Override upstream mlir-opt path")
    parser.add_argument(
        "--mlir-translate-bin",
        default="",
        help="Override upstream mlir-translate path",
    )
    parser.add_argument("--opt-bin", default="", help="Override LLVM opt path")
    parser.add_argument("--llvm-as-bin", default="", help="Override llvm-as path")
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


def discover_tool(configured: str, logical_name: str) -> str | None:
    resolved = resolve_executable(configured)
    if resolved:
        return resolved
    resolved = shutil.which(logical_name)
    if resolved:
        return resolved
    for prefix in (
        Path("/opt/homebrew/opt/llvm@20/bin"),
        Path("/usr/local/opt/llvm/bin"),
        Path("/opt/homebrew/bin"),
        Path("/usr/local/bin"),
    ):
        candidate = prefix / logical_name
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
    return None


def ensure_dirs(*paths: Path) -> None:
    for path in paths:
        path.mkdir(parents=True, exist_ok=True)


def run(
    cmd: list[str],
    *,
    cwd: Path,
    env: dict[str, str],
    stdin_text: str | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        cwd=str(cwd),
        env=env,
        input=stdin_text,
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )


def build_tools(root: Path, env: dict[str, str]) -> None:
    for script_name in ("build.sh", "build-mlir.sh"):
        proc = run([str(root / "scripts" / script_name)], cwd=root, env=env)
        if proc.returncode != 0:
            raise RuntimeError(
                f"{script_name} failed with code {proc.returncode}\n{proc.stdout}{proc.stderr}"
            )


def collect_case_dirs(dataset_root: Path, globs: list[str]) -> list[Path]:
    out: list[Path] = []
    seen: set[Path] = set()
    for pattern in globs:
        for path in sorted(dataset_root.glob(pattern)):
            if path.is_dir() and path not in seen:
                out.append(path)
                seen.add(path)
    return out


def should_skip_source(path: Path, case_dir: Path) -> bool:
    if path.name.endswith("_test.go"):
        return True
    rel_parts = path.relative_to(case_dir).parts
    for part in rel_parts:
        if part in {"artifacts", "vendor"}:
            return True
        if part.startswith("."):
            return True
    return False


def collect_sources(dataset_root: Path, globs: list[str], limit: int) -> list[Path]:
    files: list[Path] = []
    for case_dir in collect_case_dirs(dataset_root, globs):
        for path in sorted(case_dir.rglob("*.go")):
            if should_skip_source(path, case_dir):
                continue
            files.append(path)
    if limit > 0:
        files = files[:limit]
    return files


def rel_to_root(path: Path, root: Path) -> str:
    try:
        return str(path.relative_to(root))
    except ValueError:
        return os.path.relpath(path, root)


def display_external_path(path: Path, root: Path) -> str:
    return os.path.relpath(path, root)


def display_path(path: str | None, root: Path) -> str:
    if not path:
        return ""
    candidate = Path(path)
    if candidate.is_absolute():
        try:
            return str(candidate.relative_to(root))
        except ValueError:
            return str(candidate)
    return path


def sanitize_text(text: str, root: Path) -> str:
    return text.replace(str(root), ".")


def extract_reason(stdout: str, stderr: str, root: Path) -> str:
    lines = [line.strip() for line in (stdout + "\n" + stderr).splitlines() if line.strip()]
    return sanitize_text(lines[-1], root) if lines else "unknown failure"


def normalize_reason(reason: str) -> str:
    out = reason
    out = re.sub(r'loc\(".*?":\d+:\d+\): error: ', "error: ", out)
    out = re.sub(r'%[A-Za-z_][A-Za-z0-9_]*\d+', "%VAR", out)
    return out


def classify_go_features(text: str) -> list[str]:
    features: list[str] = []
    if "go.todo " in text:
        features.append("go.todo")
    if "go.todo_value" in text:
        features.append("go.todo_value")
    if "go.make_slice" in text:
        features.append("go.make_slice")
    if "go.string_constant" in text:
        features.append("go.string_constant")
    if "go.nil" in text:
        features.append("go.nil")
    if "!go." in text:
        features.append("!go.type")
    return features


def write_text(path: Path, text: str) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    path.write_text(text, encoding="utf-8")


def stage_file_name(index: int, label: str, suffix: str) -> str:
    return f"{index:02d}-{label}{suffix}"


def case_import_root(case_name: str) -> str:
    return f"github.com/timmyyuan/gobench-eq/dataset/cases/{case_name}"


def source_import_path(rel_source: Path) -> str:
    case_name = rel_source.parts[0]
    package_parts = rel_source.parts[1:-1]
    parts = [case_import_root(case_name), *package_parts]
    return "/".join(part for part in parts if part)


def package_has_main_target(source_path: Path) -> bool:
    target_re = re.compile(r"(?m)^func\s+Target\s*\(")
    for go_file in sorted(source_path.parent.glob("*.go")):
        if go_file.name.endswith("_test.go"):
            continue
        text = go_file.read_text(encoding="utf-8")
        if "package main" not in text:
            continue
        if target_re.search(text):
            return True
    return False


def stage_case_tree(source_path: Path, ctx: ProbeContext, case_name: str, rel_source: Path) -> Path:
    stage_root = ctx.tmp_dir / "ssa" / rel_source
    if stage_root.exists():
        shutil.rmtree(stage_root)
    gopath_root = stage_root / "gopath"
    case_stage = gopath_root / "src" / Path(case_import_root(case_name))
    shutil.copytree(ctx.dataset_root / case_name, case_stage)
    return gopath_root


def stage_source_for_frontend(source_path: Path, ctx: ProbeContext, case_name: str, rel_source: Path) -> tuple[Path, Path]:
    gopath_root = stage_case_tree(source_path, ctx, case_name, rel_source)
    staged_source_path = gopath_root / "src" / Path(case_import_root(case_name)) / Path(*rel_source.parts[1:])
    return gopath_root, staged_source_path


def dump_target_ssa(
    spec: SSADumpSpec,
    ctx: ProbeContext,
) -> tuple[str, str]:
    ssa_txt_path = spec.source_dir / stage_file_name(1, "ssa__main.Target", ".txt")
    ssa_stdout_path = spec.source_dir / stage_file_name(1, "ssa", ".stdout")
    ssa_stderr_path = spec.source_dir / stage_file_name(1, "ssa", ".stderr")
    if not ctx.mlse_go_ssa_dump_bin or not Path(ctx.mlse_go_ssa_dump_bin).is_file():
        note = "// mlse-go-ssa-dump unavailable\n"
        write_text(ssa_txt_path, note)
        write_text(ssa_stdout_path, note)
        write_text(ssa_stderr_path, "")
        return rel_to_root(ssa_txt_path, ctx.root), "unavailable"
    if not package_has_main_target(spec.source_path):
        note = "// main.Target not found in package\n"
        write_text(ssa_txt_path, note)
        write_text(ssa_stdout_path, note)
        write_text(ssa_stderr_path, "")
        return rel_to_root(ssa_txt_path, ctx.root), "not_found"

    gopath_root = stage_case_tree(spec.source_path, ctx, spec.case_name, spec.rel_source)
    ssa_env = dict(ctx.env)
    ssa_env["GO111MODULE"] = "off"
    ssa_env["GOPATH"] = str(gopath_root)
    pkg_path = source_import_path(spec.rel_source)
    staged_case_root = gopath_root / "src" / Path(case_import_root(spec.case_name))
    proc = run(
        [ctx.mlse_go_ssa_dump_bin, "-package", pkg_path, "-func", "Target"],
        cwd=ctx.root,
        env=ssa_env,
    )
    artifact_case_root = rel_to_root(spec.case_source_dir, ctx.root)
    sanitized_stdout = proc.stdout.replace(str(staged_case_root), artifact_case_root)
    write_text(ssa_stdout_path, sanitized_stdout)
    write_text(ssa_stderr_path, proc.stderr)
    if proc.returncode == 0:
        write_text(ssa_txt_path, sanitized_stdout)
        return rel_to_root(ssa_txt_path, ctx.root), "success"

    note = (
        f"// failed to dump SSA for main.Target from {display_external_path(spec.source_path, ctx.root)}\n"
        f"// package: {pkg_path}\n"
        f"// reason: {extract_reason(proc.stdout, proc.stderr, ctx.root)}\n"
    )
    write_text(ssa_txt_path, note)
    return rel_to_root(ssa_txt_path, ctx.root), "failure"


def verify_llvm_ir(
    llvm_ir_path: Path,
    *,
    cwd: Path,
    env: dict[str, str],
    opt_bin: str | None,
    llvm_as_bin: str | None,
) -> tuple[str, str, str, str]:
    if opt_bin:
        proc = run([opt_bin, "-Oz", "-S", str(llvm_ir_path)], cwd=cwd, env=env)
        return (
            "success" if proc.returncode == 0 else "failure",
            "opt -Oz",
            proc.stdout,
            proc.stderr,
        )
    if llvm_as_bin:
        proc = run([llvm_as_bin, "-o", os.devnull, str(llvm_ir_path)], cwd=cwd, env=env)
        return (
            "success" if proc.returncode == 0 else "failure",
            "llvm-as",
            "",
            proc.stdout + proc.stderr,
        )
    return "unavailable", "missing", "", ""


def probe_one(
    source_path: Path,
    ctx: ProbeContext,
) -> dict:
    rel_source = source_path.relative_to(ctx.dataset_root)
    case_name = rel_source.parts[0] if rel_source.parts else ""
    variant = rel_source.parts[1] if len(rel_source.parts) > 1 else ""
    source_dir = ctx.artifact_dir / "files" / rel_source
    case_source_dir = ctx.artifact_dir / "cases" / case_name / stage_file_name(0, "case-source", "")
    if source_dir.exists():
        shutil.rmtree(source_dir)
    ensure_dirs(source_dir)
    stale_compat_mlir = source_dir / "compat.mlir"
    if stale_compat_mlir.exists():
        stale_compat_mlir.unlink()
    source_copy_path = source_dir / stage_file_name(0, f"source__{source_path.name}", "")
    if not case_source_dir.exists():
        case_source_dir.parent.mkdir(parents=True, exist_ok=True)
        try:
            shutil.copytree(ctx.dataset_root / case_name, case_source_dir)
        except FileExistsError:
            pass
    shutil.copy2(source_path, source_copy_path)
    ssa_target_path, ssa_target_status = dump_target_ssa(
        SSADumpSpec(
            source_path=source_path,
            source_copy_path=source_copy_path,
            case_source_dir=case_source_dir,
            source_dir=source_dir,
            rel_source=rel_source,
            case_name=case_name,
        ),
        ctx,
    )
    formal_mlir_path = source_dir / stage_file_name(2, "formal", ".mlir")
    roundtrip_mlir_path = source_dir / stage_file_name(3, "roundtrip", ".mlir")
    go_lower_mlir_path = source_dir / stage_file_name(4, "go-bootstrap-lowered", ".mlir")
    llvm_dialect_path = source_dir / stage_file_name(5, "llvm-dialect", ".mlir")
    llvm_ir_path = source_dir / stage_file_name(6, "module", ".ll")
    llvm_oz_ir_path = source_dir / stage_file_name(7, "module.Oz", ".ll")

    record = {
        "source_path": display_external_path(source_path, ctx.root),
        "case": case_name,
        "variant": variant,
        "source_copy_path": rel_to_root(source_copy_path, ctx.root),
        "ssa_target_path": ssa_target_path,
        "ssa_target_status": ssa_target_status,
        "frontend_status": "failure",
        "frontend_reason": "",
        "formal_mlir_path": rel_to_root(formal_mlir_path, ctx.root),
        "mlse_opt_status": "skipped",
        "mlse_opt_reason": "",
        "roundtrip_mlir_path": rel_to_root(roundtrip_mlir_path, ctx.root),
        "go_lower_status": "skipped",
        "go_lower_reason": "",
        "go_lower_mlir_path": rel_to_root(go_lower_mlir_path, ctx.root),
        "go_features": [],
        "llvm_eligibility": "skipped",
        "llvm_lower_status": "skipped",
        "llvm_lower_reason": "",
        "llvm_dialect_path": rel_to_root(llvm_dialect_path, ctx.root),
        "llvm_translate_status": "skipped",
        "llvm_translate_reason": "",
        "llvm_ir_path": rel_to_root(llvm_ir_path, ctx.root),
        "llvm_oz_ir_path": rel_to_root(llvm_oz_ir_path, ctx.root),
        "llvm_verify_status": "skipped",
        "llvm_verify_tool": "missing",
        "llvm_verify_reason": "",
    }

    frontend_gopath, staged_source_path = stage_source_for_frontend(source_path, ctx, case_name, rel_source)
    frontend_env = dict(ctx.env)
    frontend_env["GO111MODULE"] = "off"
    frontend_env["GOPATH"] = str(frontend_gopath)
    frontend_env["MLSE_SOURCE_DISPLAY_PATH"] = display_external_path(source_path, ctx.root)
    frontend = run([ctx.mlse_go_bin, str(staged_source_path)], cwd=ctx.root, env=frontend_env)
    write_text(source_dir / stage_file_name(2, "frontend", ".stdout"), frontend.stdout)
    write_text(source_dir / stage_file_name(2, "frontend", ".stderr"), frontend.stderr)
    if frontend.returncode != 0:
        record["frontend_reason"] = extract_reason(frontend.stdout, frontend.stderr, ctx.root)
        return record

    write_text(formal_mlir_path, frontend.stdout)
    record["frontend_status"] = "success"

    parsed = run([ctx.mlse_opt_bin, str(formal_mlir_path)], cwd=ctx.root, env=ctx.env)
    write_text(source_dir / stage_file_name(3, "mlse-opt", ".stdout"), parsed.stdout)
    write_text(source_dir / stage_file_name(3, "mlse-opt", ".stderr"), parsed.stderr)
    if parsed.returncode != 0:
        record["mlse_opt_status"] = "failure"
        record["mlse_opt_reason"] = extract_reason(parsed.stdout, parsed.stderr, ctx.root)
        return record

    write_text(roundtrip_mlir_path, parsed.stdout)
    record["mlse_opt_status"] = "success"

    go_lowered = run(
        [ctx.mlse_opt_bin, "--lower-go-bootstrap", str(roundtrip_mlir_path)],
        cwd=ctx.root,
        env=ctx.env,
    )
    write_text(source_dir / stage_file_name(4, "go-lower", ".stdout"), go_lowered.stdout)
    write_text(source_dir / stage_file_name(4, "go-lower", ".stderr"), go_lowered.stderr)
    if go_lowered.returncode != 0:
        record["go_lower_status"] = "failure"
        record["go_lower_reason"] = extract_reason(go_lowered.stdout, go_lowered.stderr, ctx.root)
        record["go_features"] = classify_go_features(parsed.stdout)
        record["llvm_eligibility"] = "blocked_go_dialect"
        if record["go_features"]:
            record["llvm_lower_reason"] = "contains unresolved go dialect syntax: " + ", ".join(
                record["go_features"]
            )
        else:
            record["llvm_lower_reason"] = "go bootstrap lowering failed: " + record["go_lower_reason"]
        return record

    write_text(go_lower_mlir_path, go_lowered.stdout)
    record["go_lower_status"] = "success"

    features = classify_go_features(go_lowered.stdout)
    record["go_features"] = features
    if features:
        record["llvm_eligibility"] = "blocked_go_dialect"
        record["llvm_lower_reason"] = "contains unresolved go dialect syntax: " + ", ".join(
            features
        )
        return record

    if not ctx.mlir_opt_bin or not ctx.mlir_translate_bin:
        record["llvm_eligibility"] = "tool_unavailable"
        record["llvm_lower_status"] = "unavailable"
        record["llvm_lower_reason"] = "missing mlir-opt or mlir-translate"
        return record

    record["llvm_eligibility"] = "eligible"
    lowered = run(
        [ctx.mlir_opt_bin, str(go_lower_mlir_path), *LOWERING_PASSES],
        cwd=ctx.root,
        env=ctx.env,
    )
    write_text(source_dir / stage_file_name(5, "mlir-opt", ".stdout"), lowered.stdout)
    write_text(source_dir / stage_file_name(5, "mlir-opt", ".stderr"), lowered.stderr)
    if lowered.returncode != 0:
        record["llvm_lower_status"] = "failure"
        record["llvm_lower_reason"] = extract_reason(lowered.stdout, lowered.stderr, ctx.root)
        return record

    write_text(llvm_dialect_path, lowered.stdout)
    record["llvm_lower_status"] = "success"

    translated = run(
        [ctx.mlir_translate_bin, "--mlir-to-llvmir", str(llvm_dialect_path)],
        cwd=ctx.root,
        env=ctx.env,
    )
    write_text(source_dir / stage_file_name(6, "mlir-translate", ".stdout"), translated.stdout)
    write_text(source_dir / stage_file_name(6, "mlir-translate", ".stderr"), translated.stderr)
    if translated.returncode != 0:
        record["llvm_translate_status"] = "failure"
        record["llvm_translate_reason"] = extract_reason(translated.stdout, translated.stderr, ctx.root)
        return record

    write_text(llvm_ir_path, translated.stdout)
    record["llvm_translate_status"] = "success"

    verify_status, verify_tool, verify_stdout, verify_stderr = verify_llvm_ir(
        llvm_ir_path,
        cwd=ctx.root,
        env=ctx.env,
        opt_bin=ctx.opt_bin,
        llvm_as_bin=ctx.llvm_as_bin,
    )
    if verify_stdout:
        write_text(llvm_oz_ir_path, verify_stdout)
    write_text(source_dir / stage_file_name(7, "opt", ".stderr"), verify_stderr)
    record["llvm_verify_status"] = verify_status
    record["llvm_verify_tool"] = verify_tool
    if verify_status != "success":
        record["llvm_verify_reason"] = extract_reason(verify_stdout, verify_stderr, ctx.root)
    return record


def build_summary(
    ctx: SummaryContext,
    results: list[dict],
    elapsed_seconds: float,
) -> dict:
    counts = collections.Counter()
    feature_counts = collections.Counter()
    frontend_failures = collections.Counter()
    parse_failures = collections.Counter()
    lower_failures = collections.Counter()
    translate_failures = collections.Counter()
    verify_failures = collections.Counter()

    for record in results:
        counts["total_files"] += 1
        counts[f"frontend_{record['frontend_status']}"] += 1
        counts[f"mlse_opt_{record['mlse_opt_status']}"] += 1
        counts[f"go_lower_{record['go_lower_status']}"] += 1
        counts[f"llvm_eligibility_{record['llvm_eligibility']}"] += 1
        counts[f"llvm_lower_{record['llvm_lower_status']}"] += 1
        counts[f"llvm_translate_{record['llvm_translate_status']}"] += 1
        counts[f"llvm_verify_{record['llvm_verify_status']}"] += 1
        for feature in record["go_features"]:
            feature_counts[feature] += 1
        if record["frontend_status"] == "failure":
            frontend_failures[normalize_reason(record["frontend_reason"])] += 1
        if record["mlse_opt_status"] == "failure":
            parse_failures[normalize_reason(record["mlse_opt_reason"])] += 1
        if record["go_lower_status"] == "failure":
            lower_failures[normalize_reason(record["go_lower_reason"])] += 1
        if record["llvm_lower_status"] == "failure":
            lower_failures[normalize_reason(record["llvm_lower_reason"])] += 1
        if record["llvm_translate_status"] == "failure":
            translate_failures[normalize_reason(record["llvm_translate_reason"])] += 1
        if record["llvm_verify_status"] == "failure":
            verify_failures[normalize_reason(record["llvm_verify_reason"])] += 1

    return {
        "generated_at_unix": int(time.time()),
        "elapsed_seconds": round(elapsed_seconds, 3),
        "root": rel_to_root(ctx.root, ctx.root),
        "dataset_root": display_external_path(ctx.dataset_root, ctx.root),
        "artifact_dir": rel_to_root(ctx.artifact_dir, ctx.root),
        "case_globs": ctx.args.case_glob,
        "jobs": ctx.args.jobs,
        "limit": ctx.args.limit,
        "tool_paths": {name: display_path(path, ctx.root) for name, path in ctx.tool_paths.items()},
        "counts": dict(counts),
        "go_feature_counts": dict(feature_counts),
        "top_frontend_failures": frontend_failures.most_common(20),
        "top_mlse_opt_failures": parse_failures.most_common(20),
        "top_llvm_lower_failures": lower_failures.most_common(20),
        "top_llvm_translate_failures": translate_failures.most_common(20),
        "top_llvm_verify_failures": verify_failures.most_common(20),
        "results_path": rel_to_root(ctx.artifact_dir / "results.json", ctx.root),
    }


def write_summary_markdown(path: Path, summary: dict, results: list[dict]) -> None:
    counts = summary["counts"]
    lines = [
        "# gobench-eq MLIR suite",
        "",
        "这份报告测试的是当前仓库的**工程可达性**：",
        "",
        "- `Go -> formal MLIR` 是否成功",
        "- `mlse-opt` 是否能解析并 round-trip 这份 MLIR",
        "- `mlse-opt --lower-go-bootstrap` 是否能把当前 `go` bootstrap dialect lower 到不含 unresolved `go` 语法的 LLVM-legal MLIR",
        "- 对经过这一步的子集，是否能直接从 `MLIR` 继续 lower 到 `LLVM IR` 并通过 `opt -Oz` 校验",
        "",
        "它不直接证明语义等价；当前仓库也还没有完整的 `Go -> MLIR -> LLVM IR` 语义收敛。",
        "",
        "## Summary",
        "",
        f"- dataset root: `{summary['dataset_root']}`",
        f"- artifact dir: `{summary['artifact_dir']}`",
        f"- elapsed seconds: `{summary['elapsed_seconds']}`",
        f"- total files: `{counts.get('total_files', 0)}`",
        f"- frontend success: `{counts.get('frontend_success', 0)}`",
        f"- mlse-opt success: `{counts.get('mlse_opt_success', 0)}`",
        f"- go bootstrap lowering success: `{counts.get('go_lower_success', 0)}`",
        f"- llvm eligible: `{counts.get('llvm_eligibility_eligible', 0)}`",
        f"- llvm lowering success: `{counts.get('llvm_lower_success', 0)}`",
        f"- llvm translate success: `{counts.get('llvm_translate_success', 0)}`",
        f"- llvm opt -Oz success: `{counts.get('llvm_verify_success', 0)}`",
        f"- blocked by unresolved go dialect syntax: `{counts.get('llvm_eligibility_blocked_go_dialect', 0)}`",
        "",
        "## Go Features Blocking LLVM Lowering",
        "",
    ]

    feature_counts = summary["go_feature_counts"]
    if feature_counts:
        for feature, count in sorted(feature_counts.items()):
            lines.append(f"- `{feature}`: `{count}`")
    else:
        lines.append("- none")

    lines.extend(["", "## Representative Failures", ""])
    failure_records = [
        record
        for record in results
        if record["frontend_status"] != "success"
        or record["mlse_opt_status"] != "success"
        or record["go_lower_status"] == "failure"
        or record["llvm_lower_status"] == "failure"
        or record["llvm_translate_status"] == "failure"
        or record["llvm_verify_status"] == "failure"
        or record["llvm_eligibility"] == "blocked_go_dialect"
    ]
    if failure_records:
        for record in failure_records[:25]:
            lines.append(f"- `{record['source_path']}`")
            if record["frontend_status"] != "success":
                lines.append(f"  frontend: `{record['frontend_reason']}`")
                continue
            if record["mlse_opt_status"] != "success":
                lines.append(f"  mlse-opt: `{record['mlse_opt_reason']}`")
                continue
            if record["go_lower_status"] == "failure":
                lines.append(f"  go bootstrap lower: `{record['go_lower_reason']}`")
                continue
            if record["llvm_eligibility"] == "blocked_go_dialect":
                lines.append(f"  llvm blocked: `{record['llvm_lower_reason']}`")
                continue
            if record["llvm_lower_status"] == "failure":
                lines.append(f"  llvm lower: `{record['llvm_lower_reason']}`")
                continue
            if record["llvm_translate_status"] == "failure":
                lines.append(f"  llvm translate: `{record['llvm_translate_reason']}`")
                continue
            if record["llvm_verify_status"] == "failure":
                lines.append(f"  llvm opt -Oz: `{record['llvm_verify_reason']}`")
    else:
        lines.append("- none")

    write_text(path, "\n".join(lines) + "\n")


def main() -> int:
    args = parse_args()
    if not args.case_glob:
        args.case_glob = ["goeq-spec-*"]
    root = resolve_path(Path.cwd(), args.root)
    dataset_root = resolve_path(root, args.dataset_root)
    artifact_dir = resolve_path(root, args.artifact_dir)
    tmp_dir = resolve_path(root, args.tmp_dir)

    if not dataset_root.is_dir():
        print(f"error: dataset root not found: {dataset_root}", file=sys.stderr)
        return 2

    if artifact_dir.exists():
        shutil.rmtree(artifact_dir)
    if tmp_dir.exists():
        shutil.rmtree(tmp_dir)

    env = os.environ.copy()
    env["GOCACHE"] = str(tmp_dir / "go-build")
    env["GOMODCACHE"] = str(tmp_dir / "gomodcache")
    env["HOME"] = str(tmp_dir / "home")
    ensure_dirs(
        artifact_dir,
        tmp_dir,
        Path(env["GOCACHE"]),
        Path(env["GOMODCACHE"]),
        Path(env["HOME"]),
    )

    if not args.skip_build:
        print("building mlse-go and mlse-opt...", file=sys.stderr)
        build_tools(root, env)

    mlse_go_bin = discover_tool(args.mlse_go_bin, "mlse-go") or str(root / "artifacts" / "bin" / "mlse-go")
    mlse_go_ssa_dump_bin = discover_tool(args.mlse_go_ssa_dump_bin, "mlse-go-ssa-dump") or str(
        root / "artifacts" / "bin" / "mlse-go-ssa-dump"
    )
    mlse_opt_bin = discover_tool(args.mlse_opt_bin, "mlse-opt") or str(root / "tmp" / "cmake-mlir-build" / "tools" / "mlse-opt" / "mlse-opt")
    mlir_opt_bin = discover_tool(args.mlir_opt_bin, "mlir-opt")
    mlir_translate_bin = discover_tool(args.mlir_translate_bin, "mlir-translate")
    opt_bin = discover_tool(args.opt_bin, "opt")
    llvm_as_bin = discover_tool(args.llvm_as_bin, "llvm-as")

    for required in (mlse_go_bin, mlse_opt_bin):
        if not required or not Path(required).is_file():
            print(f"error: required binary not found: {required}", file=sys.stderr)
            return 2

    sources = collect_sources(dataset_root, args.case_glob, args.limit)
    if not sources:
        print("error: no source files matched", file=sys.stderr)
        return 2

    print(
        f"probing {len(sources)} files from {display_external_path(dataset_root, root)} "
        f"into {rel_to_root(artifact_dir, root)}",
        file=sys.stderr,
    )

    probe_ctx = ProbeContext(
        root=root,
        dataset_root=dataset_root,
        artifact_dir=artifact_dir,
        tmp_dir=tmp_dir,
        env=env,
        mlse_go_bin=mlse_go_bin,
        mlse_go_ssa_dump_bin=mlse_go_ssa_dump_bin,
        mlse_opt_bin=mlse_opt_bin,
        mlir_opt_bin=mlir_opt_bin,
        mlir_translate_bin=mlir_translate_bin,
        opt_bin=opt_bin,
        llvm_as_bin=llvm_as_bin,
    )
    start = time.time()
    results: list[dict] = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as executor:
        futures = [executor.submit(probe_one, source, probe_ctx) for source in sources]
        for future in concurrent.futures.as_completed(futures):
            results.append(future.result())

    results.sort(key=lambda item: item["source_path"])
    elapsed = time.time() - start

    tool_paths = {
        "mlse_go_bin": mlse_go_bin,
        "mlse_go_ssa_dump_bin": mlse_go_ssa_dump_bin,
        "mlse_opt_bin": mlse_opt_bin,
        "mlir_opt_bin": mlir_opt_bin,
        "mlir_translate_bin": mlir_translate_bin,
        "opt_bin": opt_bin,
        "llvm_as_bin": llvm_as_bin,
    }
    summary = build_summary(
        SummaryContext(
            root=root,
            dataset_root=dataset_root,
            artifact_dir=artifact_dir,
            args=args,
            tool_paths=tool_paths,
        ),
        results=results,
        elapsed_seconds=elapsed,
    )

    write_text(artifact_dir / "results.json", json.dumps(results, indent=2, ensure_ascii=False) + "\n")
    write_text(artifact_dir / "summary.json", json.dumps(summary, indent=2, ensure_ascii=False) + "\n")
    write_summary_markdown(artifact_dir / "summary.md", summary, results)

    counts = summary["counts"]
    print(
        json.dumps(
            {
                "total_files": counts.get("total_files", 0),
                "frontend_success": counts.get("frontend_success", 0),
                "mlse_opt_success": counts.get("mlse_opt_success", 0),
                "llvm_eligible": counts.get("llvm_eligibility_eligible", 0),
                "llvm_translate_success": counts.get("llvm_translate_success", 0),
                "llvm_verify_success": counts.get("llvm_verify_success", 0),
                "artifact_dir": rel_to_root(artifact_dir, root),
            },
            ensure_ascii=False,
        )
    )
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

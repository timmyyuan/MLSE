#!/usr/bin/env python3
from __future__ import annotations

import argparse
import collections
import concurrent.futures
import json
import os
import re
import shutil
import subprocess
import tempfile
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser(
        description=(
            "Bulk-probe Go files from an external gobench-eq checkout through the "
            "experimental MLSE GoIR-like -> LLVM dialect MLIR -> LLVM IR path."
        )
    )
    parser.add_argument("--root", required=True, help="MLSE repo root")
    parser.add_argument("--dataset-repo", required=True, help="External gobench-eq repo root")
    parser.add_argument(
        "--artifact-dir",
        default="artifacts/goeq-llvm-bulk-probe",
        help="Artifact directory relative to MLSE root",
    )
    parser.add_argument(
        "--jobs",
        type=int,
        default=max(1, min(8, (os.cpu_count() or 4))),
        help="Worker count",
    )
    return parser.parse_args()


def sanitize_text(text: str, root: Path, dataset_repo: Path) -> str:
    text = text.replace(str(root), ".")
    text = text.replace(str(dataset_repo), "../gobench-eq")
    text = re.sub(r"/var/folders/[^ \t\n\"']+", "<tmp>", text)
    text = re.sub(r"/private/var/folders/[^ \t\n\"']+", "<tmp>", text)
    return text


def rel_to_root(path: Path, root: Path) -> str:
    try:
        return str(path.relative_to(root))
    except ValueError:
        return os.path.relpath(path, root)


def classify_file(path: Path) -> str:
    parts = path.parts
    if "harness" in parts:
        return "harness"
    if "prog_a" in parts:
        return "prog_a"
    if "prog_b" in parts:
        return "prog_b"
    return "other"


def extract_reason(output: str) -> str:
    lines = [line.strip() for line in output.splitlines() if line.strip()]
    filtered = [
        line
        for line in lines
        if not line.startswith("go: downloading ")
        and not line.startswith("go: writing stat cache:")
    ]
    if filtered:
        return filtered[-1]
    if lines:
        return lines[-1]
    return "unknown failure"


def normalize_failure_reason(reason: str) -> str:
    normalized = reason
    normalized = re.sub(r"line \d+:", "line N:", normalized)
    normalized = re.sub(r"%funclit\d+", "%funclitN", normalized)
    normalized = re.sub(r"%g_\d+", "%g_N", normalized)
    normalized = re.sub(
        r"unknown value %[A-Za-z_][A-Za-z0-9_]*",
        "unknown value %VAR",
        normalized,
    )
    normalized = re.sub(
        r'unsupported GoIR instruction "mlse\.\+\+ .*"',
        'unsupported GoIR instruction "mlse.++ <value>"',
        normalized,
    )
    normalized = re.sub(
        r'unsupported GoIR instruction "mlse\.-- .*"',
        'unsupported GoIR instruction "mlse.-- <value>"',
        normalized,
    )
    normalized = re.sub(
        r'malformed call instruction ".*"',
        'malformed call instruction "<call>"',
        normalized,
    )
    return normalized


def tool_label(path: str | None) -> str:
    if not path:
        return "missing"
    p = Path(path)
    text = p.name
    if "llvm@20" in path:
        return f"llvm@20/{text}"
    return text


def discover_tool(env_name: str, logical_name: str) -> str | None:
    configured = os.environ.get(env_name, "").strip()
    if configured:
        candidate = Path(configured)
        if candidate.is_file() and os.access(candidate, os.X_OK):
            return str(candidate)
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


def detect_target_dir(dataset_repo: Path) -> tuple[Path, dict]:
    requested = dataset_repo / "goeq-spec"
    if requested.is_dir():
        info = {
            "requested_path": "../gobench-eq/goeq-spec",
            "resolved_path": "../gobench-eq/goeq-spec",
            "resolution": "exact",
        }
        return requested, info
    fallback = dataset_repo / "dataset" / "cases"
    if not fallback.is_dir():
        raise FileNotFoundError(
            "Could not find ../gobench-eq/goeq-spec or ../gobench-eq/dataset/cases"
        )
    info = {
        "requested_path": "../gobench-eq/goeq-spec",
        "resolved_path": "../gobench-eq/dataset/cases",
        "resolution": "fallback-dataset-cases",
        "note": (
            "The external checkout does not contain a literal goeq-spec directory. "
            "This probe used dataset/cases as the actual Go case root."
        ),
    }
    return fallback, info


def build_binaries(root: Path, env: dict[str, str]) -> dict[str, Path]:
    bin_dir = root / "artifacts" / "bin"
    bin_dir.mkdir(parents=True, exist_ok=True)
    outputs = {
        "mlse_go": bin_dir / "mlse-go",
        "mlse_goir_llvm_exp": bin_dir / "mlse-goir-llvm-exp",
    }
    subprocess.run(
        ["go", "build", "-o", str(outputs["mlse_go"]), "./cmd/mlse-go"],
        cwd=root,
        env=env,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    subprocess.run(
        ["go", "build", "-o", str(outputs["mlse_goir_llvm_exp"]), "./cmd/mlse-goir-llvm-exp"],
        cwd=root,
        env=env,
        check=True,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    return outputs


def run_cmd(
    cmd: list[str],
    *,
    input_text: str | None = None,
    cwd: Path | None = None,
) -> subprocess.CompletedProcess[str]:
    return subprocess.run(
        cmd,
        input=input_text,
        cwd=str(cwd) if cwd else None,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )


def verify_llvm_ir(
    llvm_ir: str,
    opt_bin: str | None,
    llvm_as_bin: str | None,
    work_dir: Path,
) -> tuple[str, str, str]:
    ll_path = work_dir / "module.ll"
    ll_path.write_text(llvm_ir, encoding="utf-8")
    if opt_bin:
        proc = run_cmd([opt_bin, "-passes=verify", "-disable-output", str(ll_path)])
        return ("success" if proc.returncode == 0 else "failure", proc.stdout, tool_label(opt_bin))
    if llvm_as_bin:
        proc = run_cmd([llvm_as_bin, "-o", os.devnull, str(ll_path)])
        return ("success" if proc.returncode == 0 else "failure", proc.stdout, tool_label(llvm_as_bin))
    return ("unavailable", "", "missing")


def probe_one(
    file_path: Path,
    *,
    root: Path,
    dataset_repo: Path,
    target_dir: Path,
    outputs: dict[str, Path],
    mlir_opt_bin: str | None,
    mlir_translate_bin: str | None,
    opt_bin: str | None,
    llvm_as_bin: str | None,
    failure_log_dir: Path,
) -> dict:
    rel_target = file_path.relative_to(target_dir)
    display_path = f"../gobench-eq/{file_path.relative_to(dataset_repo)}"
    record = {
        "path": display_path,
        "relative_to_target": str(rel_target),
        "file_kind": classify_file(file_path),
        "is_test_file": file_path.name.endswith("_test.go"),
        "frontend_status": "failure",
        "lowering_status": "skipped",
        "llvm_dialect_parse_status": "skipped",
        "translation_status": "skipped",
        "verification_status": "skipped",
        "verification_tool": "",
        "ok": False,
        "failure_stage": "",
        "failure_reason": "",
        "failure_log": "",
    }

    failure_texts: list[tuple[str, str]] = []

    front = run_cmd([str(outputs["mlse_go"]), "-emit=goir-like", str(file_path)], cwd=root)
    front_out = sanitize_text(front.stdout, root, dataset_repo)
    if front.returncode != 0:
        record["failure_stage"] = "frontend"
        record["failure_reason"] = extract_reason(front_out)
        failure_texts.append(("frontend", front_out))
        return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)
    record["frontend_status"] = "success"
    goir_text = front.stdout

    lower = run_cmd(
        [str(outputs["mlse_goir_llvm_exp"]), "-emit=llvm-dialect"],
        input_text=goir_text,
        cwd=root,
    )
    lower_out = sanitize_text(lower.stdout, root, dataset_repo)
    if lower.returncode != 0:
        record["failure_stage"] = "lowering"
        record["failure_reason"] = extract_reason(lower_out)
        failure_texts.append(("lowering", lower_out))
        return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)
    record["lowering_status"] = "success"
    llvm_dialect_text = lower.stdout

    if mlir_opt_bin:
        with tempfile.TemporaryDirectory(dir=root / "tmp") as temp_dir_name:
            temp_dir = Path(temp_dir_name)
            mlir_path = temp_dir / "module.mlir"
            mlir_path.write_text(llvm_dialect_text, encoding="utf-8")
            parse = run_cmd([mlir_opt_bin, str(mlir_path)])
        parse_out = sanitize_text(parse.stdout, root, dataset_repo)
        if parse.returncode != 0:
            record["llvm_dialect_parse_status"] = "failure"
            record["failure_stage"] = "llvm_dialect_parse"
            record["failure_reason"] = extract_reason(parse_out)
            failure_texts.append(("llvm-dialect-parse", parse_out))
            return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)
        record["llvm_dialect_parse_status"] = "success"
    else:
        record["llvm_dialect_parse_status"] = "unavailable"

    if not mlir_translate_bin:
        record["failure_stage"] = "translation"
        record["failure_reason"] = "mlir-translate is unavailable on this machine"
        return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)

    with tempfile.TemporaryDirectory(dir=root / "tmp") as temp_dir_name:
        temp_dir = Path(temp_dir_name)
        mlir_path = temp_dir / "module.mlir"
        mlir_path.write_text(llvm_dialect_text, encoding="utf-8")
        translate = run_cmd([mlir_translate_bin, "--mlir-to-llvmir", str(mlir_path)])
        translate_out = sanitize_text(translate.stdout, root, dataset_repo)
        if translate.returncode != 0:
            record["translation_status"] = "failure"
            record["failure_stage"] = "translation"
            record["failure_reason"] = extract_reason(translate_out)
            failure_texts.append(("translation", translate_out))
            return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)
        record["translation_status"] = "success"

        verification_status, verify_output, verification_tool = verify_llvm_ir(
            translate.stdout, opt_bin, llvm_as_bin, temp_dir
        )
    verify_out = sanitize_text(verify_output, root, dataset_repo)
    record["verification_status"] = verification_status
    record["verification_tool"] = verification_tool
    if verification_status == "failure":
        record["failure_stage"] = "verification"
        record["failure_reason"] = extract_reason(verify_out)
        failure_texts.append(("verification", verify_out))
        return finalize_failure(record, failure_texts, failure_log_dir, rel_target, root)

    record["ok"] = True
    return record


def finalize_failure(
    record: dict,
    failure_texts: list[tuple[str, str]],
    failure_log_dir: Path,
    rel_target: Path,
    root: Path,
) -> dict:
    safe = re.sub(r"[^A-Za-z0-9._-]+", "__", str(rel_target))
    log_path = failure_log_dir / f"{safe}.log"
    sections = []
    for stage, text in failure_texts:
        sections.append(f"== {stage} ==\n{text.rstrip()}\n")
    log_path.write_text("\n".join(sections), encoding="utf-8")
    record["failure_log"] = rel_to_root(log_path, root)
    return record


def summarize(rows: list[dict], *, root: Path, artifact_dir: Path, target_info: dict, tools: dict, dataset_repo: Path) -> dict:
    total = len(rows)
    success_rows = [row for row in rows if row["ok"]]
    fail_rows = [row for row in rows if not row["ok"]]
    success_rate = (len(success_rows) / total) if total else 0.0

    by_kind = collections.defaultdict(lambda: {"total": 0, "success": 0, "failure": 0})
    failure_categories = collections.Counter()
    failure_examples = {}
    stage_counts = collections.Counter()
    for row in rows:
        entry = by_kind[row["file_kind"]]
        entry["total"] += 1
        if row["ok"]:
            entry["success"] += 1
        else:
            entry["failure"] += 1
            key = f"{row['failure_stage']}: {normalize_failure_reason(row['failure_reason'])}"
            failure_categories[key] += 1
            failure_examples.setdefault(key, row["path"])
            stage_counts[row["failure_stage"]] += 1

    success_list = artifact_dir / "success.txt"
    fail_list = artifact_dir / "fail.txt"
    reasons_path = artifact_dir / "failure-reasons.txt"
    results_jsonl = artifact_dir / "results.jsonl"
    summary_json = artifact_dir / "summary.json"
    report_md = artifact_dir / "report.md"

    success_list.write_text(
        "\n".join(row["path"] for row in success_rows) + ("\n" if success_rows else ""),
        encoding="utf-8",
    )
    fail_list.write_text(
        "\n".join(row["path"] for row in fail_rows) + ("\n" if fail_rows else ""),
        encoding="utf-8",
    )
    reasons_path.write_text(
        "\n".join(
            f"{count}\t{failure_examples[reason]}\t{reason}"
            for reason, count in failure_categories.most_common()
        )
        + ("\n" if failure_categories else ""),
        encoding="utf-8",
    )
    with results_jsonl.open("w", encoding="utf-8") as fh:
        for row in rows:
            fh.write(json.dumps(row, ensure_ascii=True) + "\n")

    unique_cases = sorted({row["relative_to_target"].split("/", 1)[0] for row in rows})
    by_case: dict[str, dict[str, bool]] = {}
    for row in rows:
        case_id = row["relative_to_target"].split("/", 1)[0]
        by_case.setdefault(case_id, {})[row["file_kind"]] = row["ok"]
    materialized_cases = 0
    both_prog_success = 0
    any_prog_success = 0
    for item in by_case.values():
        if "prog_a" in item and "prog_b" in item:
            materialized_cases += 1
            if item["prog_a"] and item["prog_b"]:
                both_prog_success += 1
            if item["prog_a"] or item["prog_b"]:
                any_prog_success += 1

    summary = {
        "requested_target": target_info["requested_path"],
        "resolved_target": target_info["resolved_path"],
        "resolution": target_info["resolution"],
        "target_note": target_info.get("note", ""),
        "dataset_repo": "../gobench-eq",
        "total_go_files": total,
        "success_count": len(success_rows),
        "failure_count": len(fail_rows),
        "success_rate": success_rate,
        "case_dirs": len(unique_cases),
        "materialized_cases": materialized_cases,
        "both_prog_success_cases": both_prog_success,
        "any_prog_success_cases": any_prog_success,
        "file_kind_breakdown": by_kind,
        "failure_stage_breakdown": dict(stage_counts),
        "top_failure_reasons": [
            {
                "reason": reason,
                "count": count,
                "example": failure_examples[reason],
            }
            for reason, count in failure_categories.most_common(20)
        ],
        "success_list": rel_to_root(success_list, root),
        "fail_list": rel_to_root(fail_list, root),
        "failure_reasons": rel_to_root(reasons_path, root),
        "results_jsonl": rel_to_root(results_jsonl, root),
        "methodology": {
            "main_path": (
                "mlse-go -emit=goir-like -> mlse-goir-llvm-exp -emit=llvm-dialect "
                "-> mlir-translate --mlir-to-llvmir -> opt verify"
            ),
            "path_comparison": [
                {
                    "path": "formal-go-dialect",
                    "status": "not-selected",
                    "notes": (
                        "cmd/mlse-go default output is formal MLIR, but the repo does not yet "
                        "have a bulk-capable formal go -> LLVM lowering path. Real dataset files "
                        "quickly reach custom go.* types, which mlse-opt can parse but generic "
                        "LLVM lowering cannot carry to actual LLVM IR."
                    ),
                },
                {
                    "path": "experimental-goir-llvm",
                    "status": "selected",
                    "notes": (
                        "This is the only MLSE-native path on this machine that can take a single "
                        "Go file to actual LLVM IR without requiring full module dependency "
                        "resolution. It is experimental and placeholder-heavy, but it is the most "
                        "realistic bulk probe for the current dataset layout."
                    ),
                },
                {
                    "path": "tinygo",
                    "status": "not-selected",
                    "notes": (
                        "TinyGo can emit actual LLVM IR, but direct file probes fail on dependency-"
                        "heavy goeq-dce programs and are a poor fit for harness/case_test.go. "
                        "Existing TinyGo scripts in this repo also assume package-context probing "
                        "and omit test files."
                    ),
                },
                {
                    "path": "existing-scripts",
                    "status": "reused-by-approach",
                    "notes": (
                        "The main reference scripts were scripts/goir-llvm-experiment.sh, "
                        "scripts/mlse-go-probe-local.sh, scripts/tinygo-probe-local.sh, and "
                        "scripts/tinygo-probe-repo.py. This probe reuses their methodology, but "
                        "adds an external gobench-eq bulk target and a file-by-file LLVM IR report."
                    ),
                },
            ],
            "output_kinds": {
                "frontend": "GoIR-like text",
                "middle": "LLVM dialect MLIR",
                "final": "actual LLVM IR text",
            },
            "limitations": [
                "The external checkout has no literal goeq-spec directory, so the probe resolved to ../gobench-eq/dataset/cases.",
                "This is the experimental MLSE GoIR-like path, not the formal go dialect path. Success means the file reached actual LLVM IR and passed opt verification, not that the lowering is semantically complete.",
                "The probe is file-based and does not require full module dependency resolution, which makes it suitable for bulk coverage probing but weaker as a semantic compiler conformance signal.",
                "The gobench-eq dataset currently contains many draft cases with harness-only Go files. Those files are included because the request asked for all Go files under the resolved target.",
            ],
        },
        "toolchain": tools,
    }
    summary_json.write_text(json.dumps(summary, indent=2) + "\n", encoding="utf-8")

    lines = [
        "# goeq LLVM IR Bulk Probe",
        "",
        "## Target Resolution",
        "",
        f"- Requested target: `{summary['requested_target']}`",
        f"- Resolved target: `{summary['resolved_target']}`",
        f"- Resolution mode: `{summary['resolution']}`",
    ]
    if summary["target_note"]:
        lines.append(f"- Note: {summary['target_note']}")

    lines.extend(
        [
            "",
            "## Main Probe Path",
            "",
            f"- Chosen path: `{summary['methodology']['main_path']}`",
            "- Output kinds:",
            f"  - frontend: `{summary['methodology']['output_kinds']['frontend']}`",
            f"  - middle: `{summary['methodology']['output_kinds']['middle']}`",
            f"  - final: `{summary['methodology']['output_kinds']['final']}`",
            "",
            "## Path Comparison",
            "",
        ]
    )
    for row in summary["methodology"]["path_comparison"]:
        lines.append(f"- `{row['path']}` [{row['status']}]: {row['notes']}")

    lines.extend(
        [
            "",
            "## Summary",
            "",
            f"- Total Go files: `{summary['total_go_files']}`",
            f"- Success count: `{summary['success_count']}`",
            f"- Failure count: `{summary['failure_count']}`",
            f"- Success rate: `{summary['success_rate']:.2%}`",
            f"- Case directories touched: `{summary['case_dirs']}`",
            f"- Materialized cases with both prog sides present: `{summary['materialized_cases']}`",
            f"- Cases with both prog sides successful: `{summary['both_prog_success_cases']}`",
            f"- Cases with at least one prog side successful: `{summary['any_prog_success_cases']}`",
            f"- Success list: `{summary['success_list']}`",
            f"- Fail list: `{summary['fail_list']}`",
            f"- Failure reasons: `{summary['failure_reasons']}`",
            f"- Raw results: `{summary['results_jsonl']}`",
            f"- Failure stage breakdown: `{summary['failure_stage_breakdown']}`",
            "",
            "## File Kind Breakdown",
            "",
        ]
    )
    for kind in sorted(by_kind):
        row = by_kind[kind]
        rate = (row["success"] / row["total"]) if row["total"] else 0.0
        lines.append(
            f"- `{kind}`: total `{row['total']}`, success `{row['success']}`, "
            f"failure `{row['failure']}`, success rate `{rate:.2%}`"
        )

    lines.extend(["", "## Top Failure Reasons", ""])
    if summary["top_failure_reasons"]:
        for row in summary["top_failure_reasons"]:
            lines.append(
                f"- `{row['count']}` x `{row['reason']}` (example: `{row['example']}`)"
            )
    else:
        lines.append("- none")

    lines.extend(["", "## Toolchain", ""])
    for name, info in tools.items():
        lines.append(
            f"- `{name}`: `{info['status']}` using `{info['label']}`"
        )

    lines.extend(["", "## Methodology Limitations", ""])
    for note in summary["methodology"]["limitations"]:
        lines.append(f"- {note}")

    report_md.write_text("\n".join(lines) + "\n", encoding="utf-8")
    return summary


def main() -> int:
    args = parse_args()
    root = Path(args.root).resolve()
    dataset_repo = Path(args.dataset_repo).resolve()
    artifact_dir = (root / args.artifact_dir).resolve()
    failure_log_dir = artifact_dir / "failure-logs"
    artifact_dir.mkdir(parents=True, exist_ok=True)
    failure_log_dir.mkdir(parents=True, exist_ok=True)
    (root / "tmp").mkdir(parents=True, exist_ok=True)

    probe_cache = root / "tmp" / "goeq-bulk-probe-cache"
    probe_modcache = root / "tmp" / "goeq-bulk-probe-modcache"
    probe_cache.mkdir(parents=True, exist_ok=True)
    probe_modcache.mkdir(parents=True, exist_ok=True)
    build_env = os.environ.copy()
    build_env["GOCACHE"] = str(probe_cache)
    build_env["GOMODCACHE"] = str(probe_modcache)

    target_dir, target_info = detect_target_dir(dataset_repo)
    outputs = build_binaries(root, build_env)

    mlir_opt_bin = discover_tool("MLIR_OPT_BIN", "mlir-opt")
    mlir_translate_bin = discover_tool("MLIR_TRANSLATE_BIN", "mlir-translate")
    opt_bin = discover_tool("OPT_BIN", "opt")
    llvm_as_bin = discover_tool("LLVM_AS_BIN", "llvm-as")
    tools = {
        "mlse-go": {"status": "available", "label": rel_to_root(outputs["mlse_go"], root)},
        "mlse-goir-llvm-exp": {
            "status": "available",
            "label": rel_to_root(outputs["mlse_goir_llvm_exp"], root),
        },
        "mlir-opt": {"status": "available" if mlir_opt_bin else "missing", "label": tool_label(mlir_opt_bin)},
        "mlir-translate": {
            "status": "available" if mlir_translate_bin else "missing",
            "label": tool_label(mlir_translate_bin),
        },
        "opt": {"status": "available" if opt_bin else "missing", "label": tool_label(opt_bin)},
        "llvm-as": {
            "status": "available" if llvm_as_bin else "missing",
            "label": tool_label(llvm_as_bin),
        },
    }

    files = sorted(path for path in target_dir.rglob("*.go") if path.is_file())
    rows: list[dict] = []
    worker = lambda p: probe_one(
        p,
        root=root,
        dataset_repo=dataset_repo,
        target_dir=target_dir,
        outputs=outputs,
        mlir_opt_bin=mlir_opt_bin,
        mlir_translate_bin=mlir_translate_bin,
        opt_bin=opt_bin,
        llvm_as_bin=llvm_as_bin,
        failure_log_dir=failure_log_dir,
    )
    with concurrent.futures.ThreadPoolExecutor(max_workers=max(1, args.jobs)) as executor:
        for row in executor.map(worker, files):
            rows.append(row)
    rows.sort(key=lambda row: row["path"])
    summarize(rows, root=root, artifact_dir=artifact_dir, target_info=target_info, tools=tools, dataset_repo=dataset_repo)
    print(rel_to_root(artifact_dir / "summary.json", root))
    print(rel_to_root(artifact_dir / "report.md", root))
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env python3
from __future__ import annotations

import argparse
import collections
import concurrent.futures
import json
import os
import re
import subprocess
import sys
from pathlib import Path


def parse_args() -> argparse.Namespace:
    parser = argparse.ArgumentParser()
    parser.add_argument("--mode", choices=["packages", "files"], required=True)
    parser.add_argument("--root", required=True)
    parser.add_argument("--repo-dir", required=True)
    parser.add_argument("--tinygo-bin", required=True)
    parser.add_argument("--work-dir", required=True)
    parser.add_argument("--out-dir", required=True)
    parser.add_argument("--artifact-dir", required=True)
    parser.add_argument("--target-name", required=True)
    parser.add_argument("--jobs", type=int, default=4)
    parser.add_argument("--home-dir", required=True)
    parser.add_argument("--go-build-cache", required=True)
    parser.add_argument("--go-mod-cache", required=True)
    parser.add_argument("--go-proxy", default="off")
    parser.add_argument("--go-sumdb", default="off")
    parser.add_argument("--go-flags", default="-mod=readonly")
    return parser.parse_args()


def rel(path: Path, root: Path) -> str:
    try:
        return str(path.relative_to(root))
    except ValueError:
        home = Path.home()
        try:
            return "~/" + str(path.relative_to(home))
        except ValueError:
            return str(path)


def sanitize_reason(text: str) -> str:
    text = re.sub(r"/Users/[^ \t\n\"']+", "<path>", text)
    return text


def safe_name(text: str) -> str:
    safe = re.sub(r"[^A-Za-z0-9._-]+", "__", text)
    return safe.strip("._") or "item"


def parse_json_stream(text: str) -> list[dict]:
    decoder = json.JSONDecoder()
    idx = 0
    objs = []
    while True:
        while idx < len(text) and text[idx].isspace():
            idx += 1
        if idx >= len(text):
            break
        obj, idx = decoder.raw_decode(text, idx)
        objs.append(obj)
    return objs


def discover_modules(repo_dir: Path) -> list[Path]:
    modules = []
    for go_mod in sorted(repo_dir.rglob("go.mod")):
        if ".tinygo-probe-stubs" in go_mod.parts:
            continue
        modules.append(go_mod.parent)
    return modules


def base_go_env(args: argparse.Namespace) -> dict[str, str]:
    env = os.environ.copy()
    env.update(
        {
            "HOME": args.home_dir,
            "GOCACHE": args.go_build_cache,
            "GOMODCACHE": args.go_mod_cache,
            "GOPROXY": args.go_proxy,
            "GOSUMDB": args.go_sumdb,
            "GOFLAGS": args.go_flags,
        }
    )
    return env


def load_target_metadata(repo_dir: Path) -> dict | None:
    stamp = repo_dir / ".mlse-target.json"
    if not stamp.exists():
        return None
    try:
        return json.loads(stamp.read_text())
    except json.JSONDecodeError:
        return None


def collect_packages(args: argparse.Namespace) -> list[dict]:
    repo_dir = Path(args.repo_dir)
    root = Path(args.root)
    env = base_go_env(args)
    rows = []
    seen = set()
    module_dirs = discover_modules(repo_dir)
    for module_dir in module_dirs:
        proc = subprocess.run(
            ["go", "list", "-e", "-json", "./..."],
            cwd=str(module_dir),
            env=env,
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
        )
        payload = proc.stdout
        objects = parse_json_stream(payload)
        for obj in objects:
            import_path = obj.get("ImportPath")
            name = obj.get("Name")
            directory = obj.get("Dir")
            if not import_path or not name or not directory:
                continue
            key = (str(module_dir), import_path, name)
            if key in seen:
                continue
            seen.add(key)
            go_files = sorted(set(obj.get("GoFiles", []) + obj.get("CgoFiles", [])))
            test_files = sorted(set(obj.get("TestGoFiles", []) + obj.get("XTestGoFiles", [])))
            rows.append(
                {
                    "import_path": import_path,
                    "name": name,
                    "module_dir": str(module_dir),
                    "module_dir_rel": rel(module_dir, root),
                    "dir": rel(Path(directory), root),
                    "go_files": go_files,
                    "test_files": test_files,
                }
            )
    return rows


def select_build_packages(packages: list[dict]) -> tuple[list[dict], list[dict]]:
    buildable = []
    skipped = []
    for pkg in packages:
        if pkg["name"].endswith("_test"):
            skipped.append({**pkg, "skip_reason": "test-only package"})
            continue
        if not pkg["go_files"] and pkg["name"] != "main":
            skipped.append({**pkg, "skip_reason": "no non-test source files"})
            continue
        buildable.append(pkg)
    return buildable, skipped


def extract_reason(output: str) -> str:
    lines = [sanitize_reason(line.strip()) for line in output.splitlines() if line.strip()]
    filtered = [line for line in lines if not line.startswith("go: downloading ")]
    if filtered:
        return filtered[-1]
    if lines:
        return lines[-1]
    return "unknown failure"


def ensure_stub(pkg: dict) -> tuple[Path, str]:
    module_dir = Path(pkg["module_dir"])
    stub_dir = module_dir / ".tinygo-probe-stubs" / safe_name(pkg["import_path"])
    stub_dir.mkdir(parents=True, exist_ok=True)
    main_go = stub_dir / "main.go"
    main_go.write_text(
        f'package main\nimport _ "{pkg["import_path"]}"\nfunc main() {{}}\n'
    )
    return stub_dir, "./" + str(stub_dir.relative_to(module_dir))


def build_package(pkg: dict, args: argparse.Namespace, root: Path) -> dict:
    tinygo = Path(args.tinygo_bin)
    out_dir = Path(args.out_dir)
    env = base_go_env(args)
    module_dir = Path(pkg["module_dir"])
    safe = safe_name(pkg["import_path"])
    ll = out_dir / f"{safe}.ll"
    log = out_dir / f"{safe}.log"

    if pkg["name"] == "main":
        method = "direct-main-package"
        build_target = pkg["import_path"]
    else:
        method = "blank-import-stub"
        _, build_target = ensure_stub(pkg)

    cmd = [str(tinygo), "build", "-o", str(ll), build_target]
    proc = subprocess.run(
        cmd,
        cwd=str(module_dir),
        env=env,
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
    )
    log.write_text(proc.stdout)
    ok = proc.returncode == 0 and ll.exists()
    if not ok and ll.exists():
        ll.unlink()
    result = {
        "import_path": pkg["import_path"],
        "name": pkg["name"],
        "module_dir": pkg["module_dir_rel"],
        "dir": pkg["dir"],
        "go_files": [str(Path(pkg["dir"]) / name) for name in pkg["go_files"]],
        "test_files": [str(Path(pkg["dir"]) / name) for name in pkg["test_files"]],
        "method": method,
        "ok": ok,
        "log_path": rel(log, root),
    }
    if ok:
        result["ll_path"] = rel(ll, root)
    else:
        result["reason"] = extract_reason(proc.stdout)
    return result


def write_list(path: Path, items: list[str]) -> None:
    path.write_text("\n".join(items) + ("\n" if items else ""))


def write_jsonl(path: Path, rows: list[dict]) -> None:
    with path.open("w") as f:
        for row in rows:
            f.write(json.dumps(row, ensure_ascii=True) + "\n")


def summarize_failures(rows: list[dict], key: str, reason_key: str = "reason") -> tuple[list[dict], list[str]]:
    reasons = collections.Counter()
    examples = {}
    for row in rows:
        reason = row.get(reason_key, "unknown failure")
        reasons[reason] += 1
        examples.setdefault(reason, row[key])
    top = [
        {"reason": reason, "count": count, "example": examples[reason]}
        for reason, count in reasons.most_common(20)
    ]
    full = [f"{count}\t{examples[reason]}\t{reason}" for reason, count in reasons.most_common()]
    return top, full


def tinygo_version(args: argparse.Namespace) -> str:
    proc = subprocess.run(
        [args.tinygo_bin, "version"],
        stdout=subprocess.PIPE,
        stderr=subprocess.STDOUT,
        text=True,
        check=False,
    )
    return proc.stdout.strip()


def package_summary(args: argparse.Namespace, rows: list[dict], skipped: list[dict], target_metadata: dict | None) -> tuple[dict, str, list[str]]:
    root = Path(args.root)
    work_dir = Path(args.work_dir)
    artifact_dir = Path(args.artifact_dir)
    success_list = work_dir / f"{args.target_name}-success.txt"
    fail_list = work_dir / f"{args.target_name}-fail.txt"
    results_jsonl = artifact_dir / f"{args.target_name}-results.jsonl"
    summary_json = artifact_dir / f"{args.target_name}-summary.json"
    summary_md = artifact_dir / f"{args.target_name}-summary.md"
    reasons_txt = artifact_dir / f"{args.target_name}-failure-reasons.txt"

    ok_rows = [row for row in rows if row["ok"]]
    fail_rows = [row for row in rows if not row["ok"]]
    write_list(success_list, [row["import_path"] for row in ok_rows])
    write_list(fail_list, [row["import_path"] for row in fail_rows])
    write_jsonl(results_jsonl, rows)
    top_failures, failure_lines = summarize_failures(fail_rows, "import_path")
    write_list(reasons_txt, failure_lines)

    success_file_set = sorted({name for row in ok_rows for name in row["go_files"]})
    summary = {
        "probe_mode": "package-or-stub",
        "repo_dir": rel(Path(args.repo_dir), root),
        "target_metadata": target_metadata,
        "tinygo_version": tinygo_version(args),
        "modules_total": len(discover_modules(Path(args.repo_dir))),
        "packages_total": len(rows),
        "main_packages_total": sum(1 for row in rows if row["name"] == "main"),
        "library_packages_total": sum(1 for row in rows if row["name"] != "main"),
        "skipped_packages": len(skipped),
        "packages_success": len(ok_rows),
        "packages_fail": len(fail_rows),
        "package_success_rate": (len(ok_rows) / len(rows)) if rows else 0.0,
        "go_files_in_successful_packages": len(success_file_set),
        "success_list": rel(success_list, root),
        "fail_list": rel(fail_list, root),
        "results_jsonl": rel(results_jsonl, root),
        "failure_reasons": rel(reasons_txt, root),
        "top_failure_reasons": top_failures,
        "methodology_notes": [
            "Library packages are compiled through generated blank-import stubs because TinyGo build expects a main package as the build entrypoint.",
            "Package discovery walks every go.mod under the etcd tree instead of only the root module.",
            "Builds run with HOME/GOCACHE/GOMODCACHE inside the repo workspace; GOPROXY is forced off so missing modules are reported as offline environment limits.",
        ],
    }
    summary_json.write_text(json.dumps(summary, indent=2) + "\n")
    md_lines = [
        "# TinyGo Package Probe Summary",
        "",
        f"- Repo dir: `{summary['repo_dir']}`",
        f"- TinyGo: `{summary['tinygo_version']}`",
        f"- Modules scanned: `{summary['modules_total']}`",
        f"- Packages scanned: `{summary['packages_total']}`",
        f"- Successful packages: `{summary['packages_success']}`",
        f"- Failed packages: `{summary['packages_fail']}`",
        f"- Package success rate: `{summary['package_success_rate']:.2%}`",
        f"- Go files covered by successful packages: `{summary['go_files_in_successful_packages']}`",
        f"- Success list: `{summary['success_list']}`",
        f"- Fail list: `{summary['fail_list']}`",
        f"- Results jsonl: `{summary['results_jsonl']}`",
        f"- Failure reasons: `{summary['failure_reasons']}`",
        "",
        "## Methodology",
        "",
    ]
    md_lines.extend(f"- {note}" for note in summary["methodology_notes"])
    if target_metadata:
        md_lines.extend(
            [
                "",
                "## Target Metadata",
                "",
                f"- Requested ref: `{target_metadata.get('requested_ref')}`",
                f"- Resolved commit: `{target_metadata.get('resolved_commit')}`",
                f"- Source repo: `{target_metadata.get('source_repo')}`",
                f"- Go: `{target_metadata.get('go')}`",
                f"- Toolchain: `{target_metadata.get('toolchain')}`",
            ]
        )
    md_lines.extend(["", "## Top Failure Reasons", ""])
    if top_failures:
        md_lines.extend(
            f"- `{row['count']}` × `{row['reason']}` (example: `{row['example']}`)"
            for row in top_failures
        )
    else:
        md_lines.append("- none")
    summary_md.write_text("\n".join(md_lines) + "\n")
    return summary, rel(summary_json, root), [rel(summary_md, root)]


def file_summary(args: argparse.Namespace, rows: list[dict], skipped: list[dict], target_metadata: dict | None) -> tuple[dict, str, list[str]]:
    root = Path(args.root)
    work_dir = Path(args.work_dir)
    artifact_dir = Path(args.artifact_dir)
    success_list = work_dir / f"{args.target_name}-success.txt"
    fail_list = work_dir / f"{args.target_name}-fail.txt"
    omitted_list = work_dir / f"{args.target_name}-omitted-test-files.txt"
    results_jsonl = artifact_dir / f"{args.target_name}-results.jsonl"
    summary_json = artifact_dir / f"{args.target_name}-summary.json"
    summary_md = artifact_dir / f"{args.target_name}-summary.md"
    reasons_txt = artifact_dir / f"{args.target_name}-failure-reasons.txt"

    file_rows = []
    omitted = []
    for row in rows:
        for file_name in row["go_files"]:
            file_rows.append(
                {
                    "path": file_name,
                    "package": row["import_path"],
                    "package_name": row["name"],
                    "module_dir": row["module_dir"],
                    "ok": row["ok"],
                    "reason": row.get("reason"),
                    "method": row["method"],
                    "log_path": row["log_path"],
                    "ll_path": row.get("ll_path"),
                }
            )
        for file_name in row["test_files"]:
            omitted.append(
                {
                    "path": file_name,
                    "package": row["import_path"],
                    "skip_reason": "test file omitted from package-context probe",
                }
            )

    success_rows = [row for row in file_rows if row["ok"]]
    fail_rows = [row for row in file_rows if not row["ok"]]
    write_list(success_list, [row["path"] for row in success_rows])
    write_list(fail_list, [row["path"] for row in fail_rows])
    write_list(omitted_list, [row["path"] for row in omitted])
    write_jsonl(results_jsonl, file_rows + omitted)
    top_failures, failure_lines = summarize_failures(fail_rows, "path")
    write_list(reasons_txt, failure_lines)

    summary = {
        "probe_mode": "package-context-file-attribution",
        "repo_dir": rel(Path(args.repo_dir), root),
        "target_metadata": target_metadata,
        "tinygo_version": tinygo_version(args),
        "modules_total": len(discover_modules(Path(args.repo_dir))),
        "package_contexts_total": len(rows),
        "covered_go_files": len(file_rows),
        "successful_go_files": len(success_rows),
        "failed_go_files": len(fail_rows),
        "covered_success_rate": (len(success_rows) / len(file_rows)) if file_rows else 0.0,
        "omitted_test_go_files": len(omitted),
        "success_list": rel(success_list, root),
        "fail_list": rel(fail_list, root),
        "omitted_list": rel(omitted_list, root),
        "results_jsonl": rel(results_jsonl, root),
        "failure_reasons": rel(reasons_txt, root),
        "top_failure_reasons": top_failures,
        "methodology_notes": [
            "Each non-test Go file inherits the build result of its package context instead of being compiled as a standalone source file.",
            "Library packages are compiled through generated blank-import stubs so TinyGo can analyze them without forcing package main.",
            "Test files are reported separately as omitted because the current probe does not run tinygo test.",
        ],
    }
    summary_json.write_text(json.dumps(summary, indent=2) + "\n")
    md_lines = [
        "# TinyGo File Probe Summary",
        "",
        f"- Repo dir: `{summary['repo_dir']}`",
        f"- TinyGo: `{summary['tinygo_version']}`",
        f"- Modules scanned: `{summary['modules_total']}`",
        f"- Package contexts compiled: `{summary['package_contexts_total']}`",
        f"- Covered non-test Go files: `{summary['covered_go_files']}`",
        f"- Successful files: `{summary['successful_go_files']}`",
        f"- Failed files: `{summary['failed_go_files']}`",
        f"- Covered success rate: `{summary['covered_success_rate']:.2%}`",
        f"- Omitted test Go files: `{summary['omitted_test_go_files']}`",
        f"- Success list: `{summary['success_list']}`",
        f"- Fail list: `{summary['fail_list']}`",
        f"- Omitted list: `{summary['omitted_list']}`",
        f"- Results jsonl: `{summary['results_jsonl']}`",
        f"- Failure reasons: `{summary['failure_reasons']}`",
        "",
        "## Methodology",
        "",
    ]
    md_lines.extend(f"- {note}" for note in summary["methodology_notes"])
    if target_metadata:
        md_lines.extend(
            [
                "",
                "## Target Metadata",
                "",
                f"- Requested ref: `{target_metadata.get('requested_ref')}`",
                f"- Resolved commit: `{target_metadata.get('resolved_commit')}`",
                f"- Source repo: `{target_metadata.get('source_repo')}`",
                f"- Go: `{target_metadata.get('go')}`",
                f"- Toolchain: `{target_metadata.get('toolchain')}`",
            ]
        )
    md_lines.extend(["", "## Top Failure Reasons", ""])
    if top_failures:
        md_lines.extend(
            f"- `{row['count']}` × `{row['reason']}` (example: `{row['example']}`)"
            for row in top_failures
        )
    else:
        md_lines.append("- none")
    summary_md.write_text("\n".join(md_lines) + "\n")
    return summary, rel(summary_json, root), [rel(summary_md, root)]


def main() -> int:
    args = parse_args()
    root = Path(args.root)
    Path(args.work_dir).mkdir(parents=True, exist_ok=True)
    Path(args.out_dir).mkdir(parents=True, exist_ok=True)
    Path(args.artifact_dir).mkdir(parents=True, exist_ok=True)
    Path(args.home_dir).mkdir(parents=True, exist_ok=True)
    Path(args.go_build_cache).mkdir(parents=True, exist_ok=True)
    Path(args.go_mod_cache).mkdir(parents=True, exist_ok=True)

    all_packages = collect_packages(args)
    buildable, skipped = select_build_packages(all_packages)

    results = []
    with concurrent.futures.ThreadPoolExecutor(max_workers=args.jobs) as executor:
        for row in executor.map(lambda pkg: build_package(pkg, args, root), buildable):
            results.append(row)
    results.sort(key=lambda row: row["import_path"])

    target_metadata = load_target_metadata(Path(args.repo_dir))
    if args.mode == "packages":
        _, summary_json, extra = package_summary(args, results, skipped, target_metadata)
    else:
        _, summary_json, extra = file_summary(args, results, skipped, target_metadata)
    print(summary_json)
    for item in extra:
        print(item)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())

#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

MLSE_GO_BIN=${MLSE_GO_BIN-}
GOIR_LLVM_BIN=${GOIR_LLVM_BIN-}
CLANG_BIN=${CLANG_BIN-}
OPT_BIN=${OPT_BIN-}
LLVM_AS_BIN=${LLVM_AS_BIN-}
MLIR_OPT_BIN=${MLIR_OPT_BIN-}
MLIR_TRANSLATE_BIN=${MLIR_TRANSLATE_BIN-}
ARTIFACT_DIR=${ARTIFACT_DIR:-$ROOT/artifacts/goir-llvm-exp}
SAVED_SUCCESS_DIR=${SAVED_SUCCESS_DIR:-$ROOT/testdata/goir-llvm-exp/successes}
GOCACHE=${GOCACHE:-$ROOT/tmp/go-build}
GOMODCACHE=${GOMODCACHE:-$ROOT/tmp/gomodcache}
export GOCACHE
export GOMODCACHE

rm -rf "$ARTIFACT_DIR" "$SAVED_SUCCESS_DIR"
mkdir -p "$ROOT/artifacts/bin" "$ARTIFACT_DIR" "$SAVED_SUCCESS_DIR" "$GOCACHE" "$GOMODCACHE"

if [[ -z "$MLSE_GO_BIN" ]]; then
  MLSE_GO_BIN="$ROOT/artifacts/bin/mlse-go"
fi
if [[ -z "$GOIR_LLVM_BIN" ]]; then
  GOIR_LLVM_BIN="$ROOT/artifacts/bin/mlse-goir-llvm-exp"
fi

go build -o "$MLSE_GO_BIN" ./cmd/mlse-go
go build -o "$GOIR_LLVM_BIN" ./cmd/mlse-goir-llvm-exp

resolve_candidate() {
  local candidate="$1"

  if [[ -z "$candidate" ]]; then
    return 1
  fi
  if [[ "$candidate" == */* ]]; then
    if [[ -x "$candidate" && ! -d "$candidate" ]]; then
      printf '%s\n' "$candidate"
      return 0
    fi
    return 1
  fi
  command -v "$candidate" 2>/dev/null || return 1
}

search_tool_glob() {
  local dir="$1"
  local base="$2"
  local match

  if [[ ! -d "$dir" ]]; then
    return 1
  fi

  shopt -s nullglob
  for match in "$dir/$base" "$dir/$base"-*; do
    if [[ -x "$match" && ! -d "$match" ]]; then
      printf '%s\n' "$match"
      shopt -u nullglob
      return 0
    fi
  done
  shopt -u nullglob
  return 1
}

discover_tool() {
  local path_var="$1"
  local source_var="$2"
  local configured="$3"
  local logical_name="$4"
  local resolved=""
  local source="missing"
  local dir
  local old_ifs

  if [[ -n "$configured" ]]; then
    if resolved=$(resolve_candidate "$configured"); then
      source="env"
    else
      resolved=""
      source="env-missing"
    fi
    printf -v "$path_var" '%s' "$resolved"
    printf -v "$source_var" '%s' "$source"
    return 0
  fi

  if resolved=$(resolve_candidate "$logical_name"); then
    source="path"
    printf -v "$path_var" '%s' "$resolved"
    printf -v "$source_var" '%s' "$source"
    return 0
  fi

  if resolved=$(xcrun --find "$logical_name" 2>/dev/null); then
    if resolved=$(resolve_candidate "$resolved"); then
      source="xcrun"
      printf -v "$path_var" '%s' "$resolved"
      printf -v "$source_var" '%s' "$source"
      return 0
    fi
  fi

  old_ifs="$IFS"
  IFS=:
  for dir in $PATH; do
    if resolved=$(search_tool_glob "$dir" "$logical_name"); then
      source="path-search"
      IFS="$old_ifs"
      printf -v "$path_var" '%s' "$resolved"
      printf -v "$source_var" '%s' "$source"
      return 0
    fi
  done
  IFS="$old_ifs"

  for dir in \
    "/opt/homebrew/opt/llvm/bin" \
    "/usr/local/opt/llvm/bin" \
    "/opt/homebrew/bin" \
    "/usr/local/bin"; do
    if resolved=$(search_tool_glob "$dir" "$logical_name"); then
      source="common-dir"
      printf -v "$path_var" '%s' "$resolved"
      printf -v "$source_var" '%s' "$source"
      return 0
    fi
  done

  printf -v "$path_var" '%s' ""
  printf -v "$source_var" '%s' "$source"
}

tool_label() {
  local path="$1"

  if [[ -z "$path" ]]; then
    printf '%s\n' ""
    return 0
  fi
  basename "$path"
}

discover_tool CLANG_BIN CLANG_SOURCE "$CLANG_BIN" "clang"
discover_tool OPT_BIN OPT_SOURCE "$OPT_BIN" "opt"
discover_tool LLVM_AS_BIN LLVM_AS_SOURCE "$LLVM_AS_BIN" "llvm-as"
discover_tool MLIR_OPT_BIN MLIR_OPT_SOURCE "$MLIR_OPT_BIN" "mlir-opt"
discover_tool MLIR_TRANSLATE_BIN MLIR_TRANSLATE_SOURCE "$MLIR_TRANSLATE_BIN" "mlir-translate"

SELECTED_VERIFIER_BIN=""
SELECTED_VERIFIER_LABEL=""
SELECTED_VERIFIER_KIND="unavailable"
if [[ -n "$OPT_BIN" ]]; then
  SELECTED_VERIFIER_BIN="$OPT_BIN"
  SELECTED_VERIFIER_LABEL=$(tool_label "$OPT_BIN")
  SELECTED_VERIFIER_KIND="opt"
elif [[ -n "$LLVM_AS_BIN" ]]; then
  SELECTED_VERIFIER_BIN="$LLVM_AS_BIN"
  SELECTED_VERIFIER_LABEL=$(tool_label "$LLVM_AS_BIN")
  SELECTED_VERIFIER_KIND="llvm-as"
fi

SELECTED_COMPILE_BIN="$CLANG_BIN"
SELECTED_COMPILE_LABEL=$(tool_label "$CLANG_BIN")

RESULTS_JSONL="$ARTIFACT_DIR/results.jsonl"
REPORT_JSON="$ARTIFACT_DIR/report.json"
REPORT_MD="$ARTIFACT_DIR/report.md"
SAVED_INDEX_JSON="$SAVED_SUCCESS_DIR/index.json"
SAVED_INDEX_MD="$SAVED_SUCCESS_DIR/index.md"
: >"$RESULTS_JSONL"

run_opt_verifier() {
  local input="$1"
  local log="$2"

  if "$OPT_BIN" -passes=verify -disable-output "$input" >"$log" 2>&1; then
    return 0
  fi

  if grep -Eq "Unknown command line argument|unknown pass name|for the --passes option" "$log"; then
    "$OPT_BIN" -verify -disable-output "$input" >"$log" 2>&1
    return $?
  fi
  return 1
}

run_verifier() {
  local input="$1"
  local log="$2"

  case "$SELECTED_VERIFIER_KIND" in
    opt)
      run_opt_verifier "$input" "$log"
      ;;
    llvm-as)
      "$LLVM_AS_BIN" -o /dev/null "$input" >"$log" 2>&1
      ;;
    *)
      return 1
      ;;
  esac
}

run_compile_check() {
  local input="$1"
  local object_file="$2"
  local log="$3"

  "$CLANG_BIN" -Wno-override-module -c "$input" -o "$object_file" >"$log" 2>&1
}

prepare_input() {
  local mode="$1"
  local source="$2"
  local fixture="$3"
  local input="$4"

  if [[ "$mode" == "source" ]]; then
    "$MLSE_GO_BIN" -emit=goir-like "$source" >"$input"
  else
    cp "$fixture" "$input"
  fi
}

append_result() {
  python3 - <<'PY' \
    "$RESULTS_JSONL" \
    "$1" "$2" "$3" "$4" "$5" "$6" "$7" "$8" "$9" \
    "${10}" "${11}" "${12}" "${13}" "${14}" "${15}" "${16}" "${17}" \
    "${18}" "${19}"
import json
import sys

(
    path,
    name,
    mode,
    expected,
    expectation_status,
    source,
    fixture,
    goir_path,
    llvm_ir_path,
    saved_goir_path,
    saved_llvm_ir_path,
    translation_status,
    translation_log,
    verification_status,
    verification_tool,
    verification_kind,
    verification_log,
    compile_status,
    compile_tool,
    compile_log,
) = sys.argv[1:21]

row = {
    "name": name,
    "mode": mode,
    "expected": expected,
    "expectation_status": expectation_status,
    "source": source,
    "fixture": fixture,
    "goir_path": goir_path,
    "llvm_ir_path": llvm_ir_path,
    "saved_goir_path": saved_goir_path,
    "saved_llvm_ir_path": saved_llvm_ir_path,
    "translation_status": translation_status,
    "translation_log": translation_log,
    "verification_status": verification_status,
    "verification_tool": verification_tool,
    "verification_kind": verification_kind,
    "verification_log": verification_log,
    "compile_status": compile_status,
    "compile_tool": compile_tool,
    "compile_log": compile_log,
}

with open(path, "a", encoding="utf-8") as fh:
    fh.write(json.dumps(row, ensure_ascii=False) + "\n")
PY
}

OVERALL_EXIT=0

run_case() {
  local name="$1"
  local mode="$2"
  local expected="$3"
  local source="$4"
  local fixture="$5"
  local input_rel="artifacts/goir-llvm-exp/$name.mlir"
  local llvm_ir_rel="artifacts/goir-llvm-exp/$name.ll"
  local saved_dir_rel="testdata/goir-llvm-exp/successes/$name"
  local saved_goir_rel="$saved_dir_rel/goir.mlir"
  local saved_llvm_ir_rel="$saved_dir_rel/llvm.ll"
  local translate_log_rel="artifacts/goir-llvm-exp/$name.translate.log"
  local verify_log_rel="artifacts/goir-llvm-exp/$name.verify.log"
  local clang_log_rel="artifacts/goir-llvm-exp/$name.clang.log"
  local object_file="$ARTIFACT_DIR/$name.o"
  local input="$ROOT/$input_rel"
  local llvm_ir="$ROOT/$llvm_ir_rel"
  local saved_dir="$ROOT/$saved_dir_rel"
  local saved_goir="$ROOT/$saved_goir_rel"
  local saved_llvm_ir="$ROOT/$saved_llvm_ir_rel"
  local translate_log="$ROOT/$translate_log_rel"
  local verify_log="$ROOT/$verify_log_rel"
  local clang_log="$ROOT/$clang_log_rel"
  local translation_status="failure"
  local verification_status="skipped"
  local verification_tool=""
  local verification_kind="skipped"
  local verification_log_rel=""
  local compile_status="skipped"
  local compile_tool=""
  local compile_log_rel=""
  local llvm_ir_record=""
  local saved_goir_record=""
  local saved_llvm_ir_record=""
  local expectation_status="matched"

  prepare_input "$mode" "$source" "$fixture" "$input"
  rm -f "$llvm_ir" "$object_file" "$verify_log" "$clang_log"

  if "$GOIR_LLVM_BIN" "$input" >"$llvm_ir" 2>"$translate_log"; then
    translation_status="success"
    llvm_ir_record="$llvm_ir_rel"
    mkdir -p "$saved_dir"
    cp "$input" "$saved_goir"
    cp "$llvm_ir" "$saved_llvm_ir"
    saved_goir_record="$saved_goir_rel"
    saved_llvm_ir_record="$saved_llvm_ir_rel"

    if [[ -n "$SELECTED_VERIFIER_BIN" ]]; then
      verification_tool="$SELECTED_VERIFIER_LABEL"
      verification_kind="$SELECTED_VERIFIER_KIND"
      verification_log_rel="$verify_log_rel"
      if run_verifier "$llvm_ir" "$verify_log"; then
        verification_status="success"
      else
        verification_status="failure"
        OVERALL_EXIT=1
      fi
    else
      verification_status="unavailable"
      verification_kind="unavailable"
    fi

    if [[ -n "$SELECTED_COMPILE_BIN" ]]; then
      compile_tool="$SELECTED_COMPILE_LABEL"
      compile_log_rel="$clang_log_rel"
      if run_compile_check "$llvm_ir" "$object_file" "$clang_log"; then
        compile_status="success"
      else
        compile_status="failure"
        OVERALL_EXIT=1
      fi
    else
      compile_status="unavailable"
    fi
  else
    rm -f "$llvm_ir" "$object_file"
  fi

  if [[ "$expected" == "translation_success" && "$translation_status" != "success" ]]; then
    expectation_status="mismatch"
    OVERALL_EXIT=1
  fi
  if [[ "$expected" == "translation_failure" && "$translation_status" != "failure" ]]; then
    expectation_status="mismatch"
    OVERALL_EXIT=1
  fi

  append_result \
    "$name" \
    "$mode" \
    "$expected" \
    "$expectation_status" \
    "$source" \
    "$fixture" \
    "$input_rel" \
    "$llvm_ir_record" \
    "$saved_goir_record" \
    "$saved_llvm_ir_record" \
    "$translation_status" \
    "$translate_log_rel" \
    "$verification_status" \
    "$verification_tool" \
    "$verification_kind" \
    "$verification_log_rel" \
    "$compile_status" \
    "$compile_tool" \
    "$compile_log_rel"
}

run_case "simple_add" "source" "translation_success" "examples/go/simple_add.go" "testdata/simple_add.mlir"
run_case "sign_if" "source" "translation_success" "examples/go/sign_if.go" "testdata/goir-llvm-exp/sign_if.mlir"
run_case \
  "choose_if_else" \
  "source" \
  "translation_success" \
  "examples/go/choose_if_else.go" \
  "testdata/goir-llvm-exp/choose_if_else.mlir"
run_case \
  "choose_merge" \
  "source" \
  "translation_success" \
  "examples/go/choose_merge.go" \
  "testdata/goir-llvm-exp/choose_merge.mlir"
run_case \
  "sum_for" \
  "source" \
  "translation_success" \
  "examples/go/sum_for.go" \
  "testdata/goir-llvm-exp/sum_for.mlir"
run_case \
  "switch_value" \
  "source" \
  "translation_success" \
  "examples/go/switch_value.go" \
  "testdata/goir-llvm-exp/switch_value.mlir"

if [[ -f "$ROOT/tmp/etcd/server/storage/backend/config_windows.go" ]]; then
  run_case \
    "etcd_mmap_size" \
    "source" \
    "translation_success" \
    "tmp/etcd/server/storage/backend/config_windows.go" \
    "testdata/goir-llvm-exp/mmap_size.mlir"
else
  run_case \
    "etcd_mmap_size" \
    "fixture" \
    "translation_success" \
    "" \
    "testdata/goir-llvm-exp/mmap_size.mlir"
fi

if [[ -f "$ROOT/tmp/etcd/client/pkg/fileutil/preallocate_unsupported.go" ]]; then
  run_case \
    "etcd_preallocate_unsupported" \
    "source" \
    "translation_success" \
    "tmp/etcd/client/pkg/fileutil/preallocate_unsupported.go" \
    "testdata/goir-llvm-exp/preallocate_unsupported.mlir"
else
  run_case \
    "etcd_preallocate_unsupported" \
    "fixture" \
    "translation_success" \
    "" \
    "testdata/goir-llvm-exp/preallocate_unsupported.mlir"
fi

if [[ -f "$ROOT/tmp/etcd/pkg/cpuutil/endian.go" ]]; then
  run_case \
    "etcd_byte_order_if" \
    "source" \
    "translation_failure" \
    "tmp/etcd/pkg/cpuutil/endian.go" \
    "testdata/goir-llvm-exp/byte_order_if.mlir"
else
  run_case \
    "etcd_byte_order_if" \
    "fixture" \
    "translation_failure" \
    "" \
    "testdata/goir-llvm-exp/byte_order_if.mlir"
fi

export OPT_BIN OPT_SOURCE LLVM_AS_BIN LLVM_AS_SOURCE MLIR_OPT_BIN MLIR_OPT_SOURCE
export MLIR_TRANSLATE_BIN MLIR_TRANSLATE_SOURCE CLANG_BIN CLANG_SOURCE
export SELECTED_VERIFIER_LABEL SELECTED_VERIFIER_KIND SELECTED_COMPILE_LABEL

python3 - <<'PY' "$RESULTS_JSONL" "$REPORT_JSON" "$REPORT_MD" "$SAVED_INDEX_JSON" "$SAVED_INDEX_MD"
import json
import os
import pathlib
import sys

results_path = pathlib.Path(sys.argv[1])
report_json = pathlib.Path(sys.argv[2])
report_md = pathlib.Path(sys.argv[3])
saved_index_json = pathlib.Path(sys.argv[4])
saved_index_md = pathlib.Path(sys.argv[5])
rows = [json.loads(line) for line in results_path.read_text(encoding="utf-8").splitlines() if line.strip()]
saved_rows = [row for row in rows if row["saved_goir_path"] and row["saved_llvm_ir_path"]]

def tool_info(env_key: str, source_key: str, used_for: str) -> dict:
    path = os.environ.get(env_key, "")
    source = os.environ.get(source_key, "missing")
    label = pathlib.Path(path).name if path else ""
    return {
        "status": "available" if path else "missing",
        "tool": label,
        "discovery": source,
        "used_for": used_for,
    }

toolchain = {
    "opt": tool_info("OPT_BIN", "OPT_SOURCE", "preferred-verifier"),
    "llvm_as": tool_info("LLVM_AS_BIN", "LLVM_AS_SOURCE", "fallback-verifier"),
    "mlir_opt": tool_info("MLIR_OPT_BIN", "MLIR_OPT_SOURCE", "inspection-only"),
    "mlir_translate": tool_info("MLIR_TRANSLATE_BIN", "MLIR_TRANSLATE_SOURCE", "inspection-only"),
    "clang": tool_info("CLANG_BIN", "CLANG_SOURCE", "compile-check"),
}

translation_successes = sum(1 for row in rows if row["translation_status"] == "success")
translation_failures = sum(1 for row in rows if row["translation_status"] == "failure")
expected_translation_failures = sum(
    1
    for row in rows
    if row["expected"] == "translation_failure" and row["translation_status"] == "failure"
)
unexpected_translation_failures = sum(
    1
    for row in rows
    if row["expected"] == "translation_success" and row["translation_status"] != "success"
)
unexpected_translation_successes = sum(
    1
    for row in rows
    if row["expected"] == "translation_failure" and row["translation_status"] != "failure"
)
verification_successes = sum(1 for row in rows if row["verification_status"] == "success")
verification_failures = sum(1 for row in rows if row["verification_status"] == "failure")
verification_unavailable = sum(1 for row in rows if row["verification_status"] == "unavailable")
compile_successes = sum(1 for row in rows if row["compile_status"] == "success")
compile_failures = sum(1 for row in rows if row["compile_status"] == "failure")
compile_unavailable = sum(1 for row in rows if row["compile_status"] == "unavailable")

eligible_for_validation = sum(1 for row in rows if row["translation_status"] == "success")
dedicated_verifier_coverage = sum(
    1 for row in rows if row["translation_status"] == "success" and row["verification_status"] in {"success", "failure"}
)
compile_only_coverage = sum(
    1
    for row in rows
    if row["translation_status"] == "success"
    and row["verification_status"] == "unavailable"
    and row["compile_status"] in {"success", "failure"}
)

selected_verifier = os.environ.get("SELECTED_VERIFIER_LABEL", "")
selected_verifier_kind = os.environ.get("SELECTED_VERIFIER_KIND", "unavailable")
selected_compile = os.environ.get("SELECTED_COMPILE_LABEL", "")
saved_root = "testdata/goir-llvm-exp/successes"

saved_index = {
    "saved_root": saved_root,
    "saved_pairs": len(saved_rows),
    "samples": [
        {
            "name": row["name"],
            "mode": row["mode"],
            "origin_path": row["source"] or row["fixture"],
            "source_path": row["source"],
            "fixture_path": row["fixture"],
            "saved_goir_path": row["saved_goir_path"],
            "saved_llvm_ir_path": row["saved_llvm_ir_path"],
            "verification_status": row["verification_status"],
            "compile_status": row["compile_status"],
        }
        for row in saved_rows
    ],
}

summary = {
    "overall_status": "success"
    if not (unexpected_translation_failures or unexpected_translation_successes or verification_failures or compile_failures)
    else "failure",
    "samples_total": len(rows),
    "translation_successes": translation_successes,
    "translation_failures": translation_failures,
    "expected_translation_failures": expected_translation_failures,
    "unexpected_translation_failures": unexpected_translation_failures,
    "unexpected_translation_successes": unexpected_translation_successes,
    "verification_successes": verification_successes,
    "verification_failures": verification_failures,
    "verification_unavailable": verification_unavailable,
    "compile_successes": compile_successes,
    "compile_failures": compile_failures,
    "compile_unavailable": compile_unavailable,
    "saved_success_root": saved_root,
    "saved_success_index_json": "testdata/goir-llvm-exp/successes/index.json",
    "saved_success_index_md": "testdata/goir-llvm-exp/successes/index.md",
    "saved_success_pairs": len(saved_rows),
    "toolchain": toolchain,
    "verifier_coverage": {
        "eligible_samples": eligible_for_validation,
        "dedicated_verifier_samples": dedicated_verifier_coverage,
        "compile_only_samples": compile_only_coverage,
        "selected_verifier_tool": selected_verifier,
        "selected_verifier_kind": selected_verifier_kind,
        "selected_compile_tool": selected_compile,
        "notes": (
            "The current experiment emits LLVM IR text directly. "
            "mlir-opt/mlir-translate are inspected for local availability but are not on the active validation path."
        ),
    },
    "samples": rows,
    "working_subset": [
        "single-result functions",
        "i1/i32/i64 scalars",
        "opaque pointer-like Go types lowered as ptr",
        "direct return of literals, mutable locals, and nil",
        "top-level local initialization and reassignment lowered through stack slots",
        "arith.addi/subi/muli/divsi and integer comparisons",
        "mlse.if with i1 SSA/literal conditions",
        "mlse.if with inline arith.cmpi_* conditions lowered to br/ret basic blocks",
        "branch value merges through predeclared locals assigned in both if/else paths",
        "mlse.for with supported conditions and body updates to predeclared locals",
        "mlse.switch on integer/bool tags with single-value cases and optional default",
        "mlse.call with inferred extern declarations",
    ],
    "unsupported_subset": [
        "mlse.if/mlse.for conditions outside i1 or inline arith.cmpi_*",
        "new locals introduced inside if/for/switch control-flow bodies",
        "branch-local SSA values merged across control-flow joins",
        "mlse.range and other non-if/non-for/non-switch control-flow forms",
        "mlse.switch with multi-value cases, fallthrough, or non-scalar tags",
        "mlse.select and other opaque expression placeholders",
        "multi-result functions or returns",
        "break/continue and other explicit branch placeholders",
    ],
    "recommendation": (
        "Keep opt as the preferred dedicated verifier path, fall back to llvm-as when needed, "
        "and treat clang compile checks as a separate signal."
    ),
}

report_json.write_text(json.dumps(summary, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")
saved_index_json.write_text(json.dumps(saved_index, indent=2, ensure_ascii=False) + "\n", encoding="utf-8")

lines = [
    "# GoIR to LLVM IR Experiment",
    "",
    f"- Overall status: `{summary['overall_status']}`",
    f"- Samples run: `{summary['samples_total']}`",
    f"- Translation successes: `{summary['translation_successes']}`",
    f"- Expected translation failures: `{summary['expected_translation_failures']}`",
    f"- Dedicated verifier successes: `{summary['verification_successes']}`",
    f"- Dedicated verifier unavailable after translation: `{summary['verification_unavailable']}`",
    f"- Compile successes: `{summary['compile_successes']}`",
    f"- Durable saved success pairs: `{summary['saved_success_pairs']}`",
    f"- Durable success index: `{summary['saved_success_index_md']}`",
    "",
    "## Tool Availability",
    "",
]

for name, info in toolchain.items():
    label = info["tool"] or "unavailable"
    lines.append(
        f"- `{name}`: `{info['status']}` via `{info['discovery']}` using `{label}` for `{info['used_for']}`"
    )

lines.extend(
    [
        "",
        "## Selected Validation Path",
        "",
        f"- Dedicated verifier: `{selected_verifier or 'none'}` (`{selected_verifier_kind}`)",
        f"- Compile check: `{selected_compile or 'none'}`",
        f"- Dedicated verifier coverage: `{dedicated_verifier_coverage}/{eligible_for_validation}` translated samples",
        f"- Compile-only coverage: `{compile_only_coverage}/{eligible_for_validation}` translated samples",
        "",
        "## Sample Results",
        "",
    ]
)

for row in rows:
    source = row["source"] or row["fixture"]
    lines.append(
        f"- `{row['name']}`: expectation `{row['expectation_status']}`, "
        f"translation `{row['translation_status']}`, "
        f"verifier `{row['verification_status']}`, "
        f"compile `{row['compile_status']}` via `{row['mode']}` input `{source}`"
    )

lines.extend(
    [
        "",
        "## Saved Success Pairs",
        "",
    ]
)
for row in saved_rows:
    source = row["source"] or row["fixture"]
    lines.append(
        f"- `{row['name']}`: `{source}` -> `{row['saved_goir_path']}` and `{row['saved_llvm_ir_path']}` "
        f"(verifier `{row['verification_status']}`, compile `{row['compile_status']}`)"
    )

lines.extend(
    [
        "",
        "## Working Subset",
        "",
    ]
)
lines.extend(f"- {item}" for item in summary["working_subset"])
lines.extend(
    [
        "",
        "## Unsupported Subset",
        "",
    ]
)
lines.extend(f"- {item}" for item in summary["unsupported_subset"])
lines.extend(
    [
        "",
        "## Recommendation",
        "",
        f"- {summary['recommendation']}",
        "",
    ]
)

report_md.write_text("\n".join(lines), encoding="utf-8")

saved_lines = [
    "# Saved GoIR/LLVM IR Success Pairs",
    "",
    f"- Saved root: `{saved_root}`",
    f"- Successful pairs: `{len(saved_rows)}`",
    "",
    "## Index",
    "",
]

for row in saved_index["samples"]:
    saved_lines.append(
        f"- `{row['name']}`: `{row['origin_path']}` -> `{row['saved_goir_path']}` and `{row['saved_llvm_ir_path']}` "
        f"(verifier `{row['verification_status']}`, compile `{row['compile_status']}`)"
    )

saved_index_md.write_text("\n".join(saved_lines), encoding="utf-8")
print(report_json)
print(report_md)
print(saved_index_json)
print(saved_index_md)
PY

echo "$REPORT_JSON"
echo "$REPORT_MD"
echo "$SAVED_INDEX_JSON"
echo "$SAVED_INDEX_MD"

exit "$OVERALL_EXIT"

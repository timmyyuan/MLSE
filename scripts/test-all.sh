#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

print_section() {
  printf '\n[%s]\n' "$1"
}

display_path() {
  local path=$1
  printf '%s\n' "${path#$ROOT/}"
}

run_checked() {
  local label=$1
  shift
  local log
  log=$(mktemp "${TMPDIR:-/tmp}/mlse-test-all.XXXXXX")

  if "$@" >"$log" 2>&1; then
    printf 'PASS %s\n' "$label"
    rm -f "$log"
    return 0
  fi

  printf 'FAIL %s\n' "$label" >&2
  cat "$log" >&2
  rm -f "$log"
  return 1
}

run_checked_to_file() {
  local label=$1
  local out_file=$2
  shift 2
  local log
  log=$(mktemp "${TMPDIR:-/tmp}/mlse-test-all.XXXXXX")

  if "$@" >"$out_file" 2>"$log"; then
    printf 'PASS %s\n' "$label"
    rm -f "$log"
    return 0
  fi

  printf 'FAIL %s\n' "$label" >&2
  cat "$log" >&2
  rm -f "$log"
  return 1
}

run_mlse_opt_fixture_tests() {
  local mlse_opt=$1
  local path
  local total=0

  while IFS= read -r path; do
    total=$((total + 1))
    run_checked "$(display_path "$path")" "$mlse_opt" "$path"
  done < <(find "$ROOT/test/GoIR/ir" -maxdepth 1 -type f -name '*.mlir' | sort)

  printf 'summary: %d/%d passed\n' "$total" "$total"
}

run_frontend_bridge_tests() {
  local mlse_opt=$1
  local mlse_go=$2
  local out_dir=$3
  local path
  local total=0

  mkdir -p "$out_dir"
  for path in \
    "$ROOT/examples/go/simple_add.go" \
    "$ROOT/examples/go/sign_if.go" \
    "$ROOT/examples/go/choose_merge.go" \
    "$ROOT/examples/go/sum_for.go"; do
    total=$((total + 1))
    local name
    name=$(basename "$path" .go)
    local out="$out_dir/$name.formal.mlir"
    run_checked_to_file "$(display_path "$path") [mlse-go]" "$out" "$mlse_go" "$path"
    run_checked "$(display_path "$path") [mlse-opt]" "$mlse_opt" "$out"
  done

  printf 'summary: %d/%d passed\n' "$total" "$total"
}

run_go_builtin_lowering_tests() {
  local mlse_opt=$1
  local out_dir=$2
  local input="$ROOT/test/GoIR/ir/bootstrap_ops.mlir"
  local lowered="$out_dir/bootstrap_ops.lowered.mlir"

  mkdir -p "$out_dir"
  run_checked_to_file "$(display_path "$input") [lower-go-builtins]" "$lowered" \
    "$mlse_opt" --lower-go-builtins "$input"

  if rg -n '(^|[[:space:](,=])go\.(len|cap|index|append|append_slice)\b' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-builtins check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if ! rg -n '@runtime\.go\.(len|cap|index|append|append_slice)' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-builtins check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  printf 'PASS %s\n' "$(display_path "$input") [lower-go-builtins check]"
  printf 'summary: 1/1 passed\n'
}

run_go_bootstrap_lowering_tests() {
  local mlse_opt=$1
  local out_dir=$2
  local input="$ROOT/test/GoIR/ir/bootstrap_ops.mlir"
  local lowered="$out_dir/bootstrap_ops.go-lowered.mlir"

  mkdir -p "$out_dir"
  run_checked_to_file "$(display_path "$input") [lower-go-bootstrap]" "$lowered" \
    "$mlse_opt" --lower-go-bootstrap "$input"

  if rg -n '(^|[[:space:](,=])go\.[A-Za-z_][A-Za-z0-9_]*|!go\.' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if ! rg -n 'llvm\.extractvalue|llvm\.getelementptr|llvm\.load|llvm\.store|llvm\.intr\.memmove' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap direct-memory check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if ! rg -n 'cf\.cond_br|llvm\.unreachable' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap bounds-control check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if rg -n '@runtime\.go\.(len|cap|index|elem_addr|load|store|append_slice)' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap helper regression]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if rg -n '@runtime\.go\.append' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap append helper regression]" >&2
    cat "$lowered" >&2
    return 1
  fi

  if ! rg -n '@(runtime\.makeslice|runtime\.field\.addr|runtime\.growslice|runtime\.panic\.index)' "$lowered" >/dev/null; then
    echo "FAIL $(display_path "$input") [lower-go-bootstrap check]" >&2
    cat "$lowered" >&2
    return 1
  fi

  printf 'PASS %s\n' "$(display_path "$input") [lower-go-bootstrap check]"
  printf 'summary: 1/1 passed\n'
}

run_go_exec_diff_tests() {
  local script="$ROOT/scripts/go-exec-diff-suite.py"
  run_checked "go-exec-diff-suite" python3 "$script" --skip-build
  printf 'summary: 1/1 passed\n'
}

print_section "go"
"$ROOT/scripts/test.sh"
go test ./linters

print_section "build"
"$ROOT/scripts/build.sh"

print_section "mlir-build"
"$ROOT/scripts/build-mlir.sh"

MLSE_OPT=${MLSE_OPT:-$ROOT/tmp/cmake-mlir-build/tools/mlse-opt/mlse-opt}
MLSE_GO=${MLSE_GO:-$ROOT/artifacts/bin/mlse-go}
MLSE_RUN=${MLSE_RUN:-$ROOT/tmp/cmake-mlir-build/tools/mlse-run/mlse-run}
OUT_DIR=${OUT_DIR:-$ROOT/tmp/test-all}

if [[ ! -x "$MLSE_OPT" ]]; then
  echo "error: mlse-opt not found at $MLSE_OPT" >&2
  exit 1
fi

if [[ ! -x "$MLSE_GO" ]]; then
  echo "error: mlse-go not found at $MLSE_GO" >&2
  exit 1
fi

if [[ ! -x "$MLSE_RUN" ]]; then
  echo "error: mlse-run not found at $MLSE_RUN" >&2
  exit 1
fi

print_section "mlir-fixtures"
run_mlse_opt_fixture_tests "$MLSE_OPT"

print_section "go-builtin-lowering"
run_go_builtin_lowering_tests "$MLSE_OPT" "$OUT_DIR"

print_section "go-bootstrap-lowering"
run_go_bootstrap_lowering_tests "$MLSE_OPT" "$OUT_DIR"

print_section "frontend-bridge"
run_frontend_bridge_tests "$MLSE_OPT" "$MLSE_GO" "$OUT_DIR"

print_section "go-exec-diff"
run_go_exec_diff_tests

echo
echo "all tests passed"

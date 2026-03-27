#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$(cd "$(dirname "$0")" && pwd)/common.sh"

print_section "go"

go_files=()
while IFS= read -r path; do
  go_files+=("$path")
done < <(
  find "$ROOT/cmd" "$ROOT/internal" "$ROOT/examples" "$ROOT/linters" \
    -type f -name '*.go' ! -path '*/testdata/*' | sort
)

if ((${#go_files[@]} > 0)); then
  unformatted=$(gofmt -l "${go_files[@]}")
  if [[ -n "$unformatted" ]]; then
    printf 'gofmt required for:\n%s\n' "$unformatted" >&2
    exit 1
  fi
fi

(
  cd "$ROOT"
  go vet ./cmd/... ./internal/...
  go run ./linters/check_go_metrics.go \
    --root "$ROOT" \
    --include "cmd,internal,examples,linters" \
    --exclude "testdata,tmp,artifacts,.git" \
    --max-params "$MAX_PARAMS" \
    --max-function-lines "$MAX_FUNCTION_LINES" \
    --max-file-lines "$MAX_FILE_LINES"
)

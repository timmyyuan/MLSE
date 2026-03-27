#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$(cd "$(dirname "$0")" && pwd)/common.sh"

print_section "cpp"

python3 "$ROOT/linters/check_cpp_metrics.py" \
  --root "$ROOT" \
  --include "include,lib,tools" \
  --exclude "tmp,artifacts,.git" \
  --max-params "$MAX_PARAMS" \
  --max-function-lines "$MAX_FUNCTION_LINES" \
  --max-file-lines "$MAX_FILE_LINES"

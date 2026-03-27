#!/usr/bin/env bash
set -euo pipefail

# shellcheck source=/dev/null
source "$(cd "$(dirname "$0")" && pwd)/common.sh"

print_section "python"

py_files=()
while IFS= read -r path; do
  py_files+=("$path")
done < <(
  find "$ROOT/scripts" "$ROOT/linters" \
    -type f -name '*.py' ! -path '*/tmp/*' ! -path '*/artifacts/*' | sort
)

if ((${#py_files[@]} > 0)); then
  python3 -m py_compile "${py_files[@]}"
fi

python3 "$ROOT/linters/check_python_metrics.py" \
  --root "$ROOT" \
  --include "scripts,linters" \
  --exclude "tmp,artifacts,.git" \
  --max-params "$MAX_PARAMS" \
  --max-function-lines "$MAX_FUNCTION_LINES" \
  --max-file-lines "$MAX_FILE_LINES"

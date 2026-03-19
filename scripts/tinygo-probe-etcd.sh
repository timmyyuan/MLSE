#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

IMAGE_NAME=${IMAGE_NAME:-mlse-tinygo:dev}
REPO_URL=${REPO_URL:-https://github.com/etcd-io/etcd.git}
REPO_DIR="$ROOT/tmp/etcd"
WORK_DIR="$ROOT/tmp/tinygo-work"
OUT_DIR="$ROOT/tmp/tinygo-out"
ARTIFACT_DIR="$ROOT/artifacts/tinygo"
REPORT_JSON="$ARTIFACT_DIR/etcd-probe-report.json"
REPORT_MD="$ARTIFACT_DIR/etcd-probe-report.md"

mkdir -p "$ROOT/tmp" "$WORK_DIR" "$OUT_DIR" "$ARTIFACT_DIR"

if [ ! -d "$REPO_DIR/.git" ]; then
  git clone --depth=1 "$REPO_URL" "$REPO_DIR"
else
  git -C "$REPO_DIR" pull --ff-only || true
fi

GO_FILES_LIST="$WORK_DIR/go-files.txt"
SUCCESS_LIST="$WORK_DIR/success.txt"
FAIL_LIST="$WORK_DIR/fail.txt"
: > "$SUCCESS_LIST"
: > "$FAIL_LIST"
find "$REPO_DIR" -type f -name '*.go' | sort > "$GO_FILES_LIST"
TOTAL=$(wc -l < "$GO_FILES_LIST" | tr -d ' ')

while IFS= read -r file; do
  rel=${file#"$REPO_DIR"/}
  base=${rel//\//__}
  out="$OUT_DIR/${base%.go}.ll"
  log="$OUT_DIR/${base%.go}.log"
  if docker run --rm \
      -v "$ROOT":/workspace \
      -w /workspace \
      "$IMAGE_NAME" \
      bash -lc "mkdir -p \"$(dirname "$out")\" && tinygo build -o \"$out\" -target=llvm-ir \"$file\"" >"$log" 2>&1; then
    echo "$rel" >> "$SUCCESS_LIST"
  else
    echo "$rel" >> "$FAIL_LIST"
    rm -f "$out"
  fi
done < "$GO_FILES_LIST"

SUCCESS=$(wc -l < "$SUCCESS_LIST" | tr -d ' ')
FAIL=$(wc -l < "$FAIL_LIST" | tr -d ' ')

python3 - <<PY
import json
from pathlib import Path
root = Path(${ROOT@Q})
report = {
  'project': 'etcd',
  'repo_url': ${REPO_URL@Q},
  'repo_dir': str(Path(${REPO_DIR@Q}).relative_to(root)),
  'total_go_files': int(${TOTAL}),
  'success_count': int(${SUCCESS}),
  'fail_count': int(${FAIL}),
  'success_list': str(Path(${SUCCESS_LIST@Q}).relative_to(root)),
  'fail_list': str(Path(${FAIL_LIST@Q}).relative_to(root)),
  'llvm_ir_dir': str(Path(${OUT_DIR@Q}).relative_to(root)),
}
Path(${REPORT_JSON@Q}).write_text(json.dumps(report, indent=2) + '\n')
Path(${REPORT_MD@Q}).write_text(
    '# TinyGo etcd Probe\n\n'
    f"- Total Go files: `{report['total_go_files']}`\n"
    f"- Successful LLVM IR emissions: `{report['success_count']}`\n"
    f"- Failed emissions: `{report['fail_count']}`\n"
    f"- Success list: `{report['success_list']}`\n"
    f"- Fail list: `{report['fail_list']}`\n"
    f"- LLVM IR output dir: `{report['llvm_ir_dir']}`\n"
)
PY

echo "report: $REPORT_JSON"

#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

MLSE_GO_BIN=${MLSE_GO_BIN:-$ROOT/artifacts/bin/mlse-go}
REPO_DIR=${REPO_DIR:-$ROOT/tmp/etcd}
WORK_DIR=${WORK_DIR:-$ROOT/tmp/mlse-go-work}
OUT_DIR=${OUT_DIR:-$ROOT/tmp/mlse-go-out}
ARTIFACT_DIR=${ARTIFACT_DIR:-$ROOT/artifacts/mlse-go}
TARGET_NAME=${TARGET_NAME:-etcd-mlse-go-probe}

mkdir -p "$WORK_DIR" "$OUT_DIR" "$ARTIFACT_DIR" "$ROOT/artifacts/bin"
go build -o "$ROOT/artifacts/bin/mlse-go" ./cmd/mlse-go

GO_LIST="$WORK_DIR/${TARGET_NAME}-go-files.txt"
SUCCESS_LIST="$WORK_DIR/${TARGET_NAME}-success.txt"
FAIL_LIST="$WORK_DIR/${TARGET_NAME}-fail.txt"
SUMMARY_JSON="$ARTIFACT_DIR/${TARGET_NAME}-summary.json"
SUMMARY_MD="$ARTIFACT_DIR/${TARGET_NAME}-summary.md"
REASONS_TXT="$ARTIFACT_DIR/${TARGET_NAME}-failure-reasons.txt"

find "$REPO_DIR" -type f -name '*.go' | sort > "$GO_LIST"

python3 - <<'PY' "$GO_LIST" "$MLSE_GO_BIN" "$REPO_DIR" "$OUT_DIR" "$SUCCESS_LIST" "$FAIL_LIST"
import subprocess, sys
from pathlib import Path
files=[Path(x) for x in Path(sys.argv[1]).read_text().splitlines() if x.strip()]
mlse=Path(sys.argv[2])
repo=Path(sys.argv[3])
out=Path(sys.argv[4])
succ=Path(sys.argv[5])
fail=Path(sys.argv[6])
oks=[]
fails=[]
for path in files:
    rel=path.relative_to(repo)
    base=str(rel).replace('/', '__')[:-3]
    mlir=out/f'{base}.mlir'
    log=out/f'{base}.log'
    proc=subprocess.run([str(mlse), str(path)], stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
    log.write_text(proc.stdout)
    if proc.returncode==0:
        mlir.write_text(proc.stdout)
        oks.append(str(rel))
    else:
        fails.append(str(rel))
        if mlir.exists():
            mlir.unlink()
succ.write_text(('\n'.join(oks)+'\n') if oks else '')
fail.write_text(('\n'.join(fails)+'\n') if fails else '')
print(f'total={len(files)} success={len(oks)} fail={len(fails)}')
PY

python3 - <<'PY' "$OUT_DIR" "$SUCCESS_LIST" "$FAIL_LIST" "$SUMMARY_JSON" "$SUMMARY_MD" "$REASONS_TXT"
import collections, json, re, sys
from pathlib import Path
out=Path(sys.argv[1])
succ=Path(sys.argv[2])
fail=Path(sys.argv[3])
js=Path(sys.argv[4])
md=Path(sys.argv[5])
reasons_txt=Path(sys.argv[6])
oks=[x for x in succ.read_text().splitlines() if x.strip()] if succ.exists() else []
fails=[x for x in fail.read_text().splitlines() if x.strip()] if fail.exists() else []
reasons=collections.Counter()
examples={}
for rel in fails:
    base=rel.replace('/', '__')[:-3]
    log=out/f'{base}.log'
    text=log.read_text(errors='ignore') if log.exists() else ''
    lines=[ln.strip() for ln in text.splitlines() if ln.strip()]
    reason=lines[-1] if lines else 'unknown failure'
    reason=re.sub(r'/Users/[^ ]+','<path>', reason)
    reasons[reason]+=1
    examples.setdefault(reason, rel)
summary={
    'total_go_files': len(oks)+len(fails),
    'success_count': len(oks),
    'fail_count': len(fails),
    'success_rate': (len(oks)/(len(oks)+len(fails)) if (len(oks)+len(fails)) else 0.0),
    'success_list': str(succ),
    'fail_list': str(fail),
    'top_failure_reasons': [
        {'reason': r, 'count': c, 'example': examples[r]}
        for r,c in reasons.most_common(20)
    ]
}
js.write_text(json.dumps(summary, indent=2)+'\n')
reasons_txt.write_text(('\n'.join(f'{c}\t{examples[r]}\t{r}' for r,c in reasons.most_common())+'\n') if reasons else '')
md.write_text(
    '# MLSE GoIR Probe Summary\n\n'
    f'- Total Go files: `{summary["total_go_files"]}`\n'
    f'- Successful files: `{summary["success_count"]}`\n'
    f'- Failed files: `{summary["fail_count"]}`\n'
    f'- Success rate: `{summary["success_rate"]:.2%}`\n'
    f'- Success list: `{succ}`\n'
    f'- Fail list: `{fail}`\n'
    f'- Failure reasons: `{reasons_txt}`\n\n'
    '## Top Failure Reasons\n\n' +
    '\n'.join(f'- `{row["count"]}` × `{row["reason"]}` (example: `{row["example"]}`)' for row in summary['top_failure_reasons']) +
    '\n'
)
print(js)
print(md)
PY

#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

SOURCE_REPO=${ETCD_SOURCE_REPO:-${SOURCE_REPO:-$ROOT/tmp/etcd}}
REF=${ETCD_REF:-}
ARTIFACT_DIR=${ARTIFACT_DIR:-$ROOT/artifacts/tinygo}

relativize() {
  local path=$1
  if [[ $path == "$ROOT/"* ]]; then
    printf '%s\n' "${path#"$ROOT"/}"
    return 0
  fi
  if [[ $path == "$HOME/"* ]]; then
    printf '~/%s\n' "${path#"$HOME"/}"
    return 0
  fi
  printf '%s\n' "$path"
}

mkdir -p "$ARTIFACT_DIR"

if [[ -z "$REF" ]]; then
  printf '%s\n' "$SOURCE_REPO"
  exit 0
fi

if [[ ! -d "$SOURCE_REPO/.git" ]]; then
  echo "prepare-etcd-probe-target: source repo is not a git checkout: $SOURCE_REPO" >&2
  exit 1
fi

COMMIT=$(git -C "$SOURCE_REPO" rev-parse "$REF^{commit}" 2>/dev/null) || {
  echo "prepare-etcd-probe-target: ref not found in source repo: $REF" >&2
  exit 1
}
SHORT_COMMIT=${COMMIT:0:12}
SAFE_REF=$(printf '%s' "$REF" | tr '/:@ ' '____' | tr -cd 'A-Za-z0-9._-')
if [[ -z "$SAFE_REF" ]]; then
  SAFE_REF="ref"
fi

TARGET_DIR=${ETCD_TARGET_DIR:-$ROOT/tmp/etcd-targets/$SAFE_REF-$SHORT_COMMIT}
STAMP_FILE="$TARGET_DIR/.mlse-target.json"

if [[ -d "$TARGET_DIR" && ! -f "$STAMP_FILE" ]]; then
  suffix=1
  while [[ -d "${TARGET_DIR}-$suffix" ]]; do
    suffix=$((suffix + 1))
  done
  TARGET_DIR="${TARGET_DIR}-$suffix"
  STAMP_FILE="$TARGET_DIR/.mlse-target.json"
fi

if [[ ! -f "$STAMP_FILE" ]]; then
  mkdir -p "$TARGET_DIR"
  git -C "$SOURCE_REPO" archive --format=tar "$COMMIT" | tar -x -C "$TARGET_DIR"
fi

python3 - <<'PY' "$ROOT" "$SOURCE_REPO" "$REF" "$COMMIT" "$TARGET_DIR" "$STAMP_FILE"
import json
import sys
from pathlib import Path

root = Path(sys.argv[1])
source_repo = Path(sys.argv[2])
ref = sys.argv[3]
commit = sys.argv[4]
target_dir = Path(sys.argv[5])
stamp_file = Path(sys.argv[6])

def rel(path: Path) -> str:
    try:
        return str(path.relative_to(root))
    except ValueError:
        home = Path.home()
        try:
            return "~/" + str(path.relative_to(home))
        except ValueError:
            return str(path)

go_version = None
toolchain = None
go_mod = target_dir / "go.mod"
if go_mod.exists():
    for line in go_mod.read_text().splitlines():
        stripped = line.strip()
        if stripped.startswith("go ") and go_version is None:
            go_version = stripped.split(None, 1)[1]
        if stripped.startswith("toolchain ") and toolchain is None:
            toolchain = stripped.split(None, 1)[1]

stamp = {
    "repo": "etcd",
    "requested_ref": ref,
    "resolved_commit": commit,
    "source_repo": rel(source_repo),
    "target_dir": rel(target_dir),
    "go": go_version,
    "toolchain": toolchain,
}
stamp_file.write_text(json.dumps(stamp, indent=2) + "\n")
PY

printf '%s\n' "$TARGET_DIR"

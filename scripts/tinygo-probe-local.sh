#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

TINYGO_BIN=${TINYGO_BIN:-$ROOT/tmp/tinygo/install/bin/tinygo}
REPO_DIR=${REPO_DIR:-$ROOT/tmp/etcd}
WORK_DIR=${WORK_DIR:-$ROOT/tmp/tinygo-work}
OUT_DIR=${OUT_DIR:-$ROOT/tmp/tinygo-out}
ARTIFACT_DIR=${ARTIFACT_DIR:-$ROOT/artifacts/tinygo}
TARGET_NAME=${TARGET_NAME:-etcd-local-probe}
JOBS=${JOBS:-4}
TINYGO_HOME=${TINYGO_HOME:-$ROOT/tmp/tinygo-home}
GO_BUILD_CACHE=${GO_BUILD_CACHE:-$ROOT/tmp/go-build}
GO_MOD_CACHE=${GO_MOD_CACHE:-$ROOT/tmp/gomodcache}
GO_PROXY=${GO_PROXY:-off}
GO_SUMDB=${GO_SUMDB:-off}
GO_FLAGS=${GO_FLAGS:--mod=readonly}

mkdir -p "$WORK_DIR" "$OUT_DIR" "$ARTIFACT_DIR"

if [[ -n "${ETCD_REF:-}" ]]; then
  REPO_DIR=$(
    ETCD_SOURCE_REPO="${ETCD_SOURCE_REPO:-$REPO_DIR}" \
    ETCD_REF="$ETCD_REF" \
    ARTIFACT_DIR="$ARTIFACT_DIR" \
    "$ROOT/scripts/prepare-etcd-probe-target.sh"
  )
fi

python3 "$ROOT/scripts/tinygo-probe-repo.py" \
  --mode files \
  --root "$ROOT" \
  --repo-dir "$REPO_DIR" \
  --tinygo-bin "$TINYGO_BIN" \
  --work-dir "$WORK_DIR" \
  --out-dir "$OUT_DIR" \
  --artifact-dir "$ARTIFACT_DIR" \
  --target-name "$TARGET_NAME" \
  --jobs "$JOBS" \
  --home-dir "$TINYGO_HOME" \
  --go-build-cache "$GO_BUILD_CACHE" \
  --go-mod-cache "$GO_MOD_CACHE" \
  --go-proxy "$GO_PROXY" \
  --go-sumdb "$GO_SUMDB" \
  --go-flags="$GO_FLAGS"

#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

rm -rf \
  "$ROOT/artifacts/bin" \
  "$ROOT/artifacts/tinygo"/* \
  "$ROOT/tmp/etcd" \
  "$ROOT/tmp/tinygo-work" \
  "$ROOT/tmp/tinygo-out"

mkdir -p "$ROOT/artifacts/tinygo" "$ROOT/tmp"

echo "cleaned MLSE transient artifacts"

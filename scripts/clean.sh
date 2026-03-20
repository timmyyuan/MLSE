#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

rm -rf "$ROOT/artifacts" "$ROOT/tmp"

mkdir -p "$ROOT/artifacts" "$ROOT/tmp"

echo "cleaned MLSE transient artifacts"

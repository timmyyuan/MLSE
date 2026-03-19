#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

mkdir -p "$ROOT/artifacts/bin"
go build -o "$ROOT/artifacts/bin/mlse-go" ./cmd/mlse-go

echo "built: $ROOT/artifacts/bin/mlse-go"

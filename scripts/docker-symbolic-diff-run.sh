#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

IMAGE=${IMAGE:-mlse-symbolic-diff:dev}

docker run --rm -it \
  -v "$ROOT:/workspace" \
  -w /workspace \
  "$IMAGE" "$@"

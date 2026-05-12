#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

IMAGE=${IMAGE:-mlse-symbolic-diff:dev}

docker build -f "$ROOT/docker/Dockerfile.symbolic-diff" -t "$IMAGE" "$ROOT"

echo "built: $IMAGE"

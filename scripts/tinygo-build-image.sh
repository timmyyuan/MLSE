#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

IMAGE_NAME=${IMAGE_NAME:-mlse-tinygo:dev}

docker build -f docker/Dockerfile.tinygo -t "$IMAGE_NAME" .

echo "built image: $IMAGE_NAME"

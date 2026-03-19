#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

find . -name '*.go' -not -path './tmp/*' -print0 | xargs -0 gofmt -w

#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)

"$ROOT/linters/run-go.sh"
"$ROOT/linters/run-cpp.sh"
"$ROOT/linters/run-python.sh"

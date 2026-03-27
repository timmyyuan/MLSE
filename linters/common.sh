#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)

# shellcheck source=/dev/null
source "$ROOT/linters/limits.env"

print_section() {
  printf '\n[%s]\n' "$1"
}

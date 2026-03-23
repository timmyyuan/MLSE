#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

# Intentionally keep the default test target on repository-owned Go code.
# The workspace may contain large third-party trees under tmp/ (for example
# TinyGo installs and probe targets), and `go test ./...` would incorrectly
# try to treat them as first-class MLSE packages.
go test ./cmd/... ./internal/...

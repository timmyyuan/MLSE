# TinyGo etcd v3.5.9 Post-Tidy Snapshot

This snapshot records the rerun after trying `go mod tidy` and dependency warm-up on the prepared older etcd target.

## Target And Cache Setup

- Baseline source target: `tmp/etcd-targets/helper-smoke-v359`.
- Post-tidy probe target copy: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320`.
- Workspace-local cache seed: `tmp/tidy-v359-20260320/gomodcache`, copied from `$(go env GOMODCACHE)` because online proxy fetches were unavailable in this sandbox.
- Warm-up outcomes are recorded in `module-warmup-summary.tsv`.

## Reproduction

Run from the repo root:

```bash
cp -R tmp/etcd-targets/helper-smoke-v359 tmp/etcd-targets/helper-smoke-v359-tidied-20260320
mkdir -p tmp/tidy-v359-20260320/gomodcache
cp -R "$(go env GOMODCACHE)"/. tmp/tidy-v359-20260320/gomodcache

# Warm the copied target using the same offline cache layout.
# The exact per-module outcomes are captured in testdata/tinygo-probes/etcd-v3.5.9-post-tidy/module-warmup-summary.tsv.

TINYGO_BIN=tmp/tinygo/install/bin/tinygo REPO_DIR=tmp/etcd-targets/helper-smoke-v359-tidied-20260320 WORK_DIR=tmp/tidy-v359-20260320/probe-work-pkg OUT_DIR=tmp/tidy-v359-20260320/probe-out-pkg ARTIFACT_DIR=artifacts/tinygo/etcd-v3.5.9-post-tidy/raw TARGET_NAME=package-probe TINYGO_HOME=tmp/tidy-v359-20260320/tinygo-home GO_BUILD_CACHE=tmp/tidy-v359-20260320/build GO_MOD_CACHE=tmp/tidy-v359-20260320/gomodcache GO_PROXY=off GO_SUMDB=off GO_FLAGS=-mod=readonly scripts/tinygo-probe-packages.sh

TINYGO_BIN=tmp/tinygo/install/bin/tinygo REPO_DIR=tmp/etcd-targets/helper-smoke-v359-tidied-20260320 WORK_DIR=tmp/tidy-v359-20260320/probe-work-file OUT_DIR=tmp/tidy-v359-20260320/probe-out-file ARTIFACT_DIR=artifacts/tinygo/etcd-v3.5.9-post-tidy/raw TARGET_NAME=file-probe TINYGO_HOME=tmp/tidy-v359-20260320/tinygo-home GO_BUILD_CACHE=tmp/tidy-v359-20260320/build GO_MOD_CACHE=tmp/tidy-v359-20260320/gomodcache GO_PROXY=off GO_SUMDB=off GO_FLAGS=-mod=readonly scripts/tinygo-probe-local.sh
```

## Result Summary

- Package probe: `27 / 145` packages succeeded (`18.62%`), up from `26 / 145`.
- File probe: `49 / 644` covered non-test files succeeded (`7.61%`), up from `47 / 644`.
- New successes were limited to `go.etcd.io/etcd/pkg/v3/cobrautl` and its two source files.
- Remaining failures are still dominated by offline dependency availability rather than new TinyGo semantic findings.

## Files

- `comparison-vs-previous.json` and `comparison-vs-previous.md`: previous v3.5.9 snapshot vs post-tidy rerun.
- `package-probe-*` and `file-probe-*`: post-tidy stable snapshot artifacts.
- `module-warmup-summary.tsv`: per-module `go mod tidy` / `go mod download all` / `go list -deps ./...` exit statuses.

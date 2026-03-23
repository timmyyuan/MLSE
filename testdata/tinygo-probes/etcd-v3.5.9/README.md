# TinyGo etcd Rerun Snapshot

This snapshot records the rerun against an older etcd release line after the `main`-branch probe hit TinyGo's Go 1.25 ceiling.

## Target Choice

- Desired target: `v3.5.25` from 2025. Upstream release planning indicates Go 1.24.x tooling there, which stays below TinyGo 0.40.1's `go1.25` maximum.
- Actual exercised target in this offline workspace: `v3.5.9` at commit `bdbbde998b7ed434b23676530d10dbd601c4a7c0`.
- Reason for the fallback: the only local full etcd clone available here contains tags through `v3.5.9`, so `v3.5.25` could be identified but not materialized.

## Reproduction

Run the same flow against any local etcd git clone:

```bash
ETCD_SOURCE_REPO=<local-etcd-git-clone> \
ETCD_REF=v3.5.9 \
TARGET_NAME=etcd-v3.5.9-package-probe \
scripts/tinygo-probe-packages.sh

ETCD_SOURCE_REPO=<local-etcd-git-clone> \
ETCD_REF=v3.5.9 \
TARGET_NAME=etcd-v3.5.9-file-probe \
scripts/tinygo-probe-local.sh
```

If a local clone with `v3.5.25` becomes available, switch only `ETCD_REF`.

## Result Summary

- Package probe: `26 / 145` packages succeeded (`17.93%`).
- File probe: `47 / 644` covered non-test files succeeded (`7.30%`).
- Omitted from file probe: `314` test files.
- Main-branch baseline: `0 / 13` packages succeeded because etcd `main` requires Go 1.26 while TinyGo 0.40.1 tops out at Go 1.25.

## Why Results Improved

- Moving off `main` removed the immediate Go 1.26 incompatibility wall.
- The package probe now walks every nested `go.mod`, not only the root module.
- Library packages are compiled through generated blank-import stubs, which matches TinyGo's build entry expectations better than direct library-package builds.
- The file probe now attributes results by package context instead of forcing arbitrary standalone `.go` files through `tinygo build`.

## Remaining Limits

- Most remaining failures are still environmental: `GOPROXY=off` makes missing modules fail as offline resolution errors.
- Some failures are genuine TinyGo/runtime gaps, such as `undefined: net.SRV` and `undefined: tls.X509KeyPair`.
- Some failures are still methodological distortions from the blank-import stub approach, for example `use of internal package ... not allowed`.
- The new file probe is more accurate than the old standalone-file probe, but it is not directly comparable 1:1 because the methodology changed on purpose.

## Files

- `rerun-comparison.json`: baseline vs rerun comparison, including desired vs actual target.
- `package-probe-summary.json`: package-level aggregate counts and top failure reasons.
- `file-probe-summary.json`: file-attribution aggregate counts and top failure reasons.
- `package-probe-results.jsonl` and `file-probe-results.jsonl`: per-package and per-file raw outcomes.
- `package-probe-success.txt`, `package-probe-fail.txt`, `file-probe-success.txt`, `file-probe-fail.txt`, `file-probe-omitted-test-files.txt`: stable lists copied from the probe run.

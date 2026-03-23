# TinyGo Package Probe Summary

- Repo dir: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320`
- TinyGo: `tinygo version 0.40.1 darwin/arm64 (using go version go1.25.8 and LLVM version 20.1.1)`
- Modules scanned: `12`
- Packages scanned: `145`
- Successful packages: `27`
- Failed packages: `118`
- Package success rate: `18.62%`
- Go files covered by successful packages: `49`
- Success list: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/package-probe-success.txt`
- Fail list: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/package-probe-fail.txt`
- Results jsonl: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/package-probe-results.jsonl`
- Failure reasons: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/package-probe-failure-reasons.txt`

## Warm-up Context

- Target copy: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320`
- Module cache seed: `tmp/tidy-v359-20260320/gomodcache` copied from `$(go env GOMODCACHE)`
- Module warm-up summary: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/module-warmup-summary.tsv`

## Top Failure Reasons

- `22` x `../../../tidy-v359-20260320/gomodcache/go.etcd.io/bbolt@v1.3.7/bolt_unix.go:12:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/etcdutl/v3`)
- `10` x `logger.go:24:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/client/v3`)
- `6` x `../../../tidy-v359-20260320/gomodcache/github.com/tmc/grpc-websocket-proxy@v0.0.0-20201229170055-e5319fda7802/wsproxy/websocket_proxy.go:12:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/server/v3`)
- `5` x `../../../tidy-v359-20260320/gomodcache/github.com/olekukonko/tablewriter@v0.0.5/util.go:15:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/etcdctl/v3`)
- `4` x `raftpb/raft.pb.go:13:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/raft/v3`)
- `4` x `mvcc/index.go:21:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/server/v3/etcdserver`)
- `3` x `../../../tidy-v359-20260320/gomodcache/github.com/grpc-ecosystem/grpc-gateway@v1.16.0/runtime/proto_errors.go:8:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/etcdserverpb/gw`)
- `3` x `../client/v3/logger.go:24:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/server/v3/etcdserver/api/v3election`)
- `2` x `fileutil/preallocate_darwin.go:24:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/client/pkg/v3/fileutil`)
- `2` x `etcdserver/api/v2store/node.go:24:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/server/v3/etcdserver/api/v2store`)
- `2` x `functional/runner/global.go:27:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/tests/v3/functional/cmd/etcd-runner`)
- `2` x `functional/tester/metrics_report.go:21:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/tests/v3/functional/cmd/etcd-tester`)
- `2` x `tools/benchmark/cmd/put.go:32:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/v3/tools/benchmark`)
- `1` x `authpb/auth.pb.go:13:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/authpb`)
- `1` x `etcdserverpb/rpc.pb.go:20:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/etcdserverpb`)
- `1` x `membershippb/membership.pb.go:13:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/membershippb`)
- `1` x `mvccpb/kv.pb.go:13:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/mvccpb`)
- `1` x `v3rpc/rpctypes/error.go:19:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/v3rpc/rpctypes`)
- `1` x `version/version.go:23:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/api/v3/version`)
- `1` x `logutil/zap_journal.go:30:2: module lookup disabled by GOPROXY=off` (example: `go.etcd.io/etcd/client/pkg/v3/logutil`)

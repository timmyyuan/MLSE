# TinyGo File Probe Summary

- Repo dir: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320`
- TinyGo: `tinygo version 0.40.1 darwin/arm64 (using go version go1.25.8 and LLVM version 20.1.1)`
- Modules scanned: `12`
- Package contexts compiled: `145`
- Covered non-test Go files: `644`
- Successful files: `49`
- Failed files: `595`
- Covered success rate: `7.61%`
- Omitted test Go files: `314`
- Success list: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/file-probe-success.txt`
- Fail list: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/file-probe-fail.txt`
- Omitted list: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/file-probe-omitted-test-files.txt`
- Results jsonl: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/file-probe-results.jsonl`
- Failure reasons: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/file-probe-failure-reasons.txt`

## Warm-up Context

- Target copy: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320`
- Module cache seed: `tmp/tidy-v359-20260320/gomodcache` copied from `$(go env GOMODCACHE)`
- Module warm-up summary: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy/module-warmup-summary.tsv`

## Top Failure Reasons

- `125` x `../../../tidy-v359-20260320/gomodcache/go.etcd.io/bbolt@v1.3.7/bolt_unix.go:12:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/etcdutl/ctl.go`)
- `55` x `../../../tidy-v359-20260320/gomodcache/github.com/olekukonko/tablewriter@v0.0.5/util.go:15:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/etcdctl/main.go`)
- `53` x `logger.go:24:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/client/v3/auth.go`)
- `32` x `mvcc/index.go:21:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/etcdserver/apply.go`)
- `30` x `functional/tester/metrics_report.go:21:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/tests/functional/cmd/etcd-tester/main.go`)
- `27` x `fileutil/preallocate_darwin.go:24:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/client/pkg/fileutil/dir_unix.go`)
- `27` x `../../../tidy-v359-20260320/gomodcache/github.com/tmc/grpc-websocket-proxy@v0.0.0-20201229170055-e5319fda7802/wsproxy/websocket_proxy.go:12:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/main.go`)
- `21` x `raftpb/raft.pb.go:13:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/raft/bootstrap.go`)
- `18` x `rafttest/interaction_env_handler.go:22:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/raft/rafttest/doc.go`)
- `16` x `etcdserver/api/rafthttp/stream.go:35:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/etcdserver/api/rafthttp/coder.go`)
- `15` x `tools/benchmark/cmd/put.go:32:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/tools/benchmark/doc.go`)
- `14` x `etcdserver/api/v2store/node.go:24:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/etcdserver/api/v2store/doc.go`)
- `12` x `json.go:22:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/client/v2/auth_role.go`)
- `10` x `proxy/grpcproxy/adapter/chan_stream.go:22:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/proxy/grpcproxy/adapter/auth_client_adapter.go`)
- `9` x `functional/runner/global.go:27:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/tests/functional/cmd/etcd-runner/main.go`)
- `8` x `flags/flag.go:25:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/pkg/flags/flag.go`)
- `8` x `wal/metrics.go:17:8: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/wal/decoder.go`)
- `7` x `testutil/leak.go:161:42: http.DefaultTransport.(*http.Transport).CloseIdleConnections undefined (type *http.Transport has no field or method CloseIdleConnections)` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/client/pkg/testutil/assert.go`)
- `6` x `../client/v3/logger.go:24:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/server/etcdserver/api/v3election/doc.go`)
- `6` x `server/etcdserver/api/rafthttp/stream.go:35:2: module lookup disabled by GOPROXY=off` (example: `tmp/etcd-targets/helper-smoke-v359-tidied-20260320/contrib/raftexample/doc.go`)

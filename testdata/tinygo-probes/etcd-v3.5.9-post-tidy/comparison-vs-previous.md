# TinyGo etcd v3.5.9 Post-Tidy Comparison

- Baseline snapshot: `testdata/tinygo-probes/etcd-v3.5.9`
- Post-tidy snapshot: `testdata/tinygo-probes/etcd-v3.5.9-post-tidy`
- Warm-up stayed offline: the local Go module cache was copied into `tmp/tidy-v359-20260320/gomodcache`, then all probes still ran with `GOPROXY=off`.
- Module warm-up: full `go mod tidy` success `0/12`, full `go mod download all` success `0/12`, full `go list -deps ./...` success `1/12` (`tools/mod`).

## Package Delta

- Successful packages: `26 -> 27` (`+1`)
- Package success rate: `17.93% -> 18.62%`
- Go files in successful packages: `47 -> 49`
- Newly unlocked package: `go.etcd.io/etcd/pkg/v3/cobrautl`

## File Delta

- Successful covered non-test files: `47 -> 49` (`+2`)
- Covered success rate: `7.30% -> 7.61%`
- Newly unlocked files: `pkg/cobrautl/error.go, pkg/cobrautl/help.go`

## Dependency vs Compatibility

- Dependency-availability failures moved only slightly: packages `114 -> 113`, files `582 -> 580`.
- No package or file moved from a dependency-availability failure into a new TinyGo semantic failure after warm-up.
- Non-dependency TinyGo/runtime failures stayed flat: packages `4`, files `14`.
- Representative unchanged TinyGo/runtime failures: `undefined: net.SRV`, `undefined: tls.X509KeyPair`, and missing `CloseIdleConnections` on `http.Transport`.
- The unchanged methodology artifact is the blank-import stub hitting `use of internal package ... not allowed` for `client/v3/naming/endpoints/internal`.

## Interpretation

- The offline warm-up improved only one package and two non-test files: go.etcd.io/etcd/pkg/v3/cobrautl plus pkg/cobrautl/error.go and pkg/cobrautl/help.go.
- Dependency-availability failures remained the dominant blocker after warm-up: packages 114 -> 113 and covered files 582 -> 580.
- Actual TinyGo/runtime incompatibility counts did not increase after warm-up: package-level tinygo_or_other stayed at 4 and file-level tinygo_or_other stayed at 14, so the tidy run did not reveal a new compatibility frontier.
- The main remaining wins are still gated on missing transitive modules inside the local cache seed, with top post-tidy blockers now appearing inside cached dependencies such as go.etcd.io/bbolt, github.com/olekukonko/tablewriter, and github.com/tmc/grpc-websocket-proxy.

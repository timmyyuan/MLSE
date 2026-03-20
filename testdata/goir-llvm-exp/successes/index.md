# Saved GoIR/LLVM IR Success Pairs

- Saved root: `testdata/goir-llvm-exp/successes`
- Successful pairs: `8`

## Index

- `simple_add`: `examples/go/simple_add.go` -> `testdata/goir-llvm-exp/successes/simple_add/goir.mlir` and `testdata/goir-llvm-exp/successes/simple_add/llvm.ll` (verifier `unavailable`, compile `success`)
- `sign_if`: `examples/go/sign_if.go` -> `testdata/goir-llvm-exp/successes/sign_if/goir.mlir` and `testdata/goir-llvm-exp/successes/sign_if/llvm.ll` (verifier `unavailable`, compile `success`)
- `choose_if_else`: `examples/go/choose_if_else.go` -> `testdata/goir-llvm-exp/successes/choose_if_else/goir.mlir` and `testdata/goir-llvm-exp/successes/choose_if_else/llvm.ll` (verifier `unavailable`, compile `success`)
- `choose_merge`: `examples/go/choose_merge.go` -> `testdata/goir-llvm-exp/successes/choose_merge/goir.mlir` and `testdata/goir-llvm-exp/successes/choose_merge/llvm.ll` (verifier `unavailable`, compile `success`)
- `sum_for`: `examples/go/sum_for.go` -> `testdata/goir-llvm-exp/successes/sum_for/goir.mlir` and `testdata/goir-llvm-exp/successes/sum_for/llvm.ll` (verifier `unavailable`, compile `success`)
- `switch_value`: `examples/go/switch_value.go` -> `testdata/goir-llvm-exp/successes/switch_value/goir.mlir` and `testdata/goir-llvm-exp/successes/switch_value/llvm.ll` (verifier `unavailable`, compile `success`)
- `etcd_mmap_size`: `tmp/etcd/server/storage/backend/config_windows.go` -> `testdata/goir-llvm-exp/successes/etcd_mmap_size/goir.mlir` and `testdata/goir-llvm-exp/successes/etcd_mmap_size/llvm.ll` (verifier `unavailable`, compile `success`)
- `etcd_preallocate_unsupported`: `tmp/etcd/client/pkg/fileutil/preallocate_unsupported.go` -> `testdata/goir-llvm-exp/successes/etcd_preallocate_unsupported/goir.mlir` and `testdata/goir-llvm-exp/successes/etcd_preallocate_unsupported/llvm.ll` (verifier `unavailable`, compile `success`)
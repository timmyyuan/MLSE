#!/usr/bin/env bash
set -euo pipefail

export LLVM_VERSION=${LLVM_VERSION:-20}
export MLSE_LLVM_VERSION=${MLSE_LLVM_VERSION:-${LLVM_VERSION}}
export KLEE_LLVM_VERSION=${KLEE_LLVM_VERSION:-16}
export LLVM_PREFIX=${LLVM_PREFIX:-/usr/lib/llvm-${MLSE_LLVM_VERSION}}
export KLEE_LLVM_PREFIX=${KLEE_LLVM_PREFIX:-/usr/lib/llvm-${KLEE_LLVM_VERSION}}
export MLIR_DIR=${MLIR_DIR:-${LLVM_PREFIX}/lib/cmake/mlir}
export LLVM_DIR=${LLVM_DIR:-${LLVM_PREFIX}/lib/cmake/llvm}
export CMAKE_C_COMPILER=${CMAKE_C_COMPILER:-${LLVM_PREFIX}/bin/clang}
export CMAKE_CXX_COMPILER=${CMAKE_CXX_COMPILER:-${LLVM_PREFIX}/bin/clang++}
export PATH="${KLEE_LLVM_PREFIX}/bin:/opt/klee-build/bin:${LLVM_PREFIX}/bin:${PATH}"

exec "$@"

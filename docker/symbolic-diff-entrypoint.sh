#!/usr/bin/env bash
set -euo pipefail

export LLVM_VERSION=${LLVM_VERSION:-16}
export LLVM_PREFIX=${LLVM_PREFIX:-/usr/lib/llvm-${LLVM_VERSION}}
export MLIR_DIR=${MLIR_DIR:-${LLVM_PREFIX}/lib/cmake/mlir}
export LLVM_DIR=${LLVM_DIR:-${LLVM_PREFIX}/lib/cmake/llvm}
export CMAKE_C_COMPILER=${CMAKE_C_COMPILER:-${LLVM_PREFIX}/bin/clang}
export CMAKE_CXX_COMPILER=${CMAKE_CXX_COMPILER:-${LLVM_PREFIX}/bin/clang++}
export PATH="${LLVM_PREFIX}/bin:${PATH}"

exec "$@"

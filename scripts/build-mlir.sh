#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "$0")/.." && pwd)
cd "$ROOT"

LLVM_PREFIX=${LLVM_PREFIX:-/opt/homebrew/Cellar/llvm@20/20.1.8}
MLIR_DIR=${MLIR_DIR:-$LLVM_PREFIX/lib/cmake/mlir}
LLVM_DIR=${LLVM_DIR:-$LLVM_PREFIX/lib/cmake/llvm}
CMAKE_C_COMPILER=${CMAKE_C_COMPILER:-$LLVM_PREFIX/bin/clang}
CMAKE_CXX_COMPILER=${CMAKE_CXX_COMPILER:-$LLVM_PREFIX/bin/clang++}
BUILD_DIR=${BUILD_DIR:-$ROOT/tmp/cmake-mlir-build}
SDKROOT=${SDKROOT:-$(xcrun --show-sdk-path)}

if [ ! -f "$MLIR_DIR/MLIRConfig.cmake" ]; then
  echo "error: MLIRConfig.cmake not found at: $MLIR_DIR" >&2
  echo "hint: set LLVM_PREFIX or MLIR_DIR to your Homebrew/LLVM install" >&2
  exit 1
fi

mkdir -p "$BUILD_DIR"
rm -f "$BUILD_DIR/CMakeCache.txt"

cmake -S "$ROOT" -B "$BUILD_DIR" \
  -DCMAKE_PREFIX_PATH="$LLVM_PREFIX" \
  -DMLIR_DIR="$MLIR_DIR" \
  -DLLVM_DIR="$LLVM_DIR" \
  -DCMAKE_C_COMPILER="$CMAKE_C_COMPILER" \
  -DCMAKE_CXX_COMPILER="$CMAKE_CXX_COMPILER" \
  -DCMAKE_CXX_FLAGS="-isysroot $SDKROOT"

cmake --build "$BUILD_DIR" --target mlse-opt -j4

echo "built: $BUILD_DIR/tools/mlse-opt/mlse-opt"
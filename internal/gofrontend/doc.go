// Package gofrontend implements the current Go source to formal MLIR bridge.
//
// Important: this layer is not yet a true "Go SSA -> GoIR" importer. The
// current pipeline is closer to:
//
//	Go source -> go/ast -> formal MLIR / go dialect bridge
//
// The lowering entry points are documented in docs/go-frontend-lowering.md,
// including a per-file function map and source-before / MLIR-after examples for
// the major formal_* paths.
package gofrontend

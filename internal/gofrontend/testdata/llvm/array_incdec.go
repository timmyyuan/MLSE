// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.bump
// LLVM: call ptr @g()
// LLVM: call i64 @runtime.index.
// LLVM: add i64
// LLVM: call ptr @runtime.store.index.
package demo

var g [2][3]uint64

func bump() uint64 {
	g[1][2]++
	return g[1][2]
}

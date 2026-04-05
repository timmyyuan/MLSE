// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.first
// LLVM: icmp ugt i64
// LLVM: getelementptr i64
// LLVM: call void @runtime.panic.index
// LLVM: unreachable
// LLVM: load i64
package demo

func first(xs []int) int {
	return xs[0]
}

// MLSE-COMPILE: formal
// LLVM-LABEL: define i32 @demo.first
// LLVM: icmp ult i64
// LLVM: getelementptr i32
// LLVM: call void @runtime.panic.index
// LLVM: unreachable
// LLVM: load i32
package demo

func first(xs []int) int {
	return xs[0]
}

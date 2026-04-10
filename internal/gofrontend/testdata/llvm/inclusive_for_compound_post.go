// LLVM-LABEL: define i64 @demo.inclusive(i64 %0)
// LLVM: phi i64
// LLVM: icmp slt i64
// LLVM: select i1
// LLVM-NOT: unreachable
package demo

func inclusive(n int) int {
	i := 0
	for i = 0; i <= n; i += 1 {
	}
	return i
}

// LLVM-LABEL: define i64 @demo.after(i64 %0)
// LLVM: br label
// LLVM: icmp sgt i64 %0, 1
// LLVM: select i1
// LLVM-NOT: unreachable
package demo

func after(n int) int {
	i := 1
	for i = 1; i < n; i++ {
	}
	return i
}

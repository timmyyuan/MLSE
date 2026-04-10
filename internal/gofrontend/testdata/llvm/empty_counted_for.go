// LLVM-LABEL: define i64 @demo.spin(i64 %0)
// LLVM: br label
// LLVM: phi i64
// LLVM-NOT: unreachable
package demo

func spin(n int) int {
	i := 0
	for i = 0; i < n; i++ {
	}
	return n
}

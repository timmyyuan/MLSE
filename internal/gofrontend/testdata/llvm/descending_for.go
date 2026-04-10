// LLVM-LABEL: define i64 @demo.down()
// LLVM: phi i64
// LLVM: icmp slt i64
// LLVM: sub i64 3,
// LLVM-NOT: unreachable
package demo

func down() int {
	i := 0
	for i = 3; i >= 0; i-- {
	}
	return i
}

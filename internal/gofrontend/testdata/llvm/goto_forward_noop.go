// LLVM-LABEL: define i64 @demo.pick
// LLVM-NOT: @demo.Next
// LLVM: add
package demo

func pick(x int) int {
	if x > 0 {
		goto Next
	}
Next:
	x = x + 1
	return x
}

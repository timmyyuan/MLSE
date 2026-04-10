// LLVM-LABEL: define i64 @demo.pick
// LLVM-NOT: @demo.Exit
// LLVM: br i1
package demo

func pick(x int) int {
	for x < 10 {
		if x > 0 {
			goto Exit
		}
		x++
	}
Exit:
	return x
}

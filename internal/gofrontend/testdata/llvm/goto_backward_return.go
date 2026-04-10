// LLVM-LABEL: define i64 @demo.pick
// LLVM-NOT: @demo.lbl
// LLVM-NOT: implicit_return_placeholder
// LLVM: br i1
// LLVM: ret i64
package demo

func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}

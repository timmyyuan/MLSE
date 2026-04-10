// LLVM-LABEL: define i64 @demo.pick(i64 %0, i1 %1)
// LLVM-NOT: @demo.lbl
// LLVM-NOT: implicit_return_placeholder
// LLVM: br i1 %1
// LLVM: phi i64
// LLVM: ret i64
package demo

func pick(x int, y bool) int {
	if y {
	lbl:
		x++
		if x < 4 {
			goto lbl
		}
	}
	return x
}

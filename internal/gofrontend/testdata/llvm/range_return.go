// LLVM-LABEL: define i32 @demo.pick
// LLVM: br label
// LLVM: ret i32
package demo

func pick(xs []int) int {
	for _, x := range xs {
		if x > 0 {
			return x
		}
	}
	return 0
}

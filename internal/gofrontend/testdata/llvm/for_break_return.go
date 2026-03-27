// MLSE-COMPILE: formal
// LLVM-LABEL: define i32 @demo.pick(
// LLVM: ret i32
package demo

func pick(xs []int, limit int) int {
	for i := 0; i < len(xs); i++ {
		if i >= limit {
			break
		}
		if xs[i] > 0 {
			return xs[i]
		}
	}
	return 0
}

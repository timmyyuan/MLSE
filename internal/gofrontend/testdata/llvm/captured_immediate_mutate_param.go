// LLVM-LABEL: define i64 @demo.capture_mutate_param(i64 %{{.*}})
// LLVM-NOT: @demo.capture_mutate_param.__lit
package demo

func capture_mutate_param(p int) int {
	func() int {
		p = p + 1
		return p
	}()
	return p
}

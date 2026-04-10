// LLVM-LABEL: define i64 @demo.capture_mutate_local()
// LLVM-NOT: @demo.capture_mutate_local.__lit
package demo

func capture_mutate_local() int {
	x := 1
	y := func() int {
		x = 3
		return x
	}()
	return x + y
}

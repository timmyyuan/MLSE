// LLVM-LABEL: define i64 @demo.capture_call()
// LLVM: call i64 @demo.capture_call.__lit0
// LLVM-NOT: call ptr
package demo

func capture_call() int {
	x := 2
	return func(z int) int {
		return x + z
	}(3)
}

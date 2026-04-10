// LLVM-LABEL: define i64 @demo.guaranteed_enter_returning_for_helper_post()
// LLVM-NOT: call i64 @demo.add1
// LLVM: ret i64 0
package demo

func add1(x int) int {
	return x + 1
}

func guaranteed_enter_returning_for_helper_post() int {
	for i := 0; i < 2; i = add1(i) {
		return i
	}
	return 9
}

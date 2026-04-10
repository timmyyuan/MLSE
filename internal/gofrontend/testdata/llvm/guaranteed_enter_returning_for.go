// LLVM-LABEL: define i64 @demo.guaranteed_enter_returning_for()
// LLVM-NOT: phi i64
// LLVM: ret i64 0
package demo

func guaranteed_enter_returning_for() int {
	for i := 0; i <= 3; i++ {
		return i
	}
	return 9
}

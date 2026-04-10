// CHECK-LABEL: func.func @demo.guaranteed_enter_returning_for_nested_break(%n: i64) -> i64
// CHECK: scf.for
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo "BranchStmt"
package demo

func guaranteed_enter_returning_for_nested_break(n int) int {
	for i := 1; i != 0; i++ {
		for j := 0; j <= 1; j++ {
			if n != 0 {
				break
			}
			return j
		}
		return 7
	}
	return 9
}

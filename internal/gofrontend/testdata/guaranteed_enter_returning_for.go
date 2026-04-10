// CHECK-LABEL: func.func @demo.guaranteed_enter_returning_for() -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: scf.for
// CHECK: return %[[C:.+]] : i64
package demo

func guaranteed_enter_returning_for() int {
	for i := 0; i <= 3; i++ {
		return i
	}
	return 9
}

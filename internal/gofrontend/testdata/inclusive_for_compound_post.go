// CHECK-LABEL: func.func @demo.inclusive(%n: i64) -> i64
// CHECK: %[[TRIP:.+]] = arith.addi
// CHECK: scf.for
// CHECK: %[[EXITCMP:.+]] = arith.cmpi sle,
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo_value "loop_iv_exit"
package demo

func inclusive(n int) int {
	i := 0
	for i = 0; i <= n; i += 1 {
	}
	return i
}

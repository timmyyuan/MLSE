// CHECK-LABEL: func.func @demo.spin(%n: i64) -> i64
// CHECK: scf.for
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo_value "loop_iv_exit"
package demo

func spin(n int) int {
	i := 0
	for i = 0; i < n; i++ {
	}
	return n
}

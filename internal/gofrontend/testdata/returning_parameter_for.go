// CHECK-LABEL: func.func @demo.early(%n: i64) -> i64
// CHECK: arith.constant 0 : i64
// CHECK: return %[[C:.+]] : i64
// CHECK-NOT: scf.for
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo_value "loop_iv_exit"
package demo

func early(n int) int {
	for n = 0; n <= 9; n += 1 {
		return n
	}
	return n
}

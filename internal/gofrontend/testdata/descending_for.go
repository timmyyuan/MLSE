// CHECK-LABEL: func.func @demo.down() -> i64
// CHECK: %[[TRIP:.+]] = arith.addi
// CHECK: scf.for
// CHECK: %[[IV:.+]] = arith.subi
// CHECK: %[[EXITCMP:.+]] = arith.cmpi sge,
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo_value "loop_iv_exit"
package demo

func down() int {
	i := 0
	for i = 3; i >= 0; i-- {
	}
	return i
}

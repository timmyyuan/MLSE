// CHECK-LABEL: func.func @demo.after(%n: i64) -> i64
// CHECK: scf.for
// CHECK: arith.cmpi slt
// CHECK: arith.select
// CHECK-NOT: go.todo_value "loop_iv_exit"
package demo

func after(n int) int {
	i := 1
	for i = 1; i < n; i++ {
	}
	return i
}

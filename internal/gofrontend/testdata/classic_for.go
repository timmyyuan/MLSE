// CHECK-LABEL: func.func @demo.walk(%xs: !go.slice<!go.string>) -> !go.error
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo_value "loop_iv_exit"
// CHECK: scf.for
package demo

func walk(xs []string) error {
	for i := 0; i < len(xs); i++ {
		_ = xs[i]
	}
	return nil
}

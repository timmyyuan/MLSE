// CHECK-LABEL: func.func @demo.ensureMap() -> !go.named<"map">
// CHECK-NOT: go.todo_value "make_missing_len"
// CHECK: func.call @__mlse_make__go.named__map__()
package demo

func ensureMap() map[string]bool {
	m := make(map[string]bool)
	return m
}

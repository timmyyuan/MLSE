// CHECK-LABEL: func.func @demo.ensureMap() -> !go.named<"map">
// CHECK-NOT: go.todo_value "make_missing_len"
// CHECK: func.call @runtime.make.map()
package demo

func ensureMap() map[string]bool {
	m := make(map[string]bool)
	return m
}

// CHECK-LABEL: func.func @demo.build() -> !go.slice<i64>
// CHECK: go.make_slice
// CHECK-NOT: go.todo_value "CompositeLit"
package demo

func build() []int {
	return []int{}
}

// CHECK-LABEL: func.func @demo.build() -> !go.slice<i32>
// CHECK: go.make_slice
// CHECK-NOT: go.todo_value "CompositeLit"
package demo

func build() []int {
	return []int{}
}

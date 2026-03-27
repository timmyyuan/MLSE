// CHECK-LABEL: func.func @demo.build(%xs: !go.slice<i32>) -> !go.slice<i32>
// CHECK: go.len %xs : !go.slice<i32> -> i32
// CHECK-NOT: go.todo_value "type_conversion" : !go.named<"len">
package demo

func build(xs []int) []int {
	return make([]int, 0, len(xs))
}

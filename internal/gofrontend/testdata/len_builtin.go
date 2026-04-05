// CHECK-LABEL: func.func @demo.build(%xs: !go.slice<i64>) -> !go.slice<i64>
// CHECK: go.len %xs : !go.slice<i64> -> i64
// CHECK-NOT: go.todo_value "type_conversion" : !go.named<"len">
package demo

func build(xs []int) []int {
	return make([]int, 0, len(xs))
}

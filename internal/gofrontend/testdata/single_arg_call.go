// CHECK-LABEL: func.func @demo.apply(%x: i64) -> i64
// CHECK: func.call @demo.inc(%x) : (i64) -> i64
// CHECK-NOT: go.todo_value "type_conversion"
package demo

func inc(x int) int {
	return x + 1
}

func apply(x int) int {
	return inc(x)
}

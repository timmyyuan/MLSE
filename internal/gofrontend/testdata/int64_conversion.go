// CHECK-LABEL: func.func @demo.total() -> i64
// CHECK-NOT: go.todo_value "type_conversion"
// CHECK: arith.constant 0 : i64
package demo

func total() int64 {
	return int64(0)
}

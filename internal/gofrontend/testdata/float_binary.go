// CHECK-LABEL: func.func @demo.format(%n: i64) -> !go.string
// CHECK-NOT: go.todo_value "binary__"
// CHECK: arith.sitofp %n : i64 to f64
// CHECK: arith.sitofp %{{[A-Za-z0-9_%.]+}} : i64 to f64
// CHECK: arith.divf
// CHECK: func.call @runtime.any.box.f64(
// CHECK: func.call @runtime.fmt.Sprintf(
package demo

import "fmt"

func format(n int64) string {
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}

// CHECK-LABEL: func.func @demo.format(%n: i64) -> !go.string
// CHECK-NOT: go.todo_value "binary__"
// CHECK: func.call @__mlse_bin_____
package demo

import "fmt"

func format(n int64) string {
	return fmt.Sprintf("%.1fK", float64(n)/1000)
}

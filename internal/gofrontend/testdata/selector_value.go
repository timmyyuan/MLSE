// CHECK-LABEL: func.func @demo.add(%x: i64) -> i64
// CHECK: func.call @example.com.common.GlobalInput() : () -> i64
// CHECK: func.func private @example.com.common.GlobalInput() -> i64
// CHECK-NOT: go.todo_value "SelectorExpr" : i64
package demo

import commonpkg "example.com/common"

func add(x int) int {
	return x + commonpkg.GlobalInput
}

// CHECK-LABEL: func.func @demo.add(%x: i32) -> i32
// CHECK: func.call @example.com.common.GlobalInput() : () -> i32
// CHECK: func.func private @example.com.common.GlobalInput() -> i32
// CHECK-NOT: go.todo_value "SelectorExpr" : i32
package demo

import commonpkg "example.com/common"

func add(x int) int {
	return x + commonpkg.GlobalInput
}

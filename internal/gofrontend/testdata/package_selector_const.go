// CHECK-LABEL: func.func @demo.max() -> i64
// CHECK: arith.constant 2147483647 : i64
// CHECK-NOT: func.call @math.MaxInt32
// CHECK-NOT: func.func private @math.MaxInt32
package demo

import "math"

func max() int {
	return math.MaxInt32
}

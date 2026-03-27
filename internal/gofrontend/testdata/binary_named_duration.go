// CHECK-LABEL: func.func @demo.span() -> !go.named<"time.Duration">
// CHECK: func.call @time.Hour() : () -> !go.named<"time.Duration">
// CHECK: func.call @__mlse_convert__go.named__time.Duration____to__i32
// CHECK: arith.muli
// CHECK: func.call @__mlse_convert_i32__to___go.named__time.Duration__
package demo

import "time"

func span() time.Duration {
	return 24 * time.Hour
}

// CHECK-LABEL: func.func @demo.span() -> !go.named<"time.Duration">
// CHECK: func.call @time.Hour() : () -> !go.named<"time.Duration">
// CHECK: func.call @runtime.convert.time.Duration.to.i64
// CHECK: arith.muli
// CHECK: func.call @runtime.convert.i64.to.time.Duration
package demo

import "time"

func span() time.Duration {
	return 24 * time.Hour
}

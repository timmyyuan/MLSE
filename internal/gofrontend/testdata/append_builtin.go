// CHECK-LABEL: func.func @demo.grow(%xs: !go.slice<i64>) -> !go.slice<i64>
// CHECK: %[[VALUE:[A-Za-z0-9_%.]+]] = arith.constant 1 : i64
// CHECK: %[[APP:[A-Za-z0-9_%.]+]] = go.append %xs, %[[VALUE]] : (!go.slice<i64>, i64) -> !go.slice<i64>
package demo

func grow(xs []int) []int {
	return append(xs, 1)
}

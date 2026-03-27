// CHECK-LABEL: func.func @demo.grow(%xs: !go.slice<i32>) -> !go.slice<i32>
// CHECK: %[[VALUE:[A-Za-z0-9_%.]+]] = arith.constant 1 : i32
// CHECK: %[[APP:[A-Za-z0-9_%.]+]] = go.append %xs, %[[VALUE]] : (!go.slice<i32>, i32) -> !go.slice<i32>
package demo

func grow(xs []int) []int {
	return append(xs, 1)
}

// CHECK-LABEL: func.func @demo.first(%xs: !go.slice<i32>) -> i32
// CHECK: %[[IDX:[A-Za-z0-9_%.]+]] = arith.constant 0 : i32
// CHECK: %[[ADDR:[A-Za-z0-9_%.]+]] = go.elem_addr %xs, %[[IDX]] : (!go.slice<i32>, i32) -> !go.ptr<i32>
// CHECK: %[[VAL:[A-Za-z0-9_%.]+]] = go.load %[[ADDR]] : !go.ptr<i32> -> i32
package demo

func first(xs []int) int {
	return xs[0]
}

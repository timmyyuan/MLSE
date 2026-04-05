// CHECK-LABEL: func.func @demo.first(%xs: !go.slice<i64>) -> i64
// CHECK: %[[IDX:[A-Za-z0-9_%.]+]] = arith.constant 0 : i64
// CHECK: %[[ADDR:[A-Za-z0-9_%.]+]] = go.elem_addr %xs, %[[IDX]] : (!go.slice<i64>, i64) -> !go.ptr<i64>
// CHECK: %[[VAL:[A-Za-z0-9_%.]+]] = go.load %[[ADDR]] : !go.ptr<i64> -> i64
package demo

func first(xs []int) int {
	return xs[0]
}

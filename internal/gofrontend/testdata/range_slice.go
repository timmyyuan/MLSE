// CHECK-LABEL: func.func @demo.sum(%xs: !go.slice<i32>) -> i32
// CHECK: %[[LEN:[A-Za-z0-9_%.]+]] = go.len %xs : !go.slice<i32> -> i32
// CHECK: scf.for
// CHECK: %[[ADDR:[A-Za-z0-9_%.]+]] = go.elem_addr %xs, %{{[A-Za-z0-9_%.]+}} : (!go.slice<i32>, index) -> !go.ptr<i32>
// CHECK: %[[VAL:[A-Za-z0-9_%.]+]] = go.load %[[ADDR]] : !go.ptr<i32> -> i32
package demo

func sum(xs []int) int {
	total := 0
	for _, v := range xs {
		total = total + v
	}
	return total
}

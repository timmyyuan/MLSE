// CHECK-LABEL: func.func @demo.measure(%xs: !go.slice<i32>) -> i32
// CHECK: go.cap %xs : !go.slice<i32> -> i32
package demo

func measure(xs []int) int {
	return cap(xs)
}

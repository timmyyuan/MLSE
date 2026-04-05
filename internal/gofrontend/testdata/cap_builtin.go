// CHECK-LABEL: func.func @demo.measure(%xs: !go.slice<i64>) -> i64
// CHECK: go.cap %xs : !go.slice<i64> -> i64
package demo

func measure(xs []int) int {
	return cap(xs)
}

// CHECK-LABEL: func.func @demo.build(%n: i64) -> !go.slice<i64>
// CHECK: go.make_slice
// CHECK-LABEL: func.func @demo.fail() -> !go.error
// CHECK: go.nil : !go.error
package demo

func build(n int) []int {
	return make([]int, n)
}

func fail() error {
	return nil
}

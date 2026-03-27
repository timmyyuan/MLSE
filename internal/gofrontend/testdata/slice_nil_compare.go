// CHECK-LABEL: func.func @demo.isNil(%xs: !go.slice<i32>) -> i1
// CHECK: %nil{{[0-9]+}} = go.nil : !go.slice<i32>
// CHECK: %cmp{{[0-9]+}} = go.eq %xs, %nil{{[0-9]+}} : (!go.slice<i32>, !go.slice<i32>) -> i1
// CHECK-LABEL: func.func @demo.isNotNil(%xs: !go.slice<i32>) -> i1
// CHECK: %nil{{[0-9]+}} = go.nil : !go.slice<i32>
// CHECK: %cmp{{[0-9]+}} = go.neq %xs, %nil{{[0-9]+}} : (!go.slice<i32>, !go.slice<i32>) -> i1
package demo

func isNil(xs []int) bool {
	return xs == nil
}

func isNotNil(xs []int) bool {
	return xs != nil
}

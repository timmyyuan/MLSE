// CHECK-LABEL: func.func @demo.bump() -> i64
// CHECK-NOT: go.todo "incdec_non_integer"
// CHECK: func.call @runtime.index.
// CHECK: arith.addi
// CHECK: func.call @runtime.store.index.
package demo

var g [2][3]uint64

func bump() uint64 {
	g[1][2]++
	return g[1][2]
}

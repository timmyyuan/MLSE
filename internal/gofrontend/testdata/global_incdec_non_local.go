// CHECK-LABEL: func.func @demo.f() -> i64
// CHECK: %global{{[0-9]+}} = func.call @g() : () -> i64
// CHECK-NOT: go.todo "incdec_non_local"
// CHECK: %const{{[0-9]+}} = arith.constant 1 : i64
// CHECK: %inc{{[0-9]+}} = arith.subi %global{{[0-9]+}}, %const{{[0-9]+}} : i64
// CHECK: return %inc{{[0-9]+}} : i64
package demo

var g uint64

func f() uint64 {
	g--
	return g
}

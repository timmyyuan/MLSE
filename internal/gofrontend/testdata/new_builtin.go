// CHECK-LABEL: func.func @demo.alloc() -> !go.ptr<i64>
// CHECK: %{{[A-Za-z0-9_%.]+}} = arith.constant 8 : i64
// CHECK: %{{[A-Za-z0-9_%.]+}} = arith.constant 8 : i64
// CHECK: %{{[A-Za-z0-9_%.]+}} = func.call @runtime.newobject(%{{[A-Za-z0-9_%.]+}}, %{{[A-Za-z0-9_%.]+}}) : (i64, i64) -> !go.ptr<i64>
package demo

func alloc() *int {
	return new(int)
}

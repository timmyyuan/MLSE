// CHECK-LABEL: func.func @demo.bump(%p: !go.ptr<i64>) -> i64
// CHECK-NOT: go.todo "IncDecStmt"
// CHECK: %load{{[0-9]+}} = go.load %p : !go.ptr<i64> -> i64
// CHECK: %const{{[0-9]+}} = arith.constant 1 : i64
// CHECK: %inc{{[0-9]+}} = arith.addi %load{{[0-9]+}}, %const{{[0-9]+}} : i64
// CHECK: go.store %inc{{[0-9]+}}, %p : i64 to !go.ptr<i64>
// CHECK-LABEL: func.func @demo.drop(%p: !go.ptr<i64>) -> i64
// CHECK-NOT: go.todo "IncDecStmt"
// CHECK: %load{{[0-9]+}} = go.load %p : !go.ptr<i64> -> i64
// CHECK: %const{{[0-9]+}} = arith.constant 1 : i64
// CHECK: %inc{{[0-9]+}} = arith.subi %load{{[0-9]+}}, %const{{[0-9]+}} : i64
// CHECK: go.store %inc{{[0-9]+}}, %p : i64 to !go.ptr<i64>
package demo

func bump(p *int) int {
	(*p)++
	return *p
}

func drop(p *int) int {
	(*p)--
	return *p
}

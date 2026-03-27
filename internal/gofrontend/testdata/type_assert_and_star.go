// CHECK-LABEL: func.func @demo.use(%x: !go.named<"any">, %p: !go.ptr<i32>) -> i1
// CHECK-NOT: go.todo_value "TypeAssertExpr"
// CHECK-NOT: go.todo_value "StarExpr"
// CHECK: func.call @__mlse_type_assert
// CHECK: go.load %p : !go.ptr<i32> -> i32
// CHECK-NOT: __mlse_deref
package demo

func use(x any, p *int) bool {
	v, _ := x.(bool)
	if p != nil {
		_ = *p
	}
	return v
}

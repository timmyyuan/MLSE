// CHECK-LABEL: func.func @demo.use(%x: !go.named<"any">, %p: !go.ptr<i64>) -> i1
// CHECK-NOT: go.todo_value "TypeAssertExpr"
// CHECK-NOT: go.todo_value "StarExpr"
// CHECK: func.call @runtime.type.assert.any.to.bool
// CHECK: go.load %p : !go.ptr<i64> -> i64
// CHECK-NOT: func.call @runtime.deref.
package demo

func use(x any, p *int) bool {
	v, _ := x.(bool)
	if p != nil {
		_ = *p
	}
	return v
}

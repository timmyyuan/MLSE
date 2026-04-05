// CHECK-LABEL: func.func @demo.branch(%x: !go.named<"any">) -> i64
// CHECK-NOT: go.todo "IfStmt_condition"
// CHECK: func.call @runtime.convert.any.to.bool
// CHECK: scf.if
package demo

func branch(x any) int {
	if x {
		return 1
	}
	return 0
}

// CHECK-LABEL: func.func @demo.branch(%x: !go.named<"any">) -> i32
// CHECK-NOT: go.todo "IfStmt_condition"
// CHECK: func.call @__mlse_convert__go.named__any____to__i1
// CHECK-NOT: __mlse_stmt_if_returning_region
// CHECK: scf.if
package demo

func branch(x any) int {
	if x {
		return 1
	}
	return 0
}

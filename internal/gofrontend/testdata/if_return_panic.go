// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
// CHECK-NOT: go.todo "implicit_return_placeholder"
package demo

func pick(x int) int {
	if x > 0 {
		return x
	} else {
	}
	panic("boom")
}

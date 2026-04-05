// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK: scf.if
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func pick(x int) int {
	if x > 0 {
		return 1
	}
	if x > 1 {
		return 2
	}
	return 3
}

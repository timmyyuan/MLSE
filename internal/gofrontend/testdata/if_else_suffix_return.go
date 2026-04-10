// CHECK-LABEL: func.func @demo.choose(%flag: i1, %x: i64) -> i64
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func choose(flag bool, x int) int {
	if flag {
		return 1
	} else {
		x = x + 2
	}
	return x
}

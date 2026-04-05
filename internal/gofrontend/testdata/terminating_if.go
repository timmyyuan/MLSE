// CHECK-LABEL: func.func @demo.sign(%x: i64) -> i64
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt"
package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}

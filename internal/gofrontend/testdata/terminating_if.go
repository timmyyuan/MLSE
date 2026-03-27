// CHECK-LABEL: func.func @demo.sign(%x: i32) -> i32
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt"
package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}

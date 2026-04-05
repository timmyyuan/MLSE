// CHECK-LABEL: func.func @demo.choose(%b: i1) -> i64
// CHECK: scf.if
// CHECK: scf.yield
// CHECK-NOT: go.todo "IfStmt"
package demo

func choose(b bool) int {
	var x int
	if b {
		x = 1
	} else {
		x = 2
	}
	return x
}

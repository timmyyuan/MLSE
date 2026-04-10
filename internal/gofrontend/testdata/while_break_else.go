// CHECK-LABEL: func.func @demo.countWithElseBreak(%n: i64) -> i64
// CHECK: scf.while
// CHECK: scf.if
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo "BranchStmt"
package demo

func countWithElseBreak(n int) int {
	i := 0
	for i < n {
		if n == 0 {
			i++
		} else {
			if i >= 3 {
				break
			}
		}
		i++
	}
	return i
}

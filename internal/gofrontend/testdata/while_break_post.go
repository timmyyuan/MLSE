// CHECK-LABEL: func.func @demo.countWithPost(%n: i64) -> i64
// CHECK: scf.while
// CHECK: scf.if
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: go.todo "BranchStmt"
package demo

func countWithPost(n int) int {
	i := 0
	for i = 0; i <= n; i++ {
		if i >= 3 {
			break
		}
	}
	return i
}

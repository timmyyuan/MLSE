// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK-NOT: go.todo "BranchStmt"
// CHECK-NOT: go.todo "LabeledStmt"
// CHECK: scf.while
package demo

func pick(x int) int {
	for x < 10 {
		if x > 0 {
			goto Exit
		}
		x++
	}
Exit:
	return x
}

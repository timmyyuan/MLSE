// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK-NOT: go.todo "BranchStmt"
// CHECK-NOT: go.todo "LabeledStmt"
package demo

func pick(x int) int {
	if x > 0 {
		goto Next
	}
Next:
	x = x + 1
	return x
}

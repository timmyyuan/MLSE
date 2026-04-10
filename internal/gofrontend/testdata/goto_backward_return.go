// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK-NOT: go.todo "BranchStmt"
// CHECK-NOT: go.todo "LabeledStmt"
// CHECK-NOT: go.todo "implicit_return_placeholder"
// CHECK: scf.if
// CHECK: return
package demo

func pick(x int) int {
lbl:
	x++
	if x < 4 {
		goto lbl
	}
	return x
}

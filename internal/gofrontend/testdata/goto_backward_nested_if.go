// CHECK-LABEL: func.func @demo.pick(%x: i64, %y: i1) -> i64
// CHECK-NOT: go.todo "BranchStmt"
// CHECK-NOT: go.todo "LabeledStmt"
// CHECK-NOT: go.todo "implicit_return_placeholder"
// CHECK: scf.if %y -> (i64)
// CHECK: scf.while
// CHECK: return
package demo

func pick(x int, y bool) int {
	if y {
	lbl:
		x++
		if x < 4 {
			goto lbl
		}
	}
	return x
}

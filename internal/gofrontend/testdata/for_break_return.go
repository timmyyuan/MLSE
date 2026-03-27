// CHECK-LABEL: func.func @demo.pick(%xs: !go.slice<i32>, %limit: i32) -> i32
// CHECK: scf.for
// CHECK: scf.if
// CHECK-NOT: go.todo "BranchStmt"
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func pick(xs []int, limit int) int {
	for i := 0; i < len(xs); i++ {
		if i >= limit {
			break
		}
		if xs[i] > 0 {
			return xs[i]
		}
	}
	return 0
}

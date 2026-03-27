// CHECK-LABEL: func.func @demo.pick(%xs: !go.slice<i32>) -> i32
// CHECK: scf.for
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func pick(xs []int) int {
	for _, x := range xs {
		if x > 0 {
			return x
		}
	}
	return 0
}

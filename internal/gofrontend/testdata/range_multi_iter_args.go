// CHECK-LABEL: func.func @demo.accumulate(%xs: !go.slice<i64>) -> (i64, i64)
// CHECK: scf.for
// CHECK: iter_args(
// CHECK: -> (i64, i64)
// CHECK: scf.if
// CHECK-NOT: go.todo "RangeStmt_multi_iter_args"
// CHECK-NOT: go.todo "BranchStmt"
package demo

func accumulate(xs []int) (int, int) {
	var sum int
	var count int
	for _, x := range xs {
		if x == 0 {
			continue
		}
		sum = sum + x
		count = count + 1
	}
	return sum, count
}

// CHECK-LABEL: func.func @demo.mark(%xs: !go.slice<i32>) -> !go.slice<i32>
// CHECK: scf.for
// CHECK: scf.if
// CHECK-NOT: go.todo "BranchStmt"
package demo

func mark(xs []int) []int {
	var out []int
	for _, x := range xs {
		if x == 0 {
			out = append(out, 99)
			continue
		}
		out = append(out, x)
	}
	return out
}

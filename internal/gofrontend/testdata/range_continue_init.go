// CHECK-LABEL: func.func @demo.collect(%xs: !go.slice<i32>) -> !go.slice<i32>
// CHECK: scf.for
// CHECK: scf.if
// CHECK-NOT: go.todo "BranchStmt"
package demo

type skipset struct{}

func (s *skipset) Has(v int) bool { return v == 0 }

func collect(xs []int) []int {
	var out []int
	ss := &skipset{}
	for _, x := range xs {
		if hit := ss.Has(x); hit {
			continue
		}
		out = append(out, x)
	}
	return out
}

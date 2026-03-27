// CHECK-LABEL: func.func @demo.pick(%ok: i1) -> (!go.string, !go.error)
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func pick(ok bool) (string, error) {
	if !ok {
		return "", nil
	}
	return "x", nil
}

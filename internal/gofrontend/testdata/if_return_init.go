// CHECK-LABEL: func.func @demo.parse(%s: !go.string) -> (i64, i1)
// CHECK-NOT: go.todo "IfStmt_returning_region"
// CHECK: %ifret
package demo

func isBad(v string) bool {
	return v != ""
}

func parse(s string) (int, bool) {
	if v := s; isBad(v) {
		return 0, false
	}
	return 1, true
}

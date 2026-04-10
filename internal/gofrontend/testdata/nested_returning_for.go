// CHECK-LABEL: func.func @demo.nested_returning_for(%flag: i1) -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK: scf.for
// CHECK: scf.if
package demo

func nested_returning_for(flag bool) int {
	var i int
	var j int
	for i = 0; i < 2; i++ {
		if flag {
			continue
		}
		for j = 0; j < 3; j++ {
			return j + 1
		}
	}
	return 0
}

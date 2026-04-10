// CHECK-LABEL: func.func @demo.returning_carried_for(%flag: i1) -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK: scf.for
// CHECK: iter_args(
// CHECK: return %[[RET:.+]] : i64
package demo

func returning_carried_for(flag bool) int {
	sum := 0
	var i int
	var j int
	for i = 0; i <= 3; i += 1 {
		sum += i
		if flag {
			continue
		}
		for j = 0; j <= 2; j += 1 {
			return sum + j
		}
	}
	return sum
}

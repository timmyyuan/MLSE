// CHECK-LABEL: func.func @demo.pick(%x: i64) -> i64
// CHECK: arith.constant 1 : i64
// CHECK: scf.if
// CHECK-NOT: go.todo "IfStmt_returning_region"
package demo

func pick(x int) int {
	y := x + 1
	if y > 3 {
		return y
	}
	return x
}

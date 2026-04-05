// CHECK-LABEL: func.func @demo.sumTo(%n: i64) -> i64
// CHECK: scf.for
// CHECK: iter_args(
// CHECK-NOT: go.todo "ForStmt"
package demo

func sumTo(n int) int {
	sum := 0
	i := 0
	for i < n {
		sum = sum + i
		i = i + 1
	}
	return sum
}

// CHECK-LABEL: func.func @demo.first() -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK: return
package demo

var xs [3]int

func first() int {
	for xs[1] = 3; xs[1] >= 1; xs[1] -= 1 {
		return xs[1]
	}
	return 0
}

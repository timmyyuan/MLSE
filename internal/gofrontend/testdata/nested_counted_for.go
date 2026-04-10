// CHECK-LABEL: func.func @demo.fill() -> i64
// CHECK-NOT: go.todo_value "loop_iv_exit"
// CHECK: scf.for
// CHECK: scf.for
package demo

func fill() int {
	var xs [4][2]int
	var i int
	var j int
	for i = 0; i < 4; i++ {
		for j = 0; j < 2; j++ {
			xs[i][j] = i + j
		}
	}
	return xs[0][0]
}

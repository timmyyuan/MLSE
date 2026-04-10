// CHECK-LABEL: func.func @demo.f() -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: @runtime.index.value(%store
package demo

var g [8][2]int

func f() int {
	for g[5][0] = -16; g[5][0] >= -9; g[5][0] = g[5][0] + 8 {
		return g[0][0]
	}
	return 0
}

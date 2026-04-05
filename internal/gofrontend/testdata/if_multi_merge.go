// CHECK-LABEL: func.func @demo.touch(%flag: i1, %a: i64, %b: i64) -> (i64, i64)
// CHECK-NOT: go.todo "IfStmt_multi_merge"
// CHECK: = scf.if %flag -> (i64, i64)
package demo

func touch(flag bool, a int, b int) (int, int) {
	if flag {
		a = a + 1
		b = b + 2
	}
	return a, b
}

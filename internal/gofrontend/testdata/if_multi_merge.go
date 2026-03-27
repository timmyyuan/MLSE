// CHECK-LABEL: func.func @demo.touch(%flag: i1, %a: i32, %b: i32) -> (i32, i32)
// CHECK-NOT: go.todo "IfStmt_multi_merge"
// CHECK: = scf.if %flag -> (i32, i32)
package demo

func touch(flag bool, a int, b int) (int, int) {
	if flag {
		a = a + 1
		b = b + 2
	}
	return a, b
}

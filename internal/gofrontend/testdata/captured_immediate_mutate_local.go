// CHECK-LABEL: func.func @demo.capture_mutate_local() -> i64
// CHECK: %call{{[0-9]+}}:2 = scf.execute_region -> (i64, i64)
// CHECK-NOT: FuncLit_capture
package demo

func capture_mutate_local() int {
	x := 1
	y := func() int {
		x = 3
		return x
	}()
	return x + y
}

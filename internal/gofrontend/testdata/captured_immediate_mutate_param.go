// CHECK-LABEL: func.func @demo.capture_mutate_param(%p: i64) -> i64
// CHECK: %call{{[0-9]+}}:2 = scf.execute_region -> (i64, i64)
// CHECK-NOT: FuncLit_capture
package demo

func capture_mutate_param(p int) int {
	func() int {
		p = p + 1
		return p
	}()
	return p
}

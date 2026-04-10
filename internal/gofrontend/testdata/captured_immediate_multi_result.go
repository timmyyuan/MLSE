// CHECK-LABEL: func.func @demo.capture_pair() -> (i64, i64)
// CHECK: %call{{[0-9]+}}:2 = func.call @demo.capture_pair.__lit0(%const{{[0-9]+}}, %const{{[0-9]+}}) : (i64, i64) -> (i64, i64)
// CHECK: func.func private @demo.capture_pair.__lit0(%x: i64, %y: i64) -> (i64, i64)
// CHECK-NOT: FuncLit_capture
package demo

func capture_pair() (int, int) {
	x := 2
	y := 3
	a, b := func() (int, int) {
		return x, y
	}()
	return a, b
}

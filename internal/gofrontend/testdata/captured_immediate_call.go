// CHECK-LABEL: func.func @demo.capture_call() -> i64
// CHECK: %call{{[0-9]+}}:1 = func.call @demo.capture_call.__lit0(%const{{[0-9]+}}, %const{{[0-9]+}}) : (i64, i64) -> (i64)
// CHECK: func.func private @demo.capture_call.__lit0(%x: i64, %z: i64) -> i64
// CHECK-NOT: FuncLit_capture
package demo

func capture_call() int {
	x := 2
	return func(z int) int {
		return x + z
	}(3)
}

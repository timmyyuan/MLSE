// CHECK-LABEL: func.func @demo.capture_global_only() -> !go.ptr<i64>
// CHECK-NOT: FuncLit_capture
// CHECK: %funclit{{[0-9]+}} = func.constant @demo.capture_global_only.__lit0 : () -> !go.ptr<i64>
// CHECK: %call{{[0-9]+}} = func.call_indirect %funclit{{[0-9]+}}() : () -> !go.ptr<i64>
package demo

var capture_global_only_g int
var capture_global_only_p = &capture_global_only_g

func capture_global_only() *int {
	return func() *int {
		capture_global_only_g = 7
		return capture_global_only_p
	}()
}

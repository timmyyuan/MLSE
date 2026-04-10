// LLVM-LABEL: define ptr @demo.capture_global_only()
// LLVM-NOT: FuncLit_capture
// LLVM: call ptr @capture_global_only_p
package demo

var capture_global_only_g int
var capture_global_only_p = &capture_global_only_g

func capture_global_only() *int {
	return func() *int {
		capture_global_only_g = 7
		return capture_global_only_p
	}()
}

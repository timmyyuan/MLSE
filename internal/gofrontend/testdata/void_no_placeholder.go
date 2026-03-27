// CHECK-LABEL: func.func @demo.log(%x: i32)
// CHECK-NOT: go.todo "implicit_return_placeholder"
// CHECK: return
package demo

func log(x int) {
}

// CHECK-LABEL: func.func @demo.must() -> i64
// CHECK: func.call @demo.panic
// CHECK-NOT: go.todo "implicit_return_placeholder"
package demo

func must() int {
	panic("boom")
}

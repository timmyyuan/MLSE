// CHECK-LABEL: func.func @demo.sign(%x: i64) -> i64
// CHECK: go.todo "implicit_return_placeholder"
package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
}

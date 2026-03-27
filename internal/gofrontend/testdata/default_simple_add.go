// MLSE-COMPILE: default
// CHECK-LABEL: func.func @demo.add(%a: i32, %b: i32) -> i32
// CHECK: arith.addi %a, %b : i32
// CHECK-NOT: mlse.
package demo

func add(a int, b int) int {
	c := a + b
	return c
}

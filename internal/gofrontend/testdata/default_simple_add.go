// MLSE-COMPILE: default
// CHECK-LABEL: func.func @demo.add(%a: i64, %b: i64) -> i64
// CHECK: arith.addi %a, %b : i64
// CHECK-NOT: mlse.
package demo

func add(a int, b int) int {
	c := a + b
	return c
}

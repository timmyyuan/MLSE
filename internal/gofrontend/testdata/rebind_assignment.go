// CHECK-LABEL: func.func @demo.bump(%x: i32) -> i32
// CHECK: arith.addi %x,
// CHECK-NOT: %x = %
package demo

func bump(x int) int {
	x = x + 1
	return x
}

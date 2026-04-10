// CHECK-LABEL: func.func @demo.ops(%a: i16, %b: i16, %c: i16, %d: i16) -> (i16, i16, i16, i16, i16, i16)
// CHECK-NOT: go.todo_value "binary__"
// CHECK-NOT: go.todo_value "unary__"
// CHECK: arith.andi
// CHECK: arith.ori
// CHECK: arith.xori
// CHECK: arith.remui
// CHECK: arith.constant -1 : i16
package demo

func ops(a, b uint16, c, d int16) (uint16, uint16, uint16, uint16, int16, int16) {
	x := a & b
	y := a | b
	z := a ^ b
	m := a % b
	n := ^c
	p := +d
	return x, y, z, m, n, p
}

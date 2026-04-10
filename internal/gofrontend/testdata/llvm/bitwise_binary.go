// LLVM-LABEL: define { i16, i16, i16, i16, i16, i16 } @demo.ops
// LLVM: and i16
// LLVM: or i16
// LLVM: xor i16
// LLVM: urem i16
// LLVM: xor i16 %{{.*}}, -1
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

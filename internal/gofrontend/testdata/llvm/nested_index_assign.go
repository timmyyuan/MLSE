// MLSE-COMPILE: formal
// LLVM-LABEL: define void @demo.set()
package demo

var g [6][1]int

func set() {
	g[5][0] = -16
}

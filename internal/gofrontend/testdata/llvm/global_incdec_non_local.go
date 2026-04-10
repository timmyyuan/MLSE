// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.f
// LLVM: call i64 @g()
// LLVM: sub i64
// LLVM: ret i64
package demo

var g uint64

func f() uint64 {
	g--
	return g
}

// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.setAndRead
// LLVM: getelementptr i8, ptr %0, i64 8
// LLVM: store i64
// LLVM: getelementptr i8, ptr %0, i64 8
// LLVM: load i64
// LLVM-NOT: @runtime.field.addr.X
package demo

type Holder struct {
	Ready bool
	X     int
}

func setAndRead(h *Holder, v int) int {
	h.X = v
	return h.X
}

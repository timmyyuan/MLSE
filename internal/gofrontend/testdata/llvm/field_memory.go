// MLSE-COMPILE: formal
// LLVM-LABEL: define i32 @demo.setAndRead
// LLVM: call ptr @runtime.field.addr.X
// LLVM: store i32
// LLVM: call ptr @runtime.field.addr.X
// LLVM: load i32
package demo

type Holder struct {
	X int
}

func setAndRead(h *Holder, v int) int {
	h.X = v
	return h.X
}

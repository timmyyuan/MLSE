// MLSE-COMPILE: formal
// LLVM-LABEL: define ptr @demo.alloc()
// LLVM: call ptr @runtime.newobject(i64 8, i64 8)
package demo

func alloc() *int {
	return new(int)
}

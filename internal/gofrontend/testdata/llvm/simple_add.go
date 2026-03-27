// MLSE-COMPILE: default
// LLVM-LABEL: define i32 @demo.add(i32 %0, i32 %1)
// LLVM: %[[SUM:[0-9]+]] = add i32 %0, %1
// LLVM: ret i32 %[[SUM]]
package demo

func add(a int, b int) int {
	c := a + b
	return c
}

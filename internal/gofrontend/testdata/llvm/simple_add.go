// MLSE-COMPILE: default
// LLVM-LABEL: define i64 @demo.add(i64 %0, i64 %1)
// LLVM: %[[SUM:[0-9]+]] = add i64 %0, %1
// LLVM: ret i64 %[[SUM]]
package demo

func add(a int, b int) int {
	c := a + b
	return c
}

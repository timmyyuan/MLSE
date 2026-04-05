// MLSE-COMPILE: formal
// LLVM-LABEL: define { ptr, i64, i64 } @demo.spread
// LLVM: extractvalue { ptr, i64, i64 } %1, 1
// LLVM: icmp ugt i64
// LLVM: call { ptr, i64, i64 } @runtime.growslice
// LLVM: getelementptr i64
// LLVM: call void @llvm.memmove
package demo

func spread(dst []int, src []int) []int {
	return append(dst, src...)
}

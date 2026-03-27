// MLSE-COMPILE: formal
// LLVM-LABEL: define { ptr, i64, i64 } @demo.push
// LLVM: extractvalue { ptr, i64, i64 } %0, 1
// LLVM: extractvalue { ptr, i64, i64 } %0, 2
// LLVM: add i64 %{{.*}}, 1
// LLVM: icmp ugt i64
// LLVM: call { ptr, i64, i64 } @runtime.growslice
// LLVM: getelementptr i32
// LLVM: store i32 1,
package demo

func push(xs []int) []int {
	return append(xs, 1)
}

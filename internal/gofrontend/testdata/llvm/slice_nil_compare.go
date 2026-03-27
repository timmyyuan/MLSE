// MLSE-COMPILE: formal
// LLVM-LABEL: define i1 @demo.isNil({ ptr, i64, i64 } %0)
// LLVM: extractvalue { ptr, i64, i64 } %0, 0
// LLVM: extractvalue { ptr, i64, i64 } %0, 1
// LLVM: extractvalue { ptr, i64, i64 } %0, 2
// LLVM: icmp eq ptr
// LLVM: icmp eq i64
// LLVM: icmp eq i64
// LLVM-LABEL: define i1 @demo.isNotNil({ ptr, i64, i64 } %0)
// LLVM: extractvalue { ptr, i64, i64 } %0, 0
// LLVM: extractvalue { ptr, i64, i64 } %0, 1
// LLVM: extractvalue { ptr, i64, i64 } %0, 2
// LLVM: icmp eq ptr
// LLVM: icmp eq i64
// LLVM: icmp eq i64
package demo

func isNil(xs []int) bool {
	return xs == nil
}

func isNotNil(xs []int) bool {
	return xs != nil
}

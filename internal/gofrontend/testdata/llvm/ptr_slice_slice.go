// MLSE-COMPILE: formal
// LLVM-LABEL: define void @demo.push
// LLVM: call { ptr, i64, i64 } @runtime.growslice
// LLVM: store { ptr, i64, i64 }
package demo

func push(dataList *[][]byte, raw []byte) {
	*dataList = append(*dataList, raw)
}

// MLSE-COMPILE: formal
// LLVM-LABEL: define { ptr, i64 } @demo.extendTag({ ptr, i64 } %0, { ptr, i64 } %1)
// LLVM: call i1 @runtime.neq.string
// LLVM: call { ptr, i64 } @runtime.add.string
// LLVM: call { ptr, i64 } @runtime.add.string
// LLVM: call { ptr, i64 } @runtime.add.string
package demo

func extendTag(tag string, name string) string {
	if tag != "" {
		tag += "$$"
	}
	tag += "_podname||" + name
	return tag
}

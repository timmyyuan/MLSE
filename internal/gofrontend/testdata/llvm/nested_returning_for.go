// LLVM-LABEL: define i64 @demo.nested_returning_for(i1
// LLVM: br i1
// LLVM: ret i64
package demo

func nested_returning_for(flag bool) int {
	var i int
	var j int
	for i = 0; i < 2; i++ {
		if flag {
			continue
		}
		for j = 0; j < 3; j++ {
			return j + 1
		}
	}
	return 0
}

// MLSE-COMPILE: default
// LLVM-LABEL: define { ptr, i64, i64 } @demo.build(i32 %0)
// LLVM: call { ptr, i64, i64 } @runtime.makeslice
// LLVM-LABEL: define { ptr, i64 } @demo.empty()
// LLVM: @go.string.constant.
// LLVM-NOT: __mlse_go_string_constant_
// LLVM-LABEL: define ptr @demo.zero()
// LLVM: ret ptr null
package demo

type Box struct{}

func empty() string {
	return ""
}

func build(n int) []int {
	return make([]int, n)
}

func zero() *Box {
	return nil
}

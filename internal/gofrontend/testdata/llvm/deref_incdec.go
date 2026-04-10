// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.bump
// LLVM: load i64
// LLVM: add i64
// LLVM: store i64
// LLVM-LABEL: define i64 @demo.drop
// LLVM: load i64
// LLVM: sub i64
// LLVM: store i64
package demo

func bump(p *int) int {
	(*p)++
	return *p
}

func drop(p *int) int {
	(*p)--
	return *p
}

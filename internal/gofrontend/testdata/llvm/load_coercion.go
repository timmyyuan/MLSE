// MLSE-COMPILE: formal
// LLVM-LABEL: define i16 @demo.read16
// LLVM: load i64
// LLVM: trunc i64
package demo

func read16(p *int64) int16 {
	return int16(*p)
}

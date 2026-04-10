// MLSE-COMPILE: formal
// LLVM-LABEL: define i64 @demo.widen()
// LLVM: call i8 @demo.widen.__lit0()
// LLVM: sext i8
package demo

func widen() int64 {
	fn := func() int8 {
		return 1
	}
	return int64(fn())
}

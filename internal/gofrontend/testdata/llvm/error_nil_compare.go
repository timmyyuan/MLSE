// MLSE-COMPILE: formal
// LLVM-LABEL: define i1 @demo.isNil(ptr %0)
// LLVM: icmp eq ptr %0, null
package demo

func isNil(err error) bool {
	return err == nil
}

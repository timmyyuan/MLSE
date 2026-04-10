// LLVM-LABEL: define i64 @demo.zero_trip_returning_for()
// LLVM: icmp eq i64
// LLVM: ret i64 %
package demo

func zero_trip_returning_for() int {
	for i := 0; i == 2; i++ {
		return i
	}
	return 7
}

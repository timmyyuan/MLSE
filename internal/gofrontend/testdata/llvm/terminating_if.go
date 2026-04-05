// LLVM-LABEL: define i64 @demo.sign(i64 %0)
// LLVM: %[[CMP:[0-9]+]] = icmp sgt i64 %0, 0
// LLVM: br i1 %[[CMP]], label %[[THEN:[0-9]+]], label %[[ELSE:[0-9]+]]
// LLVM: [[MERGE:[0-9]+]]:
// LLVM: %[[PHI:[0-9]+]] = phi i64 [ 0, %[[ELSE]] ], [ 1, %[[THEN]] ]
// LLVM: ret i64 %[[PHI]]
package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}

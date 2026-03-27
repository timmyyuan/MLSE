// LLVM-LABEL: define i32 @demo.sign(i32 %0)
// LLVM: %[[CMP:[0-9]+]] = icmp sgt i32 %0, 0
// LLVM: br i1 %[[CMP]], label %[[THEN:[0-9]+]], label %[[ELSE:[0-9]+]]
// LLVM: [[MERGE:[0-9]+]]:
// LLVM: %[[PHI:[0-9]+]] = phi i32 [ 0, %[[ELSE]] ], [ 1, %[[THEN]] ]
// LLVM: ret i32 %[[PHI]]
package demo

func sign(x int) int {
	if x > 0 {
		return 1
	}
	return 0
}

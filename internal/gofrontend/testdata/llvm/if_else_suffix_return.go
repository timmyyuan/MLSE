// LLVM-LABEL: define i64 @demo.choose(i1 %0, i64 %1)
// LLVM: br i1 %0, label %[[THEN:[0-9]+]], label %[[ELSE:[0-9]+]]
// LLVM: [[THEN]]:
// LLVM: br label %[[MERGE:[0-9]+]]
// LLVM: [[ELSE]]:
// LLVM: %[[ADD:[0-9]+]] = add i64 %1, 2
// LLVM: br label %[[MERGE]]
// LLVM: [[MERGE]]:
// LLVM: %[[PHI:[0-9]+]] = phi i64 [ %[[ADD]], %[[ELSE]] ], [ 1, %[[THEN]] ]
// LLVM: br label %[[RETBLK:[0-9]+]]
// LLVM: [[RETBLK]]:
// LLVM: ret i64 %[[PHI]]
package demo

func choose(flag bool, x int) int {
	if flag {
		return 1
	} else {
		x = x + 2
	}
	return x
}

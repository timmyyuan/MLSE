; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

define i32 @sign(i32 %0) {
  %2 = alloca i32, align 4
  store i32 %0, ptr %2, align 4
  %3 = load i32, ptr %2, align 4
  %4 = icmp sgt i32 %3, 0
  br i1 %4, label %5, label %6

5:                                                ; preds = %1
  ret i32 1

6:                                                ; preds = %1
  ret i32 0
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

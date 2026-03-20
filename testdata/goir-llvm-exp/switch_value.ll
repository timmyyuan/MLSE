; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

define i32 @classify(i32 %0) {
  %2 = alloca i32, align 4
  store i32 %0, ptr %2, align 4
  %3 = load i32, ptr %2, align 4
  br label %4

4:                                                ; preds = %1
  %5 = icmp eq i32 %3, 0
  br i1 %5, label %6, label %7

6:                                                ; preds = %4
  ret i32 10

7:                                                ; preds = %4
  %8 = icmp eq i32 %3, 1
  br i1 %8, label %9, label %10

9:                                                ; preds = %7
  ret i32 20

10:                                               ; preds = %7
  ret i32 30
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

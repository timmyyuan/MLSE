; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

define i32 @chooseMerge(i1 %0) {
  %2 = alloca i1, align 1
  store i1 %0, ptr %2, align 1
  %3 = alloca i32, align 4
  store i32 0, ptr %3, align 4
  %4 = load i1, ptr %2, align 1
  br i1 %4, label %5, label %6

5:                                                ; preds = %1
  store i32 1, ptr %3, align 4
  br label %7

6:                                                ; preds = %1
  store i32 2, ptr %3, align 4
  br label %7

7:                                                ; preds = %5, %6
  %8 = load i32, ptr %3, align 4
  ret i32 %8
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

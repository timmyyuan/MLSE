; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

define i32 @choose(i1 %0) {
  %2 = alloca i1, align 1
  store i1 %0, ptr %2, align 1
  %3 = load i1, ptr %2, align 1
  br i1 %3, label %4, label %5

4:                                                ; preds = %1
  ret i32 1

5:                                                ; preds = %1
  ret i32 2
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

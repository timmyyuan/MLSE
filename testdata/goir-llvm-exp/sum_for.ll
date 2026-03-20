; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

define i32 @sumTo(i32 %0) {
  %2 = alloca i32, align 4
  store i32 %0, ptr %2, align 4
  %3 = alloca i32, align 4
  %4 = alloca i32, align 4
  store i32 0, ptr %3, align 4
  store i32 0, ptr %4, align 4
  br label %5

5:                                                ; preds = %9, %1
  %6 = load i32, ptr %4, align 4
  %7 = load i32, ptr %2, align 4
  %8 = icmp slt i32 %6, %7
  br i1 %8, label %9, label %15

9:                                                ; preds = %5
  %10 = load i32, ptr %3, align 4
  %11 = load i32, ptr %4, align 4
  %12 = add i32 %10, %11
  store i32 %12, ptr %3, align 4
  %13 = load i32, ptr %4, align 4
  %14 = add i32 %13, 1
  store i32 %14, ptr %4, align 4
  br label %5

15:                                               ; preds = %5
  %16 = load i32, ptr %3, align 4
  ret i32 %16
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

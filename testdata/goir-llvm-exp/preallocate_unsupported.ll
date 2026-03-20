; ModuleID = 'LLVMDialectModule'
source_filename = "LLVMDialectModule"

declare ptr @preallocExtendTrunc(ptr, i64)

define ptr @preallocExtend(ptr %0, i64 %1) {
  %3 = alloca ptr, align 8
  store ptr %0, ptr %3, align 8
  %4 = alloca i64, align 8
  store i64 %1, ptr %4, align 8
  %5 = alloca ptr, align 8
  %6 = load ptr, ptr %3, align 8
  %7 = load i64, ptr %4, align 8
  %8 = call ptr @preallocExtendTrunc(ptr %6, i64 %7)
  store ptr %8, ptr %5, align 8
  %9 = load ptr, ptr %5, align 8
  ret ptr %9
}

define ptr @preallocFixed(ptr %0, i64 %1) {
  %3 = alloca ptr, align 8
  store ptr %0, ptr %3, align 8
  %4 = alloca i64, align 8
  store i64 %1, ptr %4, align 8
  ret ptr null
}

!llvm.module.flags = !{!0}

!0 = !{i32 2, !"Debug Info Version", i32 3}

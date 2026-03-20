; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

declare ptr @preallocExtendTrunc(ptr, i64)

define ptr @preallocExtend(ptr %f, i64 %sizeInBytes) {
entry:
  %slot0 = alloca ptr
  store ptr %f, ptr %slot0
  %slot1 = alloca i64
  store i64 %sizeInBytes, ptr %slot1
  %slot5 = alloca ptr
  %load2 = load ptr, ptr %slot0
  %load3 = load i64, ptr %slot1
  %call4 = call ptr @preallocExtendTrunc(ptr %load2, i64 %load3)
  store ptr %call4, ptr %slot5
  %load6 = load ptr, ptr %slot5
  ret ptr %load6
}

define ptr @preallocFixed(ptr %f, i64 %sizeInBytes) {
entry:
  %slot0 = alloca ptr
  store ptr %f, ptr %slot0
  %slot1 = alloca i64
  store i64 %sizeInBytes, ptr %slot1
  ret ptr null
}

; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @chooseMerge(i1 %b) {
entry:
  %slot0 = alloca i1
  store i1 %b, ptr %slot0
  %slot1 = alloca i32
  store i32 0, ptr %slot1
  %load2 = load i1, ptr %slot0
  br i1 %load2, label %if.then0, label %if.else1
if.then0:
  store i32 1, ptr %slot1
  br label %if.end2
if.else1:
  store i32 2, ptr %slot1
  br label %if.end2
if.end2:
  %load3 = load i32, ptr %slot1
  ret i32 %load3
}

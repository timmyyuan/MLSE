; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @choose(i1 %b) {
entry:
  %slot0 = alloca i1
  store i1 %b, ptr %slot0
  %load1 = load i1, ptr %slot0
  br i1 %load1, label %if.then0, label %if.else1
if.then0:
  ret i32 1
if.else1:
  ret i32 2
}

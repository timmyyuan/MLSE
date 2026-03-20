; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @sign(i32 %x) {
entry:
  %slot0 = alloca i32
  store i32 %x, ptr %slot0
  %load1 = load i32, ptr %slot0
  %ifcond2 = icmp sgt i32 %load1, 0
  br i1 %ifcond2, label %if.then0, label %if.end1
if.then0:
  ret i32 1
if.end1:
  ret i32 0
}

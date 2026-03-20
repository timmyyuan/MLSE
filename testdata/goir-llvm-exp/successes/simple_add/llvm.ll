; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @add(i32 %a, i32 %b) {
entry:
  %slot0 = alloca i32
  store i32 %a, ptr %slot0
  %slot1 = alloca i32
  store i32 %b, ptr %slot1
  %slot5 = alloca i32
  %load2 = load i32, ptr %slot0
  %load3 = load i32, ptr %slot1
  %tmp4 = add i32 %load2, %load3
  store i32 %tmp4, ptr %slot5
  %load6 = load i32, ptr %slot5
  ret i32 %load6
}

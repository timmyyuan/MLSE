; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @sumTo(i32 %n) {
entry:
  %slot0 = alloca i32
  store i32 %n, ptr %slot0
  %slot1 = alloca i32
  %slot2 = alloca i32
  store i32 0, ptr %slot1
  store i32 0, ptr %slot2
  br label %for.cond0
for.cond0:
  %load3 = load i32, ptr %slot2
  %load4 = load i32, ptr %slot0
  %ifcond5 = icmp slt i32 %load3, %load4
  br i1 %ifcond5, label %for.body1, label %for.end2
for.body1:
  %load6 = load i32, ptr %slot1
  %load7 = load i32, ptr %slot2
  %tmp8 = add i32 %load6, %load7
  store i32 %tmp8, ptr %slot1
  %load9 = load i32, ptr %slot2
  %tmp10 = add i32 %load9, 1
  store i32 %tmp10, ptr %slot2
  br label %for.cond0
for.end2:
  %load11 = load i32, ptr %slot1
  ret i32 %load11
}

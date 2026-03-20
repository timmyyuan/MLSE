; Experimental translation from MLSE GoIR-like text to LLVM IR.
; This path only supports a tiny additive subset and is not canonical lowering.

define i32 @classify(i32 %x) {
entry:
  %slot0 = alloca i32
  store i32 %x, ptr %slot0
  %load1 = load i32, ptr %slot0
  br label %switch.case.check2
switch.case.check2:
  %switchcmp2 = icmp eq i32 %load1, 0
  br i1 %switchcmp2, label %switch.case.body3, label %switch.case.check4
switch.case.body3:
  ret i32 10
switch.case.check4:
  %switchcmp3 = icmp eq i32 %load1, 1
  br i1 %switchcmp3, label %switch.case.body5, label %switch.default1
switch.case.body5:
  ret i32 20
switch.default1:
  ret i32 30
}

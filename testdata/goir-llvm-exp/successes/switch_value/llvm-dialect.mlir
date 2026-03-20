module {
  llvm.func @classify(%x: i32) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %x, %slot1 {alignment = 4 : i64} : i32, !llvm.ptr
    %zero3 = llvm.mlir.zero : i32
    %c5 = llvm.mlir.constant(10 : i32) : i32
    %c7 = llvm.mlir.constant(20 : i32) : i32
    %c8 = llvm.mlir.constant(30 : i32) : i32
    %load2 = llvm.load %slot1 {alignment = 4 : i64} : !llvm.ptr -> i32
    llvm.br ^switch.case.check2
  ^switch.case.check2:
    %switchcmp4 = llvm.icmp "eq" %load2, %zero3 : i32
    llvm.cond_br %switchcmp4, ^switch.case.body3, ^switch.case.check4
  ^switch.case.body3:
    llvm.return %c5 : i32
  ^switch.case.check4:
    %switchcmp6 = llvm.icmp "eq" %load2, %c0 : i32
    llvm.cond_br %switchcmp6, ^switch.case.body5, ^switch.default1
  ^switch.case.body5:
    llvm.return %c7 : i32
  ^switch.default1:
    llvm.return %c8 : i32
  }
}

module {
  llvm.func @sumTo(%n: i32) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %n, %slot1 {alignment = 4 : i64} : i32, !llvm.ptr
    %zero2 = llvm.mlir.zero : i32
    %slot3 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    %slot4 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %zero2, %slot3 {alignment = 4 : i64} : i32, !llvm.ptr
    llvm.store %zero2, %slot4 {alignment = 4 : i64} : i32, !llvm.ptr
    llvm.br ^for.cond0
  ^for.cond0:
    %load5 = llvm.load %slot4 {alignment = 4 : i64} : !llvm.ptr -> i32
    %load6 = llvm.load %slot1 {alignment = 4 : i64} : !llvm.ptr -> i32
    %ifcond7 = llvm.icmp "slt" %load5, %load6 : i32
    llvm.cond_br %ifcond7, ^for.body1, ^for.end2
  ^for.body1:
    %load8 = llvm.load %slot3 {alignment = 4 : i64} : !llvm.ptr -> i32
    %load9 = llvm.load %slot4 {alignment = 4 : i64} : !llvm.ptr -> i32
    %tmp10 = llvm.add %load8, %load9 : i32
    llvm.store %tmp10, %slot3 {alignment = 4 : i64} : i32, !llvm.ptr
    %load11 = llvm.load %slot4 {alignment = 4 : i64} : !llvm.ptr -> i32
    %tmp12 = llvm.add %load11, %c0 : i32
    llvm.store %tmp12, %slot4 {alignment = 4 : i64} : i32, !llvm.ptr
    llvm.br ^for.cond0
  ^for.end2:
    %load13 = llvm.load %slot3 {alignment = 4 : i64} : !llvm.ptr -> i32
    llvm.return %load13 : i32
  }
}

module {
  llvm.func @chooseMerge(%b: i1) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i1 {alignment = 1 : i64} : (i32) -> !llvm.ptr
    llvm.store %b, %slot1 {alignment = 1 : i64} : i1, !llvm.ptr
    %zero2 = llvm.mlir.zero : i32
    %slot3 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    %c5 = llvm.mlir.constant(2 : i32) : i32
    llvm.store %zero2, %slot3 {alignment = 4 : i64} : i32, !llvm.ptr
    %load4 = llvm.load %slot1 {alignment = 1 : i64} : !llvm.ptr -> i1
    llvm.cond_br %load4, ^if.then0, ^if.else1
  ^if.then0:
    llvm.store %c0, %slot3 {alignment = 4 : i64} : i32, !llvm.ptr
    llvm.br ^if.end2
  ^if.else1:
    llvm.store %c5, %slot3 {alignment = 4 : i64} : i32, !llvm.ptr
    llvm.br ^if.end2
  ^if.end2:
    %load6 = llvm.load %slot3 {alignment = 4 : i64} : !llvm.ptr -> i32
    llvm.return %load6 : i32
  }
}

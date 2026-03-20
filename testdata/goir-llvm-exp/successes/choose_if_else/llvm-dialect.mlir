module {
  llvm.func @choose(%b: i1) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i1 {alignment = 1 : i64} : (i32) -> !llvm.ptr
    llvm.store %b, %slot1 {alignment = 1 : i64} : i1, !llvm.ptr
    %c3 = llvm.mlir.constant(2 : i32) : i32
    %load2 = llvm.load %slot1 {alignment = 1 : i64} : !llvm.ptr -> i1
    llvm.cond_br %load2, ^if.then0, ^if.else1
  ^if.then0:
    llvm.return %c0 : i32
  ^if.else1:
    llvm.return %c3 : i32
  }
}

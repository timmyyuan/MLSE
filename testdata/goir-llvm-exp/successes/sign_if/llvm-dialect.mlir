module {
  llvm.func @sign(%x: i32) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %x, %slot1 {alignment = 4 : i64} : i32, !llvm.ptr
    %zero3 = llvm.mlir.zero : i32
    %load2 = llvm.load %slot1 {alignment = 4 : i64} : !llvm.ptr -> i32
    %ifcond4 = llvm.icmp "sgt" %load2, %zero3 : i32
    llvm.cond_br %ifcond4, ^if.then0, ^if.end1
  ^if.then0:
    llvm.return %c0 : i32
  ^if.end1:
    llvm.return %zero3 : i32
  }
}

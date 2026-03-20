module {
  llvm.func @add(%a: i32, %b: i32) -> i32 {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %a, %slot1 {alignment = 4 : i64} : i32, !llvm.ptr
    %slot2 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    llvm.store %b, %slot2 {alignment = 4 : i64} : i32, !llvm.ptr
    %slot6 = llvm.alloca %c0 x i32 {alignment = 4 : i64} : (i32) -> !llvm.ptr
    %load3 = llvm.load %slot1 {alignment = 4 : i64} : !llvm.ptr -> i32
    %load4 = llvm.load %slot2 {alignment = 4 : i64} : !llvm.ptr -> i32
    %tmp5 = llvm.add %load3, %load4 : i32
    llvm.store %tmp5, %slot6 {alignment = 4 : i64} : i32, !llvm.ptr
    %load7 = llvm.load %slot6 {alignment = 4 : i64} : !llvm.ptr -> i32
    llvm.return %load7 : i32
  }
}

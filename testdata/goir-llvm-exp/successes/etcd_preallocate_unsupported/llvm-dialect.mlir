module {
  llvm.func @preallocExtendTrunc(!llvm.ptr, i64) -> !llvm.ptr

  llvm.func @preallocExtend(%f: !llvm.ptr, %sizeInBytes: i64) -> !llvm.ptr {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x !llvm.ptr {alignment = 8 : i64} : (i32) -> !llvm.ptr
    llvm.store %f, %slot1 {alignment = 8 : i64} : !llvm.ptr, !llvm.ptr
    %slot2 = llvm.alloca %c0 x i64 {alignment = 8 : i64} : (i32) -> !llvm.ptr
    llvm.store %sizeInBytes, %slot2 {alignment = 8 : i64} : i64, !llvm.ptr
    %slot6 = llvm.alloca %c0 x !llvm.ptr {alignment = 8 : i64} : (i32) -> !llvm.ptr
    %load3 = llvm.load %slot1 {alignment = 8 : i64} : !llvm.ptr -> !llvm.ptr
    %load4 = llvm.load %slot2 {alignment = 8 : i64} : !llvm.ptr -> i64
    %call5 = llvm.call @preallocExtendTrunc(%load3, %load4) : (!llvm.ptr, i64) -> !llvm.ptr
    llvm.store %call5, %slot6 {alignment = 8 : i64} : !llvm.ptr, !llvm.ptr
    %load7 = llvm.load %slot6 {alignment = 8 : i64} : !llvm.ptr -> !llvm.ptr
    llvm.return %load7 : !llvm.ptr
  }

  llvm.func @preallocFixed(%f: !llvm.ptr, %sizeInBytes: i64) -> !llvm.ptr {
    %c0 = llvm.mlir.constant(1 : i32) : i32
    %slot1 = llvm.alloca %c0 x !llvm.ptr {alignment = 8 : i64} : (i32) -> !llvm.ptr
    llvm.store %f, %slot1 {alignment = 8 : i64} : !llvm.ptr, !llvm.ptr
    %slot2 = llvm.alloca %c0 x i64 {alignment = 8 : i64} : (i32) -> !llvm.ptr
    llvm.store %sizeInBytes, %slot2 {alignment = 8 : i64} : i64, !llvm.ptr
    %zero3 = llvm.mlir.zero : !llvm.ptr
    llvm.return %zero3 : !llvm.ptr
  }
}

module {
  func.func @sign(%x: i32) -> i32 {
    mlse.if arith.cmpi_gt %x, 0 : i32 {
        return 1 : i32
    }
    return 0 : i32
  }
}

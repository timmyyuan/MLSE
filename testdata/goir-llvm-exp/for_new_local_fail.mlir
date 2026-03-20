module {
  func.func @badLoop(%n: i32) -> i32 {
    %i = 0 : i32
    mlse.for arith.cmpi_lt %i, %n : i32 {
      %next = arith.addi %i, 1 : i32
      %i = %next : i32
    }
    return %i : i32
  }
}

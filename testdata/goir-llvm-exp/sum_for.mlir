module {
  func.func @sumTo(%n: i32) -> i32 {
    %sum = 0 : i32
    %i = 0 : i32
    mlse.for arith.cmpi_lt %i, %n : i32 {
        %sum = arith.addi %sum, %i : i32
        %i = arith.addi %i, 1 : i32
    }
    return %sum : i32
  }
}

module {
  func.func @add(%a: i32, %b: i32) -> i32 {
    %c = arith.addi %a, %b : i32
    return %c : i32
  }
}

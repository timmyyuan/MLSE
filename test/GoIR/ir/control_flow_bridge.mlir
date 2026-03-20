module {
  func.func @choose(%flag: i1) -> i32 {
    %value = scf.if %flag -> (i32) {
      %one = arith.constant 1 : i32
      scf.yield %one : i32
    } else {
      %two = arith.constant 2 : i32
      scf.yield %two : i32
    }
    return %value : i32
  }

  func.func @sum_to(%n: i32) -> i32 {
    %zero = arith.constant 0 : i32
    %zero_idx = arith.index_cast %zero : i32 to index
    %n_idx = arith.index_cast %n : i32 to index
    %step = arith.constant 1 : index
    %sum = scf.for %iv = %zero_idx to %n_idx step %step iter_args(%acc = %zero) -> (i32) {
      %iv_i32 = arith.index_cast %iv : index to i32
      %next = arith.addi %acc, %iv_i32 : i32
      scf.yield %next : i32
    }
    return %sum : i32
  }
}

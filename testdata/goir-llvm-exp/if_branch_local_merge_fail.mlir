module {
  func.func @badMerge(%b: i1) -> i32 {
    mlse.if %b : i1 {
      %x = 1 : i32
    } else {
      %x = 2 : i32
    }
    return %x : i32
  }
}

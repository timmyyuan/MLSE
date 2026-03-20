module {
  func.func @choose(%b: i1) -> i32 {
    mlse.if %b : i1 {
        return 1 : i32
    } else {
        return 2 : i32
    }
  }
}

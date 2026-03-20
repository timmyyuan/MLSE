module {
  func.func @classify(%x: i32) -> i32 {
    mlse.switch %x : i32 {
      case 0 : i32 {
          return 10 : i32
      }
      case 1 : i32 {
          return 20 : i32
      }
      default {
          return 30 : i32
      }
    }
  }
}

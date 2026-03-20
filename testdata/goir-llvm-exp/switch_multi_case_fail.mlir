module {
  func.func @badSwitch(%x: i32) -> i32 {
    mlse.switch %x : i32 {
      case 0, 1 : i32 {
        return 10 : i32
      }
      default {
        return 20 : i32
      }
    }
  }
}

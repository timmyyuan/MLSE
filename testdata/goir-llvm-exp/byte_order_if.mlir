module {
  func.func @ByteOrder() -> !go.sel<"binary.ByteOrder"> {
    mlse.if mlse.select %cpu.IsBigEndian : !go.any {
        return mlse.select %binary.BigEndian : !go.any
    }
    return mlse.select %binary.LittleEndian : !go.any
  }
}

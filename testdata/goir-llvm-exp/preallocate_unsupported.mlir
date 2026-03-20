module {
  func.func @preallocExtend(%f: !go.ptr<!go.sel<"os.File">>, %sizeInBytes: !go.named<"int64">) -> !go.error {
    %call1 = mlse.call %preallocExtendTrunc(%f, %sizeInBytes) : !go.any
    return %call1 : !go.any
  }
  func.func @preallocFixed(%f: !go.ptr<!go.sel<"os.File">>, %sizeInBytes: !go.named<"int64">) -> !go.error {
    return mlse.nil : !go.nil
  }
}

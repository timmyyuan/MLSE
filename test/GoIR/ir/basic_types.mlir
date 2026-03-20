module {
  func.func @identity_slice(%xs: !go.slice<i32>) -> !go.slice<i32> {
    return %xs : !go.slice<i32>
  }

  func.func @identity_ptr(%ctx: !go.ptr<!go.named<"context.Context">>) -> !go.ptr<!go.named<"context.Context">> {
    return %ctx : !go.ptr<!go.named<"context.Context">>
  }

  func.func @pair(%message: !go.string, %err: !go.error) -> (!go.string, !go.error) {
    return %message, %err : !go.string, !go.error
  }
}

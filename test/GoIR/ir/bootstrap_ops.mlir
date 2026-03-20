module {
  func.func @build_slice(%n: i64) -> !go.slice<i32> {
    %two_n = arith.addi %n, %n : i64
    %slice = go.make_slice %n, %two_n : i64 to !go.slice<i32>
    return %slice : !go.slice<i32>
  }

  func.func @maybe_error() -> !go.error {
    %nil = go.nil : !go.error
    return %nil : !go.error
  }

  func.func @maybe_buffer() -> !go.ptr<!go.named<"bytes.Buffer">> {
    %nil = go.nil : !go.ptr<!go.named<"bytes.Buffer">>
    return %nil : !go.ptr<!go.named<"bytes.Buffer">>
  }
}

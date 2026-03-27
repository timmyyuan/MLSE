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

  func.func @slice_builtins(%xs: !go.slice<i32>, %i: i64, %v: i32) -> (!go.slice<i32>, i64, i64, i32) {
    %len = go.len %xs : !go.slice<i32> -> i64
    %cap = go.cap %xs : !go.slice<i32> -> i64
    %elt_addr = go.elem_addr %xs, %i : (!go.slice<i32>, i64) -> !go.ptr<i32>
    %elt = go.load %elt_addr : !go.ptr<i32> -> i32
    %out = go.append %xs, %v : (!go.slice<i32>, i32) -> !go.slice<i32>
    return %out, %len, %cap, %elt : !go.slice<i32>, i64, i64, i32
  }

  func.func @slice_spread_builtins(%dst: !go.slice<i32>, %src: !go.slice<i32>) -> !go.slice<i32> {
    %out = go.append_slice %dst, %src : (!go.slice<i32>, !go.slice<i32>) -> !go.slice<i32>
    return %out : !go.slice<i32>
  }

  func.func @string_builtins(%s: !go.string, %i: i64) -> (i64, i8) {
    %len = go.len %s : !go.string -> i64
    %ch = go.index %s, %i : (!go.string, i64) -> i8
    return %len, %ch : i64, i8
  }

  func.func @compare_values(%s: !go.string, %p: !go.ptr<i32>, %err: !go.error, %xs: !go.slice<i32>) -> (i1, i1, i1, i1) {
    %empty = go.string_constant "" : !go.string
    %string_neq = go.neq %s, %empty : (!go.string, !go.string) -> i1
    %nil_ptr = go.nil : !go.ptr<i32>
    %ptr_eq = go.eq %p, %nil_ptr : (!go.ptr<i32>, !go.ptr<i32>) -> i1
    %nil_err = go.nil : !go.error
    %err_eq = go.eq %err, %nil_err : (!go.error, !go.error) -> i1
    %nil_slice = go.nil : !go.slice<i32>
    %slice_neq = go.neq %xs, %nil_slice : (!go.slice<i32>, !go.slice<i32>) -> i1
    return %string_neq, %ptr_eq, %err_eq, %slice_neq : i1, i1, i1, i1
  }

  func.func @field_memory(%h: !go.ptr<!go.named<"demo.Holder">>, %v: i32) -> i32 {
    %field = go.field_addr %h, "X" : !go.ptr<!go.named<"demo.Holder">> -> !go.ptr<i32>
    go.store %v, %field : i32 to !go.ptr<i32>
    %loaded = go.load %field : !go.ptr<i32> -> i32
    return %loaded : i32
  }
}

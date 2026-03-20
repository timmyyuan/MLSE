module {
  func.func @greet(%name: !go.string) -> !go.string {
    %format = go.string_constant "hi %s" : !go.string
    %message = go.todo_value "variadic fmt lowering" : !go.string
    return %message : !go.string
  }

  func.func @fallback(%n: i32) -> !go.slice<i32> {
    go.todo "unsupported structured control flow"
    %len = arith.constant 1 : i32
    %slice = go.make_slice %len, %n : i32 to !go.slice<i32>
    return %slice : !go.slice<i32>
  }
}

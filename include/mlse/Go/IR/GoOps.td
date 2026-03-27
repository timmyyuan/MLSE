#ifndef MLSE_GO_IR_GOOPS
#define MLSE_GO_IR_GOOPS

include "mlse/Go/IR/GoTypes.td"
include "mlir/Interfaces/SideEffectInterfaces.td"

def Go_SliceLike : Type<CPred<"::llvm::isa<::mlir::go::SliceType>($_self)">,
                        "go slice type">;
def Go_PointerLike : Type<CPred<"::llvm::isa<::mlir::go::PointerType>($_self)">,
                          "go pointer type">;
def Go_ErrorLike : Type<CPred<"::llvm::isa<::mlir::go::ErrorType>($_self)">,
                        "go error type">;
def Go_NilableType : AnyTypeOf<[Go_SliceLike, Go_PointerLike, Go_ErrorLike],
                               "Go nilable type">;
def Go_LenLike : AnyTypeOf<[Go_StringType, Go_SliceLike],
                           "Go len-capable type">;
def Go_IndexLike : AnyTypeOf<[Go_StringType, Go_SliceLike],
                             "Go indexable type">;
def Go_ElemAddrBaseLike : AnyTypeOf<[Go_SliceLike],
                                    "Go aggregate that exposes element addresses">;
def Go_FieldBaseLike : AnyTypeOf<[Go_NamedType, Go_PointerLike],
                                 "Go aggregate handle that can expose field addresses">;

def Go_StringConstantOp : Go_Op<"string_constant", [ConstantLike, Pure]> {
  let summary = "Go string constant";
  let description = [{
    The `go.string_constant` operation materializes a frontend string value
    without prematurely committing to a lower-level runtime layout.
  }];

  let arguments = (ins StrAttr:$value);
  let results = (outs Go_StringType:$result);
  let assemblyFormat = "$value attr-dict `:` type($result)";
}

def Go_NilOp : Go_Op<"nil", [ConstantLike, Pure]> {
  let summary = "Go nil constant";
  let description = [{
    The `go.nil` operation materializes a typed nil value for Go nilable
    frontend types. This keeps nil explicit in the GoIR layer instead of
    prematurely collapsing it into a lower-level target-specific sentinel.
  }];

  let results = (outs Go_NilableType:$result);
  let assemblyFormat = "attr-dict `:` type($result)";
}

def Go_MakeSliceOp : Go_Op<"make_slice", [Pure, AllTypesMatch<["length", "capacity"]>]> {
  let summary = "construct a Go slice";
  let description = [{
    The `go.make_slice` operation models Go slice construction before runtime
    semantics are lowered away. It keeps both logical `len` and `cap` explicit
    in the frontend IR.
  }];

  let arguments = (ins AnySignlessIntegerOrIndex:$length,
                       AnySignlessIntegerOrIndex:$capacity);
  let results = (outs Go_SliceLike:$result);
  let assemblyFormat = "$length `,` $capacity attr-dict `:` type($length) `to` type($result)";
}

def Go_LenOp : Go_Op<"len", [Pure]> {
  let summary = "query the logical length of a Go string or slice";
  let description = [{
    The `go.len` operation models the Go `len` builtin for values that still
    carry Go-specific layout or runtime semantics in the frontend IR.
  }];

  let arguments = (ins Go_LenLike:$value);
  let results = (outs AnySignlessIntegerOrIndex:$result);
  let assemblyFormat = "$value attr-dict `:` type($value) `->` type($result)";
}

def Go_CapOp : Go_Op<"cap", [Pure]> {
  let summary = "query the logical capacity of a Go slice";
  let description = [{
    The `go.cap` operation models the Go `cap` builtin for slice values before
    runtime semantics are lowered away.
  }];

  let arguments = (ins Go_SliceLike:$value);
  let results = (outs AnySignlessIntegerOrIndex:$result);
  let assemblyFormat = "$value attr-dict `:` type($value) `->` type($result)";
}

def Go_EqOp : Go_Op<"eq", [Pure]> {
  let summary = "compare two Go values for equality";
  let description = [{
    The `go.eq` operation keeps Go-specific equality semantics explicit in the
    frontend IR instead of encoding them in helper symbol names. The verifier
    currently accepts the subset that the formal bridge can lower with stable
    semantics today.
  }];

  let arguments = (ins AnyType:$lhs, AnyType:$rhs);
  let results = (outs I1:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$lhs `,` $rhs attr-dict `:` functional-type(operands, results)";
}

def Go_NeqOp : Go_Op<"neq", [Pure]> {
  let summary = "compare two Go values for inequality";
  let description = [{
    The `go.neq` operation keeps Go-specific inequality semantics explicit in
    the frontend IR instead of encoding them in helper symbol names. The
    verifier currently accepts the subset that the formal bridge can lower with
    stable semantics today.
  }];

  let arguments = (ins AnyType:$lhs, AnyType:$rhs);
  let results = (outs I1:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$lhs `,` $rhs attr-dict `:` functional-type(operands, results)";
}

def Go_IndexOp : Go_Op<"index", [Pure]> {
  let summary = "index a Go value-semantics aggregate";
  let description = [{
    The `go.index` operation models indexed reads that should remain value-like
    in the frontend IR, such as string indexing. Addressable aggregates should
    prefer `go.elem_addr` + `go.load` / `go.store`.
  }];

  let arguments = (ins Go_IndexLike:$value,
                       AnySignlessIntegerOrIndex:$index);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$value `,` $index attr-dict `:` functional-type(operands, results)";
}

def Go_ElemAddrOp : Go_Op<"elem_addr", [Pure]> {
  let summary = "materialize the address of an indexed Go element";
  let description = [{
    The `go.elem_addr` operation isolates indexed address formation from the
    frontend. It is the preferred bridge for addressable indexed aggregates,
    such as slices, before the Go frontend commits to a concrete LLVM-level
    memory layout.
  }];

  let arguments = (ins Go_ElemAddrBaseLike:$base,
                       AnySignlessIntegerOrIndex:$index);
  let results = (outs Go_PointerLike:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$base `,` $index attr-dict `:` functional-type(operands, results)";
}

def Go_AppendOp : Go_Op<"append", [Pure]> {
  let summary = "append values to a Go slice";
  let description = [{
    The `go.append` operation models the Go `append` builtin before slice
    growth and runtime ownership semantics are lowered to a concrete runtime
    contract.
  }];

  let arguments = (ins Go_SliceLike:$slice,
                       Variadic<AnyType>:$values);
  let results = (outs Go_SliceLike:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$slice (`,` $values^)? attr-dict `:` functional-type(operands, results)";
}

def Go_AppendSliceOp : Go_Op<"append_slice", [Pure]> {
  let summary = "append one Go slice into another";
  let description = [{
    The `go.append_slice` operation models `append(dst, src...)` before slice
    growth and bulk-copy semantics are lowered to a concrete runtime and memory
    contract.
  }];

  let arguments = (ins Go_SliceLike:$dst, Go_SliceLike:$src);
  let results = (outs Go_SliceLike:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$dst `,` $src attr-dict `:` functional-type(operands, results)";
}

def Go_FieldAddrOp : Go_Op<"field_addr", [Pure]> {
  let summary = "materialize the address of a Go field";
  let description = [{
    The `go.field_addr` operation isolates field-address formation from the
    frontend. It lets the frontend stop faking selector reads and writes with
    typed helper calls, while still postponing concrete runtime layout
    decisions to later lowering.
  }];

  let arguments = (ins Go_FieldBaseLike:$base, StrAttr:$field);
  let results = (outs Go_PointerLike:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$base `,` $field attr-dict `:` type($base) `->` type($result)";
}

def Go_LoadOp : Go_Op<"load"> {
  let summary = "load a Go value from a Go pointer";
  let description = [{
    The `go.load` operation models dereference and field reads before the Go
    frontend has committed to a concrete LLVM-level memory layout.
  }];

  let arguments = (ins Go_PointerLike:$addr);
  let results = (outs AnyType:$result);
  let hasVerifier = 1;
  let assemblyFormat = "$addr attr-dict `:` type($addr) `->` type($result)";
}

def Go_StoreOp : Go_Op<"store"> {
  let summary = "store a Go value through a Go pointer";
  let description = [{
    The `go.store` operation models pointer writes and selector writes before
    concrete runtime layout is lowered away.
  }];

  let arguments = (ins AnyType:$value, Go_PointerLike:$addr);
  let hasVerifier = 1;
  let assemblyFormat = "$value `,` $addr attr-dict `:` type($value) `to` type($addr)";
}

def Go_TodoOp : Go_Op<"todo"> {
  let summary = "temporary frontend placeholder statement";
  let description = [{
    The `go.todo` operation marks a frontend statement that has not yet been
    lowered to stable GoIR. It is intended only as a migration aid while the
    typed frontend moves from the old text prototype to the formal dialect.
  }];

  let arguments = (ins StrAttr:$reason);
  let assemblyFormat = "$reason attr-dict";
}

def Go_TodoValueOp : Go_Op<"todo_value", [Pure]> {
  let summary = "temporary frontend placeholder value";
  let description = [{
    The `go.todo_value` operation produces a typed placeholder value so that
    partially lowered frontend output remains parseable and testable.
  }];

  let arguments = (ins StrAttr:$reason);
  let results = (outs AnyType:$result);
  let assemblyFormat = "$reason attr-dict `:` type($result)";
}

#endif // MLSE_GO_IR_GOOPS

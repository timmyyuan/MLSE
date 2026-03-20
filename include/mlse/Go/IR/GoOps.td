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

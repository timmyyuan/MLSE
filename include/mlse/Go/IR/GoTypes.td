#ifndef MLSE_GO_IR_GOTYPES
#define MLSE_GO_IR_GOTYPES

include "mlse/Go/IR/GoBase.td"

def Go_StringType : Go_Type<"String", "string"> {
  let summary = "Go string value";
}

def Go_ErrorType : Go_Type<"Error", "error"> {
  let summary = "Go error interface value";
}

def Go_NamedType : Go_Type<"Named", "named"> {
  let summary = "named Go type";
  let description = [{
    Represents a nominal Go type name carried through the frontend IR, such as
    `context.Context` or `mypkg.Value`.
  }];
  let parameters = (ins StringRefParameter<"fully-qualified Go type name">:$name);
  let assemblyFormat = "`<` $name `>`";
}

def Go_PointerType : Go_Type<"Pointer", "ptr"> {
  let summary = "Go pointer type";
  let parameters = (ins "Type":$pointee);
  let builders = [
    TypeBuilderWithInferredContext<(ins "Type":$pointee), [{
      return $_get(pointee.getContext(), pointee);
    }]>
  ];
  let assemblyFormat = "`<` qualified($pointee) `>`";
}

def Go_SliceType : Go_Type<"Slice", "slice"> {
  let summary = "Go slice type";
  let parameters = (ins "Type":$elementType);
  let builders = [
    TypeBuilderWithInferredContext<(ins "Type":$elementType), [{
      return $_get(elementType.getContext(), elementType);
    }]>
  ];
  let assemblyFormat = "`<` qualified($elementType) `>`";
}

#endif // MLSE_GO_IR_GOTYPES

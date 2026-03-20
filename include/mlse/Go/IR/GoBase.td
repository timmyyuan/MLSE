#ifndef MLSE_GO_IR_GOBASE
#define MLSE_GO_IR_GOBASE

include "mlir/IR/AttrTypeBase.td"
include "mlir/IR/OpBase.td"

def Go_Dialect : Dialect {
  let name = "go";
  let cppNamespace = "::mlir::go";
  let summary = "Go frontend dialect";
  let description = [{
    The `go` dialect is the long-lived, verifier-backed frontend IR entry for Go
    in MLSE. The intent is to keep Go-specific type information in this dialect
    while using standard MLIR dialects such as `func`, `arith`, `scf` and `cf`
    for most structural operations and control flow.
  }];
  let useDefaultTypePrinterParser = 1;
}

class Go_Type<string name, string typeMnemonic, list<Trait> traits = []>
    : TypeDef<Go_Dialect, name, traits> {
  let mnemonic = typeMnemonic;
}

class Go_Op<string mnemonic, list<Trait> traits = []>
    : Op<Go_Dialect, mnemonic, traits>;

#endif // MLSE_GO_IR_GOBASE

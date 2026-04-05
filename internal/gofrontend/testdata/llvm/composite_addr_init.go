// MLSE-COMPILE: formal
// LLVM-LABEL: define ptr @demo.makeResp
// LLVM: call ptr @runtime.newobject(i64 8, i64 8)
// LLVM: store i64
// LLVM-NOT: @runtime.field.addr.Version
package demo

type Resp struct {
	Version int
}

func makeResp(version int) *Resp {
	return &Resp{Version: version}
}

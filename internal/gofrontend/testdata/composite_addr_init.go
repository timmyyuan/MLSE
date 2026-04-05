// CHECK-LABEL: func.func @demo.makeResp(%version: i64) -> !go.ptr<!go.named<"Resp">>
// CHECK: %{{[A-Za-z0-9_%.]+}} = arith.constant 8 : i64
// CHECK: %{{[A-Za-z0-9_%.]+}} = arith.constant 8 : i64
// CHECK: func.call @runtime.newobject(%{{[A-Za-z0-9_%.]+}}, %{{[A-Za-z0-9_%.]+}}) : (i64, i64) -> !go.ptr<!go.named<"Resp">>
// CHECK: go.field_addr %{{[A-Za-z0-9_%.]+}}, "Version" {offset = 0 : i64} : !go.ptr<!go.named<"Resp">> -> !go.ptr<i64>
// CHECK: go.store %version, %{{[A-Za-z0-9_%.]+}} : i64 to !go.ptr<i64>
// CHECK-NOT: @runtime.new.
package demo

type Resp struct {
	Version int
}

func makeResp(version int) *Resp {
	return &Resp{Version: version}
}

// CHECK-LABEL: func.func @demo.readX(%h: !go.ptr<!go.named<"Holder">>) -> i64
// CHECK: %field{{[0-9]+}} = go.field_addr %h, "X" {offset = 8 : i64} : !go.ptr<!go.named<"Holder">> -> !go.ptr<i64>
// CHECK: %load{{[0-9]+}} = go.load %field{{[0-9]+}} : !go.ptr<i64> -> i64
package demo

type Holder struct {
	Ready bool
	X     int
}

func readX(h *Holder) int {
	return h.X
}

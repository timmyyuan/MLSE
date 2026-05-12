// CHECK-LABEL: func.func @testdata.F(
// CHECK: func.call @runtime.newobject
// CHECK: func.call @runtime.any.box.ptr.pair(
// CHECK-SAME: (!go.ptr<!go.named<"pair">>) -> !go.named<"any">
// CHECK: func.call @testdata.sink(%{{[^)]*}}) : (!go.named<"any">) -> ()
// CHECK-NOT: func.call @testdata.sink(%{{[^)]*}}) : (!go.ptr<!go.named<"pair">>) -> ()
package testdata

func sink(_ any) {}

type pair struct {
	x any
	y any
}

func F(a, b string) {
	p := &pair{}
	p.x = a
	p.y = b
	sink(p)
}

// CHECK-LABEL: func.func @testdata.F(
// CHECK: %any{{[0-9]+}} = func.call @runtime.any.box.string
// CHECK: func.call @testdata.sink(%{{[^,]+}}, %any{{[0-9]+}}) : (!go.string, !go.named<"any">) -> ()
// CHECK-NOT: func.call @testdata.sink(%{{[^,]+}}, %{{[^)]+}}) : (!go.string, !go.string) -> ()
// CHECK-LABEL: func.func @testdata.sink(
// CHECK-SAME: %fallback: !go.named<"any">
package testdata

func sink(_ string, fallback any) {}

func F() {
	sink("key", "default")
}

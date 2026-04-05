// CHECK-LABEL: func.func @demo.isNil(%v: !go.named<"Result">) -> i1
// CHECK: func.call @runtime.zero.Result() : () -> !go.named<"Result">
// CHECK-NOT: go.nil : !go.named<"Result">
package demo

type Result struct{}

func isNil(v Result) bool {
	return v == nil
}

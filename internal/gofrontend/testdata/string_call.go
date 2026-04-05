// CHECK-LABEL: func.func @demo.greet(%name: !go.string) -> !go.string
// CHECK: go.string_constant "hi %s" : !go.string
// CHECK: go.make_slice
// CHECK: func.call @runtime.any.box.string(
// CHECK: func.call @runtime.fmt.Sprintf(
// CHECK: func.func private @runtime.fmt.Sprintf(!go.string, !go.slice<!go.named<"any">>) -> !go.string
package demo

import "fmt"

func greet(name string) string {
	return fmt.Sprintf("hi %s", name)
}

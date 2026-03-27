// CHECK-LABEL: func.func @demo.greet(%name: !go.string) -> !go.string
// CHECK: go.string_constant "hi %s" : !go.string
// CHECK: func.call @fmt.Sprintf(
// CHECK: func.func private @fmt.Sprintf(
package demo

import "fmt"

func greet(name string) string {
	return fmt.Sprintf("hi %s", name)
}

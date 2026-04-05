// CHECK-LABEL: func.func @demo.contains(%text: !go.string) -> i1
// CHECK: func.call @runtime.strings.Contains(
// CHECK-SAME: (!go.string, !go.string) -> i1
// CHECK-LABEL: func.func @demo.failure(%name: !go.string) -> !go.error
// CHECK: func.call @runtime.any.box.string(
// CHECK: func.call @runtime.fmt.Errorf(
// CHECK-SAME: (!go.string, !go.slice<!go.named<"any">>) -> !go.error
// CHECK-LABEL: func.func @demo.format(%name: !go.string) -> !go.string
// CHECK: func.call @runtime.any.box.string(
// CHECK: func.call @runtime.fmt.Sprintf(
// CHECK-SAME: (!go.string, !go.slice<!go.named<"any">>) -> !go.string
// CHECK-LABEL: func.func @demo.replace(%text: !go.string) -> !go.string
// CHECK: func.call @runtime.strings.ReplaceAll(
// CHECK-SAME: (!go.string, !go.string, !go.string) -> !go.string
// CHECK-LABEL: func.func @demo.split(%text: !go.string) -> !go.slice<!go.string>
// CHECK: func.call @runtime.strings.Split(
// CHECK-SAME: (!go.string, !go.string) -> !go.slice<!go.string>
// CHECK-LABEL: func.func @demo.wrap(%text: !go.string) -> !go.error
// CHECK: func.call @runtime.errors.New(
// CHECK-SAME: (!go.string) -> !go.error
package demo

import (
	e "errors"
	f "fmt"
	s "strings"
)

func contains(text string) bool {
	return s.Contains(text, "x")
}

func failure(name string) error {
	return f.Errorf("bad %s", name)
}

func format(name string) string {
	return f.Sprintf("hi %s", name)
}

func split(text string) []string {
	return s.Split(text, ",")
}

func replace(text string) string {
	return s.ReplaceAll(text, "x", "y")
}

func wrap(text string) error {
	return e.New(text)
}

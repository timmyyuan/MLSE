// CHECK-LABEL: func.func @demo.ptr.StringSet.Len(%s: !go.ptr<!go.named<"StringSet">>) -> i32
// CHECK: %nil{{[0-9]+}} = go.nil : !go.ptr<!go.named<"StringSet">>
// CHECK: %cmp{{[0-9]+}} = go.eq %s, %nil{{[0-9]+}} : (!go.ptr<!go.named<"StringSet">>, !go.ptr<!go.named<"StringSet">>) -> i1
package demo

type StringSet struct{}

func (s *StringSet) Len() int {
	if s == nil {
		return 0
	}
	return 1
}

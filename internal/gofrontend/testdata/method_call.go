// CHECK-LABEL: func.func @demo.size(%s: !go.ptr<!go.named<"StringSet">>) -> i32
// CHECK: func.call @demo.ptr.StringSet.Len
// CHECK-NOT: go.todo_value "indirect_call"
package demo

type StringSet struct{}

func (s *StringSet) Len() int {
	return 1
}

func size(s *StringSet) int {
	return s.Len()
}

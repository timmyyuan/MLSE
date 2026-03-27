// MLSE-COMPILE: formal
// CHECK-LABEL: func.func @demo.collect(
// CHECK-SAME: %xs: !go.slice<!go.ptr<!go.named<"Item">>>
// CHECK: = go.len %xs : !go.slice<!go.ptr<!go.named<"Item">>> -> i32
// CHECK: = go.elem_addr %xs, %{{[A-Za-z0-9_%.]+}} : (!go.slice<!go.ptr<!go.named<"Item">>>, index) -> !go.ptr<!go.ptr<!go.named<"Item">>>
// CHECK: = go.load %{{[A-Za-z0-9_%.]+}} : !go.ptr<!go.ptr<!go.named<"Item">>> -> !go.ptr<!go.named<"Item">>
// CHECK: = func.call @demo.ptr.Item.ToView
// CHECK: = go.append
// CHECK-NOT: __mlse_range_len
// CHECK-NOT: __mlse_index
// CHECK-NOT: @demo.append
package demo

type Item struct{}

type ItemList []*Item

type View struct{}

func (it *Item) ToView() *View {
	return &View{}
}

func collect(xs ItemList) []*View {
	var out []*View
	for _, x := range xs {
		out = append(out, x.ToView())
	}
	return out
}

// CHECK-LABEL: func.func @demo.hasOwner(%ctx: !go.ptr<!go.named<"Context">>) -> i1
// CHECK: %field{{[0-9]+}} = go.field_addr %ctx, "Owner" : !go.ptr<!go.named<"Context">> -> !go.ptr<!go.string>
// CHECK: %load{{[0-9]+}} = go.load %field{{[0-9]+}} : !go.ptr<!go.string> -> !go.string
// CHECK: %str{{[0-9]+}} = go.string_constant "" : !go.string
// CHECK: %cmp{{[0-9]+}} = go.neq %load{{[0-9]+}}, %str{{[0-9]+}} : (!go.string, !go.string) -> i1
// CHECK-NOT: go.todo_value "SelectorExpr"
package demo

type Context struct{}

func hasOwner(ctx *Context) bool {
	return ctx.Owner != ""
}

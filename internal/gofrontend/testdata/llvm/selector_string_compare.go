// MLSE-COMPILE: formal
// LLVM-LABEL: define i1 @demo.hasOwner(ptr %0)
// LLVM: call i1 @runtime.neq.string
package demo

type Context struct{}

func hasOwner(ctx *Context) bool {
	return ctx.Owner != ""
}

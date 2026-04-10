package demo

// CHECK-LABEL: func.func @demo.extendTag(%tag: !go.string, %name: !go.string) -> !go.string
// CHECK: %cmp{{[0-9]+}} = go.neq %tag, %str{{[0-9]+}} : (!go.string, !go.string) -> i1
// CHECK: %add{{[0-9]+}} = func.call @runtime.add.string(%tag, %str{{[0-9]+}}) : (!go.string, !go.string) -> !go.string
// CHECK: %add{{[0-9]+}} = func.call @runtime.add.string(%str{{[0-9]+}}, %name) : (!go.string, !go.string) -> !go.string
// CHECK: %add{{[0-9]+}} = func.call @runtime.add.string(%add{{[0-9]+}}, %add{{[0-9]+}}) : (!go.string, !go.string) -> !go.string
// CHECK-NOT: go.todo "compound_assign"

func extendTag(tag string, name string) string {
	if tag != "" {
		tag += "$$"
	}
	tag += "_podname||" + name
	return tag
}

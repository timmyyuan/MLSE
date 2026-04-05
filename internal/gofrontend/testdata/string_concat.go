package testdata

// CHECK-NOT: go.todo_value "binary__"
// CHECK: func.call @runtime.add.string

func join(name string) string {
	return "/repo/" + name
}

package testdata

// CHECK-NOT: go.todo_value "binary__"
// CHECK: func.call @__mlse_add__go.string

func join(name string) string {
	return "/repo/" + name
}

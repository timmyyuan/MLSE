package testdata

// CHECK-NOT: go.todo "assign_target"
// CHECK: func.call @__mlse_store_index
// CHECK: go.elem_addr %xs, %{{[A-Za-z0-9_%.]+}} : (!go.slice<i32>, i32) -> !go.ptr<i32>
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i32 to !go.ptr<i32>
// CHECK: go.field_addr %h, "X"
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i32 to !go.ptr<i32>
// CHECK: func.call @defaultValue() : () -> !go.ptr<i32>
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i32 to !go.ptr<i32>
// CHECK-NOT: __mlse_store_selector_X
// CHECK-NOT: __mlse_store_deref

type holder struct {
	X int
}

var defaultValue = new(int)

func update(m map[string]int, xs []int, key string, value int, h *holder) *holder {
	_ = key
	_ = value
	m[key] = value
	xs[0] = value
	h.X = value
	*defaultValue = value
	return h
}

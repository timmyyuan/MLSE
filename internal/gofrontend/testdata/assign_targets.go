package testdata

// CHECK-NOT: go.todo "assign_target"
// CHECK: func.call @runtime.store.index.
// CHECK: go.elem_addr %xs, %{{[A-Za-z0-9_%.]+}} : (!go.slice<i64>, i64) -> !go.ptr<i64>
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i64 to !go.ptr<i64>
// CHECK: go.field_addr %h, "X"
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i64 to !go.ptr<i64>
// CHECK: func.call @defaultValue() : () -> !go.ptr<i64>
// CHECK: go.store %value, %{{[A-Za-z0-9_%.]+}} : i64 to !go.ptr<i64>
// CHECK-NOT: func.call @runtime.store.selector.
// CHECK-NOT: func.call @runtime.store.deref.

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

package testdata

// CHECK-NOT: go.todo "IfStmt_init"
// CHECK: scf.if

func withInit(x int) int {
	if y := x + 1; y > 0 {
		x = y
	}
	return x
}

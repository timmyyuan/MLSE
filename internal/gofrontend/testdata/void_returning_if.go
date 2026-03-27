package testdata

// CHECK-NOT: go.todo "IfStmt_returning_region"
// CHECK: scf.if

func sink(s string)

func guard(x string) {
	if x == "" {
		sink(x)
		return
	}
	sink(x)
}

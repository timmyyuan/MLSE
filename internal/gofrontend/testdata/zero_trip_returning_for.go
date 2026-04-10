// CHECK-LABEL: func.func @demo.zero_trip_returning_for() -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK: return %[[C:.+]] : i64
package demo

func zero_trip_returning_for() int {
	for i := 0; i == 2; i++ {
		return i
	}
	return 7
}

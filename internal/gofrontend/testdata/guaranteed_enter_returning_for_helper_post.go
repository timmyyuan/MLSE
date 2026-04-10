// CHECK-LABEL: func.func @demo.guaranteed_enter_returning_for_helper_post() -> i64
// CHECK-NOT: go.todo "ForStmt"
// CHECK-NOT: func.call @demo.add1
// CHECK: return %[[C:.+]] : i64
package demo

func add1(x int) int {
	return x + 1
}

func guaranteed_enter_returning_for_helper_post() int {
	for i := 0; i < 2; i = add1(i) {
		return i
	}
	return 9
}

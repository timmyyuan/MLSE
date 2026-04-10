// CHECK-LABEL: func.func @demo.castInclusive(%n: i8) -> i8
// CHECK: scf.for
// CHECK-NOT: go.todo "ForStmt"
// CHECK-LABEL: func.func @demo.castReturning(%n: i8) -> i8
// CHECK: arith.constant 0 : i8
// CHECK: return %[[C:.+]] : i8
// CHECK-NOT: scf.for
// CHECK-NOT: go.todo "ForStmt"
package demo

func castInclusive(n int8) int8 {
	i := int8(0)
	for i = 0; int(i) <= int(n); i += 1 {
	}
	return i
}

func castReturning(n int8) int8 {
	for n = 0; int(n) <= 3; n += 1 {
		return n
	}
	return n
}

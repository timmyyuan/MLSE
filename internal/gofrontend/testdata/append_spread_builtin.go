// MLSE-COMPILE: formal
// CHECK: %[[NIL:[A-Za-z0-9_%.]+]] = go.nil : !go.slice<i64>
// CHECK: %[[APP:[A-Za-z0-9_%.]+]] = go.append_slice %[[NIL]], %xs : (!go.slice<i64>, !go.slice<i64>) -> !go.slice<i64>
package demo

func spread(xs []int) []int {
	var out []int
	return append(out, xs...)
}

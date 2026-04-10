// CHECK-LABEL: func.func @demo.read16(%p: !go.ptr<i64>) -> i16
// CHECK: %load{{[0-9]+}} = go.load %p : !go.ptr<i64> -> i64
// CHECK: %conv{{[0-9]+}} = arith.trunci %load{{[0-9]+}} : i64 to i16
package demo

func read16(p *int64) int16 {
	return int16(*p)
}

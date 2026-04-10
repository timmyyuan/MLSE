// CHECK-LABEL: func.func @demo.widen() -> i64
// CHECK: %funclit{{[0-9]+}} = func.constant @demo.widen.__lit0 : () -> i8
// CHECK: %call{{[0-9]+}} = func.call_indirect %funclit{{[0-9]+}}() : () -> i8
// CHECK: %conv{{[0-9]+}} = arith.extsi %call{{[0-9]+}} : i8 to i64
package demo

func widen() int64 {
	fn := func() int8 {
		return 1
	}
	return int64(fn())
}

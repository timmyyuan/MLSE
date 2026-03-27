// CHECK-LABEL: func.func @demo.isNil(%err: !go.error) -> i1
// CHECK: %nil{{[0-9]+}} = go.nil : !go.error
// CHECK: %cmp{{[0-9]+}} = go.eq %err, %nil{{[0-9]+}} : (!go.error, !go.error) -> i1
package demo

func isNil(err error) bool {
	return err == nil
}

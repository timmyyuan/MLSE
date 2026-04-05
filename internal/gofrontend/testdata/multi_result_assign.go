// CHECK-LABEL: func.func @demo.use() -> i1
// CHECK: %call{{[0-9]+}}:2 = func.call @demo.lookup() : () -> (!go.ptr<!go.named<"Conf">>, !go.error)
// CHECK: %nil{{[0-9]+}} = go.nil : !go.error
// CHECK: %cmp{{[0-9]+}} = go.neq %call{{[0-9]+}}#1, %nil{{[0-9]+}} : (!go.error, !go.error) -> i1
// CHECK: %nil{{[0-9]+}} = go.nil : !go.ptr<!go.named<"Conf">>
// CHECK: %cmp{{[0-9]+}} = go.eq %call{{[0-9]+}}#0, %nil{{[0-9]+}} : (!go.ptr<!go.named<"Conf">>, !go.ptr<!go.named<"Conf">>) -> i1
package demo

type Conf struct{}

func lookup() (*Conf, error) {
	return nil, nil
}

func use() bool {
	cfg, err := lookup()
	if err != nil {
		return false
	}
	return cfg == nil
}

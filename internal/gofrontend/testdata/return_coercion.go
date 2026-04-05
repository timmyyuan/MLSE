// CHECK-LABEL: func.func @demo.lookup(%cfg: !go.ptr<!go.named<"Conf">>, %key: !go.string) -> (!go.string, i1)
// CHECK-NOT: go.todo_value "return_type_mismatch"
// CHECK: func.call @runtime.convert.{{(value|type)}}.to.string
package demo

type Conf struct {
	Envs map[string]string
}

func lookup(cfg *Conf, key string) (string, bool) {
	if cfg == nil {
		return "", false
	}
	v, ok := cfg.Envs[key]
	if ok {
		return v, true
	}
	return "", false
}

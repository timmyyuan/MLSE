// MLSE-COMPILE: formal
// LLVM-LABEL: define i1 @demo.use()
// LLVM: call { ptr, ptr } @demo.lookup()
// LLVM: icmp ne ptr %{{[0-9]+}}, null
// LLVM: icmp eq ptr %{{[0-9]+}}, null
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

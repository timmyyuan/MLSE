package diffcase

// sink models a side-effecting external call: no return values.
func sink(_ any) {}

type pair struct {
	x any
	y any
}

func F(a, b string) {
	p := &pair{}
	p.x = a
	p.y = b
	// Passing the local alloc itself can surface alloc-based roots.
	sink(p)
}

func foo2(a, b string) {
	// Semantically identical to foo1, but the SMT encoding may still
	// compare the varargs alloc address and return sat.
	p := &pair{}
	p.x = a
	p.y = b
	sink(p)
}

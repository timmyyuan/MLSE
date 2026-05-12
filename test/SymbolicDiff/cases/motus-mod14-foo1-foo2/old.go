package diffcase

var GlobalInput int

func F(x int) int {
	return x + GlobalInput
}

func foo2(x int) int {
	return x + GlobalInput
}

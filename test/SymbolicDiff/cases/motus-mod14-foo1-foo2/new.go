package diffcase

var GlobalInput int

func foo1(x int) int {
	return x + GlobalInput
}

func F(x int) int {
	return x + GlobalInput
}

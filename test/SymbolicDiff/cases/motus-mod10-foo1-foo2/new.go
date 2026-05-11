package diffcase

func foo1(pl []int) []int {
	var res []int
	res = append(res, pl...)
	return res
}

func F(pl []int) []int {
	res := make([]int, 0, len(pl))
	res = append(res, pl...)
	return res
}

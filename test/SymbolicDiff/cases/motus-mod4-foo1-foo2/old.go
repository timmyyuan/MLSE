package diffcase

func F(pl []int) []int {
	var res []int
	res = append(res, pl...)
	return res
}

func foo2(pl []int) []int {
	var res []int
	res = append(res, pl...)
	return res
}

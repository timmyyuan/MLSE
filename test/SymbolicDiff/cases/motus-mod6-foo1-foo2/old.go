package diffcase

func F(pl []int) []int {
	var res []int
	res = append(res, pl...)
	return res
}

func foo2(pl []int) []int {
	var res []int
	if len(pl) > 0 {
		res = make([]int, 0, len(pl))
	}
	res = append(res, pl...)
	return res
}

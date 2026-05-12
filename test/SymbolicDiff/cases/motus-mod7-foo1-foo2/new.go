package diffcase

func foo1(pl []int) []int {
	var res []int
	for _, pb := range pl {
		res = append(res, pb)
	}
	return res
}

func F(pl []int) []int {
	var res []int
	if len(pl) > 0 {
		res = make([]int, 0, len(pl))
	}
	for _, pb := range pl {
		res = append(res, pb)
	}
	return res
}

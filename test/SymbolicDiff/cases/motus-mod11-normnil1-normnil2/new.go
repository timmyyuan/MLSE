package diffcase

func NormNil1(r []string) []string {
	if len(r) == 0 {
		r = nil
	}
	return r
}

func F(r []string) []string {
	if r == nil || len(r) == 0 {
		r = nil
	}
	return r
}

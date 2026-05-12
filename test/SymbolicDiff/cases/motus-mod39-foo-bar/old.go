package diffcase

import "math"

type Resp struct {
	Version int
}

func F(flag bool) *Resp {
	version := 0
	if flag {
		version = math.MaxInt32
	} else {
		version = 7
	}
	return &Resp{Version: version}
}

func Bar(flag bool) *Resp {
	version := 0
	if flag {
		version = math.MaxInt
	} else {
		version = 7
	}
	return &Resp{Version: version}
}

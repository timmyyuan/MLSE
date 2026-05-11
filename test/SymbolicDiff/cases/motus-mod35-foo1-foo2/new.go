package diffcase

type SceneResp struct {
	SceneInfos []int
}

type CommonResp struct {
	Code int
}

func success(_ *SceneResp) *CommonResp {
	return &CommonResp{Code: 0}
}

func Foo1(flag bool) *CommonResp {
	resp := &SceneResp{}
	if flag {
		resp.SceneInfos = nil
	} else {
		resp.SceneInfos = []int{}
	}
	return success(resp)
}

func F(flag bool) *CommonResp {
	resp := &SceneResp{}
	resp.SceneInfos = []int{}
	return success(resp)
}

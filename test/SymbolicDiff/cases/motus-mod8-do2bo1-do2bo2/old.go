package diffcase

type PlaybookList []*Playbook

type Playbook struct{}

func (pl *Playbook) Do2Bo() *PlaybookPms {
	return &PlaybookPms{}
}

type PlaybookPms struct{}

func (pl PlaybookList) Do2Bo1() []*PlaybookPms {
	var res []*PlaybookPms
	for _, pb := range pl {
		res = append(res, pb.Do2Bo())
	}
	return res
}

func (pl PlaybookList) Do2Bo2() []*PlaybookPms {
	var res []*PlaybookPms
	if len(pl) > 0 {
		res = make([]*PlaybookPms, 0, len(pl))
	}
	for _, pb := range pl {
		res = append(res, pb.Do2Bo())
	}
	return res
}

func F(pl PlaybookList) []*PlaybookPms {
	return pl.Do2Bo1()
}

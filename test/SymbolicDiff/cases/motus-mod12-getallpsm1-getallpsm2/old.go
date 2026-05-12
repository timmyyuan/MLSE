package diffcase

import (
	"context"

	"example.com/smtcmpmod12/dal"
	"example.com/smtcmpmod12/logs"
	"example.com/smtcmpmod12/sets"
)

func F(ctx context.Context, pluginName string, filterPsmArr []string) []string {
	keyStat, err := dal.GetKeyStatInfo(ctx, dal.GetCDSConfKey(pluginName))
	if err != nil {
		logs.CtxError(ctx, "[GetAllPsm] get key stat info error: %+v pluginName=%s", err, pluginName)
		return []string{}
	}

	var r []string
	psmSet := sets.NewStringSetFromSlice(filterPsmArr)
	for _, psm := range keyStat.PSM {
		if psmSet.Len() > 0 && !psmSet.Contains(psm) {
			continue
		}
		r = append(r, psm)
	}
	return r
}

func GetAllPsm2(ctx context.Context, pluginName string, filterPsmArr []string) []string {
	keyStat, err := dal.GetKeyStatInfo(ctx, dal.GetCDSConfKey(pluginName))
	if err != nil {
		logs.CtxError(ctx, "[GetAllPsm] get key stat info error: %+v pluginName=%s", err, pluginName)
		return []string{}
	}

	var r []string
	if n := len(keyStat.PSM); n > 0 {
		r = make([]string, 0, n)
	}
	psmSet := sets.NewStringSetFromSlice(filterPsmArr)
	for _, psm := range keyStat.PSM {
		if psmSet.Len() > 0 && !psmSet.Contains(psm) {
			continue
		}
		r = append(r, psm)
	}
	if len(r) == 0 {
		r = nil
	}
	return r
}

package diffcase

import (
	"context"
	"strings"
)

type Region string

type GetPSMRelatedPluginListReq struct {
	Psm         string
	VRegionList string
}

var (
	VRegionMap = map[string]Region{}
)

func F(ctx context.Context, req *GetPSMRelatedPluginListReq) int64 {
	pluginByVregion := make(map[string]map[string]bool)
	vregions := strings.Split(req.VRegionList, ",")
	total := int64(0)

	for _, vregion := range vregions {
		legoRegion := VRegionMap[vregion]
		_ = legoRegion
		key := req.Psm + "/plugin_conf/" + vregion
		if !strings.Contains(key, "/plugin_conf/") {
			continue
		}

		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			continue
		}
		pluginName := parts[2]

		if _, ok := pluginByVregion[vregion]; !ok {
			pluginByVregion[vregion] = make(map[string]bool)
		}
		if !pluginByVregion[vregion][pluginName] {
			pluginByVregion[vregion][pluginName] = true
			total++
		}
	}

	return total
}

func GetPsmRelatedPluginList2(ctx context.Context, req *GetPSMRelatedPluginListReq) int64 {
	pluginByVregion := make(map[string]map[string]bool)
	vregions := strings.Split(req.VRegionList, ",")
	total := int64(0)

	for _, vregion := range vregions {
		legoRegion := VRegionMap[vregion]
		_ = legoRegion
		key := req.Psm + "/plugin_conf/" + vregion
		if !strings.Contains(key, "/plugin_conf/") {
			continue
		}

		parts := strings.Split(key, "/")
		if len(parts) != 3 {
			continue
		}
		pluginName := parts[2]

		if _, ok := pluginByVregion[vregion]; !ok {
			pluginByVregion[vregion] = make(map[string]bool)
		}
		if !pluginByVregion[vregion][pluginName] {
			pluginByVregion[vregion][pluginName] = true
			total++
		}
	}

	return total
}

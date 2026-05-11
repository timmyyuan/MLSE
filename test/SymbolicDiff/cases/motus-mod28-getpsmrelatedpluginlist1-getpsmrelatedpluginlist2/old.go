package diffcase

import "context"

type Region string

type GetPSMRelatedPluginListReq struct {
	Psm        string
	VRegions   []string
	PluginName string
}

var (
	VRegionMap = map[string]Region{}
)

func F(ctx context.Context, req *GetPSMRelatedPluginListReq) int64 {
	pluginByVregion := make(map[string]map[string]bool)
	total := int64(0)

	for _, vregion := range req.VRegions {
		legoRegion := VRegionMap[vregion]
		_ = legoRegion
		key := req.Psm + "/plugin_conf/" + vregion
		if len(key) < 0 {
			continue
		}
		pluginName := req.PluginName

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
	total := int64(0)

	for _, vregion := range req.VRegions {
		legoRegion := VRegionMap[vregion]
		_ = legoRegion
		key := req.Psm + "/plugin_conf/" + vregion
		if len(key) < 0 {
			continue
		}
		pluginName := req.PluginName

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

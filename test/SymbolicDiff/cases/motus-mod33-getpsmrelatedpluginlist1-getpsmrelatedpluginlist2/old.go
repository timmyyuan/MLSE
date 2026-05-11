package diffcase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Region int

var VRegionMap = map[string]Region{
	"v1": 1,
	"v2": 2,
}

type GetPSMRelatedPluginListReq struct {
	Psm         string
	VRegionList string
}

func GetCDSApiClientByRegionV2(region Region) *Client {
	return &Client{}
}

type GetPSMRelatedPluginListRespData struct {
	Items []*GetPSMRelatedPluginListRespDataItem
}

type GetPSMRelatedPluginListRespDataItem struct {
	VRegion        string
	PluginNameList []string
}

type GetPsmRelatedKeyReq struct {
	Psm         string
	VRegionList []string
	Namespace   string
}

type KeyItem struct {
	Key          string
	VRegion      string
	DispatchTime time.Time
}

type GetPsmRelatedKeyResp struct {
	KeyList []KeyItem
}

type Client struct{}

func (c *Client) Context(ctx context.Context) *Client {
	return c
}

func GetApmConf(ctx context.Context) *ApmConf {
	return &ApmConf{
		PsmRelatedPluginListDispatchTimeThreshold: 1,
	}
}

type ApmConf struct {
	PsmRelatedPluginListDispatchTimeThreshold int64
}

func (c *Client) GetPsmRelatedKey(req GetPsmRelatedKeyReq) (*GetPsmRelatedKeyResp, error) {
	key1 := req.Psm + "/plugin_conf/demo"
	key2 := req.Psm + "/other_conf/demo"
	return &GetPsmRelatedKeyResp{
		KeyList: []KeyItem{
			{Key: key1, VRegion: "v1", DispatchTime: time.Now()},
			{Key: key2, VRegion: "v1", DispatchTime: time.Now()},
		},
	}, nil
}

var Namespace = "ns"

func CtxWarn(ctx context.Context, format string, args ...any)  {}
func CtxError(ctx context.Context, format string, args ...any) {}

func F(ctx context.Context, req *GetPSMRelatedPluginListReq) (*GetPSMRelatedPluginListRespData, int64, error) {
	pluginByVregion := make(map[string]map[string]struct{})
	vregions := strings.Split(req.VRegionList, ",")

	legoRegion2ClientMap := make(map[Region]*Client)
	legoRegion2Vregions := make(map[Region][]string)

	for _, vregion := range vregions {
		legoRegion, ok := VRegionMap[vregion]
		if !ok {
			CtxWarn(ctx, "[GetPsmRelatedPluginList] unsupported vregion: %s", vregion)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] unsupported vregion: %s", vregion)
		}
		legoRegion2Vregions[legoRegion] = append(legoRegion2Vregions[legoRegion], vregion)
		// 如果已经处理过这个legoRegion，直接跳过
		if _, exists := legoRegion2ClientMap[legoRegion]; exists {
			continue
		}

		client := GetCDSApiClientByRegionV2(legoRegion)
		if client == nil {
			CtxError(ctx, "[GetPsmRelatedPluginList] cds client not initialized for vregion: %s", vregion)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] cds client not initialized for vregion: %s", vregion)
		}

		legoRegion2ClientMap[legoRegion] = client
	}

	for legoRegion, vregionList := range legoRegion2Vregions {
		client := legoRegion2ClientMap[legoRegion]
		if client == nil {
			return nil, 0, errors.New("[GetPsmRelatedPluginList] get client from legoRegion2ClientMap failed")
		}
		openapiReq := GetPsmRelatedKeyReq{
			Psm:         req.Psm,
			VRegionList: vregionList,
			Namespace:   Namespace,
		}

		openapiResp, err := client.Context(ctx).GetPsmRelatedKey(openapiReq)
		if err != nil {
			CtxWarn(ctx, "[GetPsmRelatedPluginList] failed to get psm related key for vregions %v, err: %v", vregionList, err)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] failed to get psm related key for vregions %v, err: %v", vregionList, err)
		}

		for _, item := range openapiResp.KeyList {
			if !strings.Contains(item.Key, "/plugin_conf/") {
				continue
			}

			psmRelatedPluginListDispatchTimeThreshold := time.Duration(GetApmConf(ctx).PsmRelatedPluginListDispatchTimeThreshold) * 24 * time.Hour
			if time.Since(item.DispatchTime) > psmRelatedPluginListDispatchTimeThreshold {
				continue
			}

			parts := strings.Split(item.Key, "/")
			if len(parts) != 3 {
				continue
			}
			pluginName := parts[2]

			if _, ok := pluginByVregion[item.VRegion]; !ok {
				pluginByVregion[item.VRegion] = make(map[string]struct{})
			}
			pluginByVregion[item.VRegion][pluginName] = struct{}{}
		}
	}

	return nil, 0, nil
}

func GetPsmRelatedPluginList2(ctx context.Context, req *GetPSMRelatedPluginListReq) (*GetPSMRelatedPluginListRespData, int64, error) {
	pluginByVregion := make(map[string]map[string]struct{})
	vregions := strings.Split(req.VRegionList, ",")
	legoRegion2ClientMap := make(map[Region]*Client)
	legoRegion2Vregions := make(map[Region][]string)

	for _, vregion := range vregions {
		legoRegion, ok := VRegionMap[vregion]
		if !ok {
			CtxWarn(ctx, "[GetPsmRelatedPluginList] unsupported vregion: %s", vregion)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] unsupported vregion: %s", vregion)
		}
		legoRegion2Vregions[legoRegion] = append(legoRegion2Vregions[legoRegion], vregion)
		// 如果已经处理过这个legoRegion，直接跳过
		if _, exists := legoRegion2ClientMap[legoRegion]; exists {
			continue
		}

		client := GetCDSApiClientByRegionV2(legoRegion)
		if client == nil {
			CtxError(ctx, "[GetPsmRelatedPluginList] cds client not initialized for vregion: %s", vregion)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] cds client not initialized for vregion: %s", vregion)
		}

		legoRegion2ClientMap[legoRegion] = client
	}

	for legoRegion, vregionList := range legoRegion2Vregions {
		client := legoRegion2ClientMap[legoRegion]
		if client == nil {
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] get client from legoRegion2ClientMap failed")
		}
		openapiReq := GetPsmRelatedKeyReq{
			Psm:         req.Psm,
			VRegionList: vregionList,
			Namespace:   Namespace,
		}

		openapiResp, err := client.Context(ctx).GetPsmRelatedKey(openapiReq)
		if err != nil {
			CtxWarn(ctx, "[GetPsmRelatedPluginList] failed to get psm related key for vregions %v, err: %v", vregionList, err)
			return nil, 0, fmt.Errorf("[GetPsmRelatedPluginList] failed to get psm related key for vregions %v, err: %v", vregionList, err)
		}

		for _, item := range openapiResp.KeyList {
			if !strings.Contains(item.Key, "/plugin_conf/") {
				continue
			}

			psmRelatedPluginListDispatchTimeThreshold := time.Duration(GetApmConf(ctx).PsmRelatedPluginListDispatchTimeThreshold) * 24 * time.Hour
			if time.Since(item.DispatchTime) > psmRelatedPluginListDispatchTimeThreshold {
				continue
			}

			parts := strings.Split(item.Key, "/")
			if len(parts) != 3 {
				continue
			}
			pluginName := parts[2]

			if _, ok := pluginByVregion[item.VRegion]; !ok {
				pluginByVregion[item.VRegion] = make(map[string]struct{})
			}
			pluginByVregion[item.VRegion][pluginName] = struct{}{}
		}
	}
	return nil, 0, nil
}

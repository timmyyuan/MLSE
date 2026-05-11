package diffcase

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"
)

type Region int

type GetPSMRelatedPluginListReq struct {
	Psm         string
	VRegionList string
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
	legoRegion2ClientMap := make(map[Region]*Client)
	legoRegion2Vregions := make(map[Region][]string)

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
		}
	}

	return nil, 0, nil
}

func GetPsmRelatedPluginList2(ctx context.Context, req *GetPSMRelatedPluginListReq) (*GetPSMRelatedPluginListRespData, int64, error) {
	legoRegion2ClientMap := make(map[Region]*Client)
	legoRegion2Vregions := make(map[Region][]string)

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
		}
	}

	return nil, 0, nil
}

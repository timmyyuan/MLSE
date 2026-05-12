package diffcase

import (
	"fmt"
	"strconv"
	"strings"
)

// Extracted from room_pack_compare getRankExtend and kept close to the real shape.
// Real diff point is only these two constant assignments:
// 1) fmt.Sprintf("助力主播冲刺小时榜") -> "助力主播冲刺小时榜"
// 2) fmt.Sprintf("助力主播冲刺人气榜") -> "助力主播冲刺人气榜"

type Context struct{}

type Key struct {
	id int64
}

func (k Key) Raw() any { return k.id }

type PreviewExtendAreaPart struct {
	Text      string
	FontSize  int
	Interval  int
	FontColor string
	Cuttable  bool
}

type PreviewExtendAreaActionConfig struct {
	AreaAction int
}

type PreviewExtendArea struct {
	IconType   int
	ExtendIcon string
	ExtendType int
	MidPart    []*PreviewExtendAreaPart
	ActionCfg  *PreviewExtendAreaActionConfig
}

type PreviewExtendAreaModel struct {
	IconURI string
}

type RequestInfoType struct{}

func RequestInfo(Context) RequestInfoType { return RequestInfoType{} }
func (RequestInfoType) GetAppID() int64   { return 1128 }

func PackOrigin(Context, int64, string) string { return "icon" }

const (
	RankHour        = 1
	RankPopular     = 2
	RankVertical    = 3
	IconTypeIcon    = 1
	ActionEnterWith = 1
)

type ABValue struct{}

type IABParam struct{}

func GetIABParam(Context) *IABParam           { return &IABParam{} }
func (*IABParam) Get(string, string) *ABValue { return &ABValue{} }
func (*ABValue) ToBool(bool) bool             { return true }

type RoomStruct struct {
	OwnerUserId int64
}

type RoomDataProxy struct{}

func (*RoomDataProxy) Result(Context, Key) (*RoomStruct, error) {
	return &RoomStruct{OwnerUserId: 7}, nil
}

type ByNameAnchorRankProxy struct{}

func (*ByNameAnchorRankProxy) Top(Context, Key, int64) (map[string]int, error) {
	return map[string]int{}, nil
}

type ByNamesAnchorRankProxy struct{}

func (*ByNamesAnchorRankProxy) TopAll(Context, Key, int64) (map[string]map[string]int, error) {
	return map[string]map[string]int{"vertical": {}}, nil
}

type HourEntranceConfig struct{}

func GetTccHourEntranceConfig(Context) HourEntranceConfig { return HourEntranceConfig{} }
func GetVerticalRankNames2Content(HourEntranceConfig, int64) map[string]string {
	return map[string]string{"vertical": "垂类"}
}

func keys[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

func IsPPE() bool { return true }

func LogsCtxWarn(Context, string, ...any)       {}
func LogsCtxInfo(Context, string, ...any)       {}
func LogsCtxPushNotice(Context, string, string) {}
func LogwCtxWarn(Context, string, ...any)       {}
func LogwCtxPushNotice(Context, string, string) {}

type ExtendBus struct {
	RoomData       *RoomDataProxy
	hourTop100     *ByNameAnchorRankProxy
	popTop100      *ByNameAnchorRankProxy
	verticalTop100 *ByNamesAnchorRankProxy
}

type resPreviewExpose struct {
	bus *ExtendBus
}

func getExtendData(ctx Context, extendArea *PreviewExtendArea, conf *PreviewExtendAreaModel, btnText string, extendType int) {
	if extendArea == nil || conf == nil {
		return
	}
	extendArea.IconType = IconTypeIcon
	extendArea.ExtendIcon = PackOrigin(ctx, RequestInfo(ctx).GetAppID(), conf.IconURI)
	extendArea.ExtendType = extendType
	extendArea.MidPart = []*PreviewExtendAreaPart{
		{Text: "查看直播", FontSize: 15},
		{Text: "丨", FontSize: 15, Interval: 0, FontColor: "#33FFFFFF"},
		{Text: btnText, FontSize: 15, Cuttable: true},
	}
	extendArea.ActionCfg = &PreviewExtendAreaActionConfig{AreaAction: ActionEnterWith}
}

func GetRankExtend1(r *resPreviewExpose, ctx Context, key Key, extendArea *PreviewExtendArea, conf *PreviewExtendAreaModel) (bool, error) {
	roomID := key.Raw().(int64)
	abParam := GetIABParam(ctx)
	if abParam == nil {
		LogwCtxWarn(ctx, "room_pack#rank | ab not found | roomID:%d", roomID)
		return false, nil
	}
	feedAbcConfig := abParam.Get("livehead_btn_add", "rank")
	if feedAbcConfig == nil || !feedAbcConfig.ToBool(false) {
		return false, nil
	}
	roomData, rErr := r.bus.RoomData.Result(ctx, key)
	if rErr != nil {
		return false, rErr
	}
	anchorID := strconv.FormatInt(roomData.OwnerUserId, 10)

	var text string
	// 小时榜
	hourTops, err := r.bus.hourTop100.Top(ctx, key, 100)
	if err != nil || len(hourTops) == 0 {
		LogsCtxWarn(ctx, "[getRankExtend]get hour rank err:%s", err)
	}
	_, isIn := hourTops[strconv.FormatInt(roomData.OwnerUserId, 10)]
	if isIn {
		text = fmt.Sprintf("助力主播冲刺小时榜")
		getExtendData(ctx, extendArea, conf, text, RankHour)
		LogsCtxPushNotice(ctx, "get_extend_hit_rank", anchorID)
		return true, nil
	}

	// 人气榜
	popTops, err := r.bus.popTop100.Top(ctx, key, 100)
	if err != nil || len(popTops) == 0 {
		LogsCtxWarn(ctx, "[getRankExtend]get pop rank err:%s", err)
	}
	_, isIn = popTops[strconv.FormatInt(roomData.OwnerUserId, 10)]
	if isIn {
		text = fmt.Sprintf("助力主播冲刺人气榜")
		getExtendData(ctx, extendArea, conf, text, RankPopular)
		LogsCtxPushNotice(ctx, "get_extend_hit_rank", anchorID)
		return true, nil
	}

	// 垂类榜单
	verticalTops, err := r.bus.verticalTop100.TopAll(ctx, key, 100)
	if err != nil {
		LogsCtxWarn(ctx, "[getRankExtend]get vertical rank err:%s", err.Error())
	}

	rankConf := GetTccHourEntranceConfig(ctx)
	rankNames2Content := GetVerticalRankNames2Content(rankConf, int64(1128))
	for _, rName := range keys(rankNames2Content) {
		if len(verticalTops[rName]) == 0 {
			continue
		}
		for _, k := range keys(verticalTops[rName]) {
			if strings.Contains(k, anchorID) {
				text = rankNames2Content[rName]
				text = fmt.Sprintf("主播冲刺%s榜中", text)
				getExtendData(ctx, extendArea, conf, text, RankVertical)
				LogwCtxPushNotice(ctx, "get_extend_hit_vertical_rank", rName)
				return true, nil
			}
		}
	}
	if IsPPE() {
		LogsCtxInfo(ctx, "vertical_ranks:%v", verticalTops)
		LogsCtxInfo(ctx, "hour_ranks:%v", hourTops)
		LogsCtxInfo(ctx, "pop_ranks:%v", popTops)
		LogwCtxPushNotice(ctx, "get_extend_hit_rank_anchor_id", anchorID)
	}

	return false, nil
}

func F(r *resPreviewExpose, ctx Context, key Key, extendArea *PreviewExtendArea, conf *PreviewExtendAreaModel) (bool, error) {
	roomID := key.Raw().(int64)
	abParam := GetIABParam(ctx)
	if abParam == nil {
		LogwCtxWarn(ctx, "room_pack#rank | ab not found | roomID:%d", roomID)
		return false, nil
	}
	feedAbcConfig := abParam.Get("livehead_btn_add", "rank")
	if feedAbcConfig == nil || !feedAbcConfig.ToBool(false) {
		return false, nil
	}
	roomData, rErr := r.bus.RoomData.Result(ctx, key)
	if rErr != nil {
		return false, rErr
	}
	anchorID := strconv.FormatInt(roomData.OwnerUserId, 10)

	var text string
	// 小时榜
	hourTops, err := r.bus.hourTop100.Top(ctx, key, 100)
	if err != nil || len(hourTops) == 0 {
		LogsCtxWarn(ctx, "[getRankExtend]get hour rank err:%s", err)
	}
	_, isIn := hourTops[strconv.FormatInt(roomData.OwnerUserId, 10)]
	if isIn {
		text = "助力主播冲刺小时榜"
		getExtendData(ctx, extendArea, conf, text, RankHour)
		LogsCtxPushNotice(ctx, "get_extend_hit_rank", anchorID)
		return true, nil
	}

	// 人气榜
	popTops, err := r.bus.popTop100.Top(ctx, key, 100)
	if err != nil || len(popTops) == 0 {
		LogsCtxWarn(ctx, "[getRankExtend]get pop rank err:%s", err)
	}
	_, isIn = popTops[strconv.FormatInt(roomData.OwnerUserId, 10)]
	if isIn {
		text = "助力主播冲刺人气榜"
		getExtendData(ctx, extendArea, conf, text, RankPopular)
		LogsCtxPushNotice(ctx, "get_extend_hit_rank", anchorID)
		return true, nil
	}

	// 垂类榜单
	verticalTops, err := r.bus.verticalTop100.TopAll(ctx, key, 100)
	if err != nil {
		LogsCtxWarn(ctx, "[getRankExtend]get vertical rank err:%s", err.Error())
	}

	rankConf := GetTccHourEntranceConfig(ctx)
	rankNames2Content := GetVerticalRankNames2Content(rankConf, int64(1128))
	for _, rName := range keys(rankNames2Content) {
		if len(verticalTops[rName]) == 0 {
			continue
		}
		for _, k := range keys(verticalTops[rName]) {
			if strings.Contains(k, anchorID) {
				text = rankNames2Content[rName]
				text = fmt.Sprintf("主播冲刺%s榜中", text)
				getExtendData(ctx, extendArea, conf, text, RankVertical)
				LogwCtxPushNotice(ctx, "get_extend_hit_vertical_rank", rName)
				return true, nil
			}
		}
	}
	if IsPPE() {
		LogsCtxInfo(ctx, "vertical_ranks:%v", verticalTops)
		LogsCtxInfo(ctx, "hour_ranks:%v", hourTops)
		LogsCtxInfo(ctx, "pop_ranks:%v", popTops)
		LogwCtxPushNotice(ctx, "get_extend_hit_rank_anchor_id", anchorID)
	}

	return false, nil
}

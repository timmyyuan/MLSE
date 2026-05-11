package diffcase

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

const LogUrlFormat = "https://%s/%s?psm=%s&start=%s&end=%s&keywords=%s&filter=%s&or=%t"

func F(ctx context.Context, info *InstanceInfo, logType InstanceUrlType,
	plugin string, startTime, endTime string, abnormalType *AbnormalInstanceInfoType) *InstanceUrlInfo {
	logMeta := getMatchedLogUrlMetaConfigItem(ctx, info)
	if logMeta == nil {
		logs.CtxWarn(ctx, "getMatchedLogUrlMetaConfigItem failed, info: %v", info)
		return nil
	}
	logParams := getMatchedLogUrlParamsConfigItem(ctx, logType, abnormalType)
	if logParams == nil {
		logs.CtxInfo(ctx, "getMatchedLogUrlParamsConfigItem unmatched, info: %v, logType: %v, abnormalType: %v", info, logType, abnormalType)
		return nil
	}
	keywords := genKeyWords(ctx, info, logParams, plugin)
	filterTag := ""
	if logParams.EnableErrorWarningFilter {
		filterTag = "_level||Warn::Error"
	}
	if logParams.EnablePodNameFilter {
		if filterTag != "" {
			filterTag += "$$"
		}
		filterTag += fmt.Sprintf("_podname||%s", info.PodName)

	}
	keywordsStr := strings.Join(keywords, ",")
	logUrl := fmt.Sprintf(LogUrlFormat, logMeta.Domain, logMeta.RegionArg,
		info.Psm, startTime, endTime, url.QueryEscape(keywordsStr), url.QueryEscape(filterTag), logParams.IsOr)
	return &InstanceUrlInfo{
		URL:  logUrl,
		Type: logType,
	}
}

func foo2(ctx context.Context, info *InstanceInfo, logType InstanceUrlType,
	plugin string, startTime, endTime string, abnormalType *AbnormalInstanceInfoType) *InstanceUrlInfo {
	logMeta := getMatchedLogUrlMetaConfigItem(ctx, info)
	if logMeta == nil {
		logs.CtxWarn(ctx, "getMatchedLogUrlMetaConfigItem failed, info: %v", info)
		return nil
	}
	logParams := getMatchedLogUrlParamsConfigItem(ctx, logType, abnormalType)
	if logParams == nil {
		logs.CtxInfo(ctx, "getMatchedLogUrlParamsConfigItem unmatched, info: %v, logType: %v, abnormalType: %v", info, logType, abnormalType)
		return nil
	}
	keywords := genKeyWords(ctx, info, logParams, plugin)
	filterTag := ""
	if logParams.EnableErrorWarningFilter {
		filterTag = "_level||Warn::Error"
	}
	if logParams.EnablePodNameFilter {
		if filterTag != "" {
			filterTag += "$$"
		}
		filterTag += "_podname||" + info.PodName

	}
	keywordsStr := strings.Join(keywords, ",")
	logUrl := fmt.Sprintf(LogUrlFormat, logMeta.Domain, logMeta.RegionArg,
		info.Psm, startTime, endTime, url.QueryEscape(keywordsStr), url.QueryEscape(filterTag), logParams.IsOr)
	return &InstanceUrlInfo{
		URL:  logUrl,
		Type: logType,
	}
}

type InstanceInfo struct {
	Psm     string
	PodName string
}

type InstanceUrlType int

type AbnormalInstanceInfoType int

type InstanceUrlInfo struct {
	URL  string
	Type InstanceUrlType
}

type LogUrlMeta struct {
	Domain    string
	RegionArg string
}

type LogUrlParams struct {
	EnableErrorWarningFilter bool
	EnablePodNameFilter      bool
	IsOr                     bool
}

type logger struct{}

func (logger) CtxWarn(ctx context.Context, format string, args ...any) {}

func (logger) CtxInfo(ctx context.Context, format string, args ...any) {}

var logs logger

func getMatchedLogUrlMetaConfigItem(ctx context.Context, info *InstanceInfo) *LogUrlMeta {
	if info == nil {
		return nil
	}
	return &LogUrlMeta{
		Domain:    "example.com",
		RegionArg: "region",
	}
}

func getMatchedLogUrlParamsConfigItem(ctx context.Context, logType InstanceUrlType, abnormalType *AbnormalInstanceInfoType) *LogUrlParams {
	return &LogUrlParams{
		EnableErrorWarningFilter: true,
		EnablePodNameFilter:      true,
		IsOr:                     false,
	}
}

func genKeyWords(ctx context.Context, info *InstanceInfo, logParams *LogUrlParams, plugin string) []string {
	return []string{"psm", plugin}
}

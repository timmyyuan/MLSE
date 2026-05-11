package diffcase

import (
	"example.com/smtcmpmod30/binding"
	"example.com/smtcmpmod30/db"
	"example.com/smtcmpmod30/logs"
	"example.com/smtcmpmod30/model"
	"example.com/smtcmpmod30/response"
)

type RequestContext struct{}

func F(ctx *RequestContext) []string {
	request := &model.AddOwnerRequest{}
	err := binding.BindAndValidate(ctx, request)
	if err != nil {
		logs.CtxWarn(ctx, "Binding json error, err:%v", err)
		response.BadRequest(ctx, "请求不合法，请确认JSON格式")
		return nil
	}

	if len(request.Plugins) == 0 || len(request.Owners) == 0 {
		response.OK(ctx, nil)
		return nil
	}

	var handlerPlugin []string
	for _, pluginName := range request.Plugins {
		s := &db.PluginInfo{}
		if err = s.GetByPluginName(pluginName); err != nil {
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	return handlerPlugin
}

func AddOwners2(ctx *RequestContext) []string {
	request := &model.AddOwnerRequest{}
	err := binding.BindAndValidate(ctx, request)
	if err != nil {
		logs.CtxWarn(ctx, "Binding json error, err:%v", err)
		response.BadRequest(ctx, "请求不合法，请确认JSON格式")
		return nil
	}

	if len(request.Plugins) == 0 || len(request.Owners) == 0 {
		response.OK(ctx, nil)
		return nil
	}

	var handlerPlugin []string
	if n := len(request.Plugins); n > 0 {
		handlerPlugin = make([]string, 0, n)
	}
	for _, pluginName := range request.Plugins {
		s := &db.PluginInfo{}
		if err = s.GetByPluginName(pluginName); err != nil {
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	return handlerPlugin
}

package diffcase

import (
	"encoding/json"

	"example.com/smtcmpmod31/binding"
	"example.com/smtcmpmod31/dal"
	"example.com/smtcmpmod31/db"
	"example.com/smtcmpmod31/logs"
	"example.com/smtcmpmod31/model"
	"example.com/smtcmpmod31/response"
)

type RequestContext struct{}

var (
	inputPlugins []string
	inputOwners  []string
)

func F(ctx *RequestContext) []string {
	operator := dal.GetUser(ctx)
	if !dal.IsSuperAdmin(operator) {
		response.BadRequest(ctx, "只有超级管理员才能调用")
		return nil
	}

	request := &model.AddOwnerRequest{}
	err := binding.BindAndValidate(ctx, request)
	if err != nil {
		logs.CtxWarn(ctx, "Binding json error, err:%v", err)
		response.BadRequest(ctx, "请求不合法，请确认JSON格式")
		return nil
	}
	request.Plugins = inputPlugins
	request.Owners = inputOwners

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
		var owners []string
		if s.Owners != "" {
			json.Unmarshal([]byte(s.Owners), &owners)
		}
		ownerMap := make(map[string]*dal.Empty)
		for i := range owners {
			ownerMap[owners[i]] = &dal.Empty{}
		}

		for i := range request.Owners {
			ownerMap[request.Owners[i]] = &dal.Empty{}
		}

		var newOwners []string
		for owner := range ownerMap {
			newOwners = append(newOwners, owner)
		}
		logs.Info("user %s try to update plugin %s", operator, pluginName)
		bytes, _ := json.Marshal(newOwners)
		s.Owners = string(bytes)

		if err = s.UpdateOwners(); err != nil {
			logs.CtxError(ctx, "db operation failed(%v)", err.Error())
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	response.OK(ctx, handlerPlugin)
	return handlerPlugin
}

func AddOwners2(ctx *RequestContext) []string {
	operator := dal.GetUser(ctx)
	if !dal.IsSuperAdmin(operator) {
		response.BadRequest(ctx, "只有超级管理员才能调用")
		return nil
	}

	request := &model.AddOwnerRequest{}
	err := binding.BindAndValidate(ctx, request)
	if err != nil {
		logs.CtxWarn(ctx, "Binding json error, err:%v", err)
		response.BadRequest(ctx, "请求不合法，请确认JSON格式")
		return nil
	}
	request.Plugins = inputPlugins
	request.Owners = inputOwners

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
		var owners []string
		if s.Owners != "" {
			json.Unmarshal([]byte(s.Owners), &owners)
		}
		ownerMap := make(map[string]*dal.Empty)
		for i := range owners {
			ownerMap[owners[i]] = &dal.Empty{}
		}

		for i := range request.Owners {
			ownerMap[request.Owners[i]] = &dal.Empty{}
		}

		var newOwners []string
		for owner := range ownerMap {
			newOwners = append(newOwners, owner)
		}
		logs.Info("user %s try to update plugin %s", operator, pluginName)
		bytes, _ := json.Marshal(newOwners)
		s.Owners = string(bytes)

		if err = s.UpdateOwners(); err != nil {
			logs.CtxError(ctx, "db operation failed(%v)", err.Error())
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	response.OK(ctx, handlerPlugin)
	return handlerPlugin
}

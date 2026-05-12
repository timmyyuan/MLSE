package diffcase

import (
	"example.com/smtcmpmod29/db"
	"example.com/smtcmpmod29/model"
)

type RequestContext struct{}

var (
	inputPlugins []string
	inputOwners  []string
)

func AddOwners1(ctx *RequestContext, forceFail bool) []string {
	request := &model.AddOwnerRequest{}
	var handlerPlugin []string
	for _, pluginName := range request.Plugins {
		s := &db.PluginInfo{}
		if err := s.GetByPluginName(pluginName); err != nil {
			continue
		}

		if forceFail {
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	return handlerPlugin
}

func F(ctx *RequestContext, forceFail bool) []string {
	request := &model.AddOwnerRequest{}
	var handlerPlugin []string
	if n := len(request.Plugins); n > 0 {
		handlerPlugin = make([]string, 0, n)
	}
	for _, pluginName := range request.Plugins {
		s := &db.PluginInfo{}
		if err := s.GetByPluginName(pluginName); err != nil {
			continue
		}

		if forceFail {
			continue
		}

		handlerPlugin = append(handlerPlugin, pluginName)
	}
	return handlerPlugin
}

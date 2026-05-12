package diffcase

import "encoding/json"

type RequestContext struct{}
type Empty struct{}
type AddOwnerRequest struct{ Plugins, Owners []string }
type PluginInfo struct{ Owners string }

func (p *PluginInfo) GetByPluginName(string) error { return nil }
func (p *PluginInfo) UpdateOwners() error          { return nil }

func foo1(ctx *RequestContext, req *AddOwnerRequest) []string {
	var handled []string
	for _, name := range req.Plugins {
		s := &PluginInfo{}
		if err := s.GetByPluginName(name); err != nil {
			continue
		}
		var owners []string
		if s.Owners != "" {
			_ = json.Unmarshal([]byte(s.Owners), &owners)
		}
		ownerMap := map[string]*Empty{}
		for i := range req.Owners {
			ownerMap[req.Owners[i]] = &Empty{}
		}

		b, _ := json.Marshal(owners)
		s.Owners = string(b)
		if err := s.UpdateOwners(); err != nil {
			continue
		}
		handled = append(handled, name)
	}
	return handled
}

func F(ctx *RequestContext, req *AddOwnerRequest) []string {
	var handled []string
	if n := len(req.Plugins); n > 0 {
		handled = make([]string, 0, n)
	}
	for _, name := range req.Plugins {
		s := &PluginInfo{}
		if err := s.GetByPluginName(name); err != nil {
			continue
		}
		var owners []string
		if s.Owners != "" {
			_ = json.Unmarshal([]byte(s.Owners), &owners)
		}
		ownerMap := map[string]*Empty{}
		for i := range req.Owners {
			ownerMap[req.Owners[i]] = &Empty{}
		}

		b, _ := json.Marshal(owners)
		s.Owners = string(b)
		if err := s.UpdateOwners(); err != nil {
			continue
		}
		handled = append(handled, name)
	}
	return handled
}

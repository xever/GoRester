package app

// 本文件实现工作区标签页的生命周期：树节点选中后打开接口/用例标签、
// 根据请求内容自动选中请求配置 Tab、新建草稿与关闭标签。

import (
	"gorester/internal/i18n"
	"gorester/internal/model"
)

// currentTab 返回当前工作区标签页（越界返回 nil）。
func (s *projectSession) currentTab() *workTab {
	if s.activeWork < 0 || s.activeWork >= len(s.workTabs) {
		return nil
	}
	return s.workTabs[s.activeWork]
}

// selectEntry 处理树节点被选中。
func (s *projectSession) selectEntry(e treeEntry) {
	s.selectedID = e.id
	switch e.kind {
	case nodeInterface:
		s.openInterfaceTab(e.iface, false)
	case nodeCase:
		s.openCaseTab(e.iface, e.case_)
	}
}

// openInterfaceTab 打开（或切换到）一个接口的编辑标签页。
func (s *projectSession) openInterfaceTab(iface *model.Interface, isNew bool) {
	// 已打开就直接切换。
	for i, t := range s.workTabs {
		if t.kind == tabInterface && t.iface == iface {
			s.activeWork = i
			s.autoSelectReqTab(iface.Params, iface.Headers, iface.Cookies, iface.Body, iface.Auth)
			return
		}
	}
	name := iface.Name
	if name == "" {
		name = i18n.T("workspace.newInterface")
	}
	s.workTabs = append(s.workTabs, &workTab{kind: tabInterface, id: iface.ID, name: name, isNew: isNew, iface: iface})
	s.activeWork = len(s.workTabs) - 1
	s.autoSelectReqTab(iface.Params, iface.Headers, iface.Cookies, iface.Body, iface.Auth)
}

// openCaseTab 打开（或切换到）一个用例的编辑标签页。
func (s *projectSession) openCaseTab(iface *model.Interface, cs *model.Case) {
	for i, t := range s.workTabs {
		if t.kind == tabCase && t.case_ == cs {
			s.activeWork = i
			s.autoSelectReqTab(cs.Params, cs.Headers, cs.Cookies, cs.Body, cs.Auth)
			return
		}
	}
	parentID := ""
	if iface != nil {
		parentID = iface.ID
	}
	s.workTabs = append(s.workTabs, &workTab{kind: tabCase, id: cs.ID, name: cs.Name, parentID: parentID, case_: cs})
	s.activeWork = len(s.workTabs) - 1
	s.autoSelectReqTab(cs.Params, cs.Headers, cs.Cookies, cs.Body, cs.Auth)
}

// autoSelectReqTab 根据接口 / 用例实际含有的参数类型，自动选中对应的请求配置 Tab，
// 并在选中 Body 时把 bodyFormat 同步成请求体格式。
func (s *projectSession) autoSelectReqTab(params, headers, cookies []model.KeyValue, body model.Body, auth model.Auth) {
	s.reqTab = reqTabFor(params, headers, cookies, body, auth)
	if s.reqTab == 1 { // Body 标签
		s.bodyFormat = bodyFormatFor(body.Mode)
	}
}

// reqTabFor 返回应打开的请求配置 Tab 下标：Body 优先，其次 Params、Header、Cookie、Auth。
func reqTabFor(params, headers, cookies []model.KeyValue, body model.Body, auth model.Auth) int {
	if body.Mode != "" && body.Mode != "none" {
		return 1 // Body
	}
	if hasKV(params) {
		return 0 // Params
	}
	if hasKV(headers) {
		return 3 // Header
	}
	if hasKV(cookies) {
		return 2 // Cookie
	}
	if auth.Type != "" && auth.Type != "none" {
		return 4 // Auth
	}
	return 0 // 默认 Params
}

// bodyFormatFor 返回请求体格式对应的子 Tab 下标。
func bodyFormatFor(mode string) int {
	for i, m := range bodyFormatModes {
		if m == mode {
			return i
		}
	}
	return 0
}

// hasKV 判断键值对切片里是否有有效条目（键名非空）。
func hasKV(kvs []model.KeyValue) bool {
	for _, kv := range kvs {
		if kv.Key != "" {
			return true
		}
	}
	return false
}

// createNewInterfaceDraft 新建一个未保存的接口草稿并打开编辑标签页。
// 用户编辑后在「保存」时再选择名称与目标文件夹。
func (s *projectSession) createNewInterfaceDraft(parentID string) {
	iface := &model.Interface{
		ID:      model.NewID(),
		Name:    i18n.T("workspace.newInterface"),
		Method:  "GET",
		Params:  []model.KeyValue{},
		Headers: []model.KeyValue{{Enabled: true}},
		Cookies: []model.KeyValue{},
	}
	s.draftParent = parentID
	s.openInterfaceTab(iface, true)
}

// closeWorkTab 关闭一个工作区标签页。
func (s *projectSession) closeWorkTab(i int) {
	if i < 0 || i >= len(s.workTabs) {
		return
	}
	s.workTabs = append(s.workTabs[:i], s.workTabs[i+1:]...)
	if len(s.workTabs) == 0 {
		// 至少保留一个「项目概览」。
		s.workTabs = append(s.workTabs, &workTab{kind: tabOverview, name: i18n.T("workspace.overview")})
		s.activeWork = 0
		return
	}
	if s.activeWork >= len(s.workTabs) {
		s.activeWork = len(s.workTabs) - 1
	}
}

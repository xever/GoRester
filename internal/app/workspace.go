package app

// 本文件是项目右侧主工作区：顶层页面、标签页管理、增删改、OpenAPI 导入导出、
// 发送请求、主工作区布局（标签栏 / 分栏 / 概览 / 状态栏）与保存逻辑。

import (
	"fmt"
	"os"
	"strings"
	"time"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/highlight"
	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/openapi"
	"gorester/internal/storage"
)

// 请求配置 / 响应结果 / Body 格式的名称表。
var (

	// todo 标签bodyFormatLabels改名x-www为简写，小心后面格式出错
	reqTabNames      = []string{"Params", "Body", "Cookie", "Header", "Auth"}
	respTabNames     = []string{"Body", "Cookie", "Header"}
	bodyFormatLabels = []string{"none", "form-data", "form-urlencoded", "json", "xml", "raw"}
	bodyFormatModes  = []string{"none", "form-data", "x-www-form-urlencoded", "json", "xml", "raw"}
	methodStrings    = []string{"GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	authTypeNames    = []string{"none", "basic", "bearer", "api-key"}

	// 各 Tab / 下拉框的下标切片（提为包级变量，避免每次 paint 重新分配）。
	reqTabIndices     = []int{0, 1, 2, 3, 4}
	bodyFormatIndices = []int{0, 1, 2, 3, 4, 5}
	respTabIndices    = []int{0, 1, 2, 3}
	prettyIndices     = []int{0, 1}

	// 下拉框选项。
	prettyOptions     = []string{"Pretty", "Raw"}
	langOptions       = []string{"JSON", "XML", "text"}
	encodingOptions   = []string{"utf-8", "GBK"}
	apiKeyInOptions   = []string{"header", "query"}
	formFieldTypes    = []string{"text", "file", "string", "number"}
	filterMethods     = []string{"", "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	filterMethodNames = []string{"", "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
)

// 这些是给 Select / Tabs 用的标签函数，提为包级函数避免每次 paint 分配闭包。
func reqTabLabel(i int) string     { return reqTabNames[i] }
func bodyFormatLabel(i int) string { return bodyFormatLabels[i] }
func respTabLabel(i int) string {
	if i == 3 {
		return i18n.T("workspace.requestTab")
	}
	return respTabNames[i]
}
func prettyLabel(i int) string      { return prettyOptions[i] }
func identityLabel(s string) string { return s }

// filterMethodLabel 把空过滤值显示为当前语言的「全部方法 / All Methods」。
func filterMethodLabel(i int) string {
	if i == 0 {
		return i18n.T("workspace.filterAll")
	}
	return filterMethodNames[i]
}

// langLabel 把内部的 "text" 选项显示为当前语言的「文本 / Text」。
func langLabel(s string) string {
	if s == "text" {
		return i18n.T("workspace.langText")
	}
	return s
}

// ---------------------------------------------------------------------------
// 项目页面（左右分栏）
// ---------------------------------------------------------------------------

// page 渲染整个项目工作区：左侧边栏 + 分隔条 + 右侧主工作区，外加弹窗。
func (s *projectSession) page(c ui.C) {
	if s.treeDirty {
		s.rebuildTree()
	}

	theme := *c.Theme()

	// 处理异步发送请求的结果（Effect 完成后在这里取回）。
	if s.sendKey > 0 {
		ui.Effect(c, s.send, s.sendKey)
		if res, _, ok := s.send.Take(); ok {
			s.response = res
			s.sending = false
			s.respLines, s.respHighlighted = highlight.BuildResponseLines(res.Body, s.responseContentType(), theme)
		}
	}

	ui.Row(c).Fill().Children(func() {
		// —— 左侧边栏 ——
		ui.Column(c).Width(s.split.Pos).Background(theme.Card).Children(func() {
			s.sidebar(c)
		})
		// —— 分隔条 ——
		ui.Splitter(c, &s.split).Mins(200, 420).Label(i18n.T("workspace.adjustSidebar"))
		// —— 右侧主工作区 ——
		ui.Column(c).Grow(1).Children(func() {
			s.workspace(c)
		})
	})

	// 弹窗与抽屉。
	s.dialogs(c)
}

// ---------------------------------------------------------------------------
// 标签页管理
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 增删改
// ---------------------------------------------------------------------------

// deleteFolder 删除文件夹及其全部内容。
func (s *projectSession) deleteFolder(f *model.Folder) {
	if f == nil {
		return
	}
	s.removeFolder(&s.project.Folders, f.ID)
	s.markTreeDirty()
	s.persist()
}

// removeFolder 递归地从切片里移除指定 ID 的文件夹。
func (s *projectSession) removeFolder(list *[]*model.Folder, id string) bool {
	for i, f := range *list {
		if f.ID == id {
			*list = append((*list)[:i], (*list)[i+1:]...)
			return true
		}
		if s.removeFolder(&f.Folders, id) {
			return true
		}
	}
	return false
}

// deleteInterface 删除一个接口（根目录或某个文件夹里）。
func (s *projectSession) deleteInterface(iface *model.Interface) {
	if iface == nil {
		return
	}
	// 根目录里找。
	for i, it := range s.project.Interfaces {
		if it == iface {
			s.project.Interfaces = append(s.project.Interfaces[:i], s.project.Interfaces[i+1:]...)
			s.afterDeleteInterface(iface)
			return
		}
	}
	// 文件夹里递归找。
	s.removeInterfaceFromFolders(s.project.Folders, iface)
	s.afterDeleteInterface(iface)
}

func (s *projectSession) removeInterfaceFromFolders(folders []*model.Folder, iface *model.Interface) bool {
	for _, f := range folders {
		for i, it := range f.Interfaces {
			if it == iface {
				f.Interfaces = append(f.Interfaces[:i], f.Interfaces[i+1:]...)
				return true
			}
		}
		if s.removeInterfaceFromFolders(f.Folders, iface) {
			return true
		}
	}
	return false
}

// afterDeleteInterface 删除接口后：关闭其标签页、清理选中、持久化。
func (s *projectSession) afterDeleteInterface(iface *model.Interface) {
	// 关闭对应标签页。
	for i := len(s.workTabs) - 1; i >= 0; i-- {
		if s.workTabs[i].kind == tabInterface && s.workTabs[i].iface == iface {
			s.workTabs = append(s.workTabs[:i], s.workTabs[i+1:]...)
		}
	}
	if len(s.workTabs) == 0 {
		s.workTabs = append(s.workTabs, &workTab{kind: tabOverview, name: i18n.T("workspace.overview")})
		s.activeWork = 0
	} else if s.activeWork >= len(s.workTabs) {
		s.activeWork = len(s.workTabs) - 1
	}
	s.markTreeDirty()
	s.persist()
}

// deleteCase 删除一个用例。
func (s *projectSession) deleteCase(iface *model.Interface, cs *model.Case) {
	if iface == nil || cs == nil {
		return
	}
	for i, c := range iface.Cases {
		if c == cs {
			iface.Cases = append(iface.Cases[:i], iface.Cases[i+1:]...)
			break
		}
	}
	s.markTreeDirty()
	s.persist()
}

// persist 把项目写回磁盘并提示。
func (s *projectSession) persist() {
	// 保存前先把代码编辑器里未落盘的编辑写回请求体。
	s.flushBodyEditor()
	if err := storage.SaveProject(s.project); err != nil {
		s.app.toaster.Show(i18n.T("toast.saveFailed"), err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show(i18n.T("toast.saved"), s.project.Name, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// OpenAPI 导入导出
// ---------------------------------------------------------------------------

// exportOpenAPI 把项目导出为 OpenAPI 3.0 文件。
func (s *projectSession) exportOpenAPI() {
	defer s.app.invalidate() // 对话框/提示后重绘，让 toast 显示
	doc := openapi.Export(s.project)
	data, err := doc.Marshal()
	if err != nil {
		s.app.toaster.Show(i18n.T("toast.exportFailed"), err.Error(), ui.ToastError)
		return
	}
	path, err := vivaapp.SaveFile(vivaapp.SaveFileOptions{
		Title:            i18n.T("workspace.exportOpenAPI"),
		SuggestedName:    s.project.Name + ".openapi.json",
		DefaultExtension: "json",
		Filters:          []vivaapp.FileFilter{{Name: i18n.T("workspace.openAPIJSON"), Pattern: "*.json"}},
	})
	if err != nil {
		s.app.toaster.Show(i18n.T("toast.exportCancelled"), err.Error(), ui.ToastInfo)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		s.app.toaster.Show(i18n.T("toast.exportFailed"), err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show(i18n.T("toast.exportSuccess"), path, ui.ToastSuccess)
}

// importOpenAPI 从文件导入 OpenAPI 3.0 到项目。
func (s *projectSession) importOpenAPI() {
	defer s.app.invalidate() // 导入后重绘，刷新目录树并显示 toast
	paths, err := vivaapp.OpenFiles(vivaapp.OpenFileOptions{
		Title:         i18n.T("workspace.importOpenAPI"),
		AllowMultiple: false,
		Filters:       []vivaapp.FileFilter{{Name: i18n.T("workspace.openAPIJSON"), Pattern: "*.json"}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		s.app.toaster.Show(i18n.T("toast.importFailed"), err.Error(), ui.ToastError)
		return
	}
	doc, err := openapi.Parse(data)
	if err != nil {
		s.app.toaster.Show(i18n.T("toast.importFailed"), err.Error(), ui.ToastError)
		return
	}
	openapi.Import(s.project, doc, doc.Info.Title)
	s.markTreeDirty()
	s.persist()
	s.app.toaster.Show(i18n.T("toast.importSuccess"), doc.Info.Title, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// 发送请求
// ---------------------------------------------------------------------------

// doSend 根据当前编辑内容 + 环境发起请求（异步）。
func (s *projectSession) doSend() {
	// 发送前先把代码编辑器里未落盘的编辑写回请求体。
	s.flushBodyEditor()
	opts := s.buildOptions()
	if opts.Method == "" {
		s.app.toaster.Show(i18n.T("toast.cannotSend"), i18n.T("toast.selectMethod"), ui.ToastError)
		return
	}
	// 记录本次实际发送的请求（用于「请求」标签展示）。
	s.lastReq = s.client.Describe(opts)
	s.sendMu.Lock()
	s.sendOpts = opts
	s.sendMu.Unlock()
	s.sending = true
	s.sendKey++
}

// responseContentType 从响应头里提取 Content-Type。
func (s *projectSession) responseContentType() string {
	if s.response == nil {
		return ""
	}
	for _, h := range s.response.Headers {
		if strings.EqualFold(h.Key, "Content-Type") {
			return h.Value
		}
	}
	return ""
}

// ---------------------------------------------------------------------------
// 主工作区布局
// ---------------------------------------------------------------------------

// workspace 渲染右侧主工作区（顶部标签栏 + 上下分栏）。
func (s *projectSession) workspace(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		// —— 顶部标签栏 + 环境操作 ——
		s.workTabBar(c)
		ui.Divider(c)

		// 项目概览标签单独渲染。
		if tab := s.currentTab(); tab != nil && tab.kind == tabOverview {
			s.overview(c)
			return
		}

		// —— 请求构造栏 ——
		s.urlBar(c)
		ui.Divider(c)

		// —— 上下分栏：请求配置 / 响应结果 ——
		ui.Column(c).Grow(1).Children(func() {
			ui.Column(c).Height(s.vsplit.Pos).Children(func() {
				s.requestConfig(c)
			})
			ui.Splitter(c, &s.vsplit).Mins(120, 160).Label(i18n.T("workspace.adjustSplit"))
			ui.Column(c).Grow(1).Children(func() {
				s.responsePanel(c)
			})
		})

		// —— 状态栏 ——
		s.statusBar(c)
	})
}

// overview 渲染「项目概览」标签页。
func (s *projectSession) overview(c ui.C) {
	theme := c.Theme()
	p := s.project
	ifaces := p.AllInterfaces()
	caseCount := 0
	for _, i := range ifaces {
		caseCount += len(i.Cases)
	}
	ui.Scroll(c).Fill().Pad(24).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, p.Name).FontSize(theme.Metrics.FontSizeL).Weight(gfx.WeightSemibold)
			if p.Description != "" {
				ui.Text(c, p.Description).Muted()
			}
			ui.Divider(c)
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.statInterfaces"), len(ifaces))).Muted()
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.statFolders"), countFolders(p.Folders))).Muted()
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.statCases"), caseCount)).Muted()
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.statEnvs"), len(p.Environments))).Muted()
		})
	})
}

// countFolders 递归统计文件夹数量。
func countFolders(folders []*model.Folder) int {
	n := len(folders)
	for _, f := range folders {
		n += countFolders(f.Folders)
	}
	return n
}

// statusBar 渲染底部状态栏（耗时 / 状态码 / 大小 / 环境）。
func (s *projectSession) statusBar(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().PadX(12).PadY(4).Gap(12).Background(theme.Card).Items(ui.Center).Children(func() {
		if s.sending {
			ui.Spinner(c)
			ui.Text(c, i18n.T("workspace.sending")).Muted()
		} else if s.response != nil {
			statusColor := gfx.RGB(34, 197, 94)
			if s.response.StatusCode >= 400 {
				statusColor = gfx.RGB(239, 68, 68)
			}
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.status"), s.response.StatusCode, httpStatusText(s.response.StatusCode))).Color(statusColor).Weight(gfx.WeightMedium)
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.duration"), s.response.Elapsed.Round(time.Millisecond))).Muted()
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.size"), humanBytes(len(s.response.Body)))).Muted()
		} else {
			ui.Text(c, i18n.T("workspace.notSent")).Muted()
		}
		ui.Spacer(c).Grow(1)
		if env := s.currentEnv(); env != nil {
			ui.Text(c, fmt.Sprintf(i18n.T("workspace.envStatus"), env.Name)).Muted()
		}
	})
}

// workTabBar 渲染工作区顶部标签页与环境操作区。
func (s *projectSession) workTabBar(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Height(40).PadX(8).Gap(2).Items(ui.Center).Children(func() {
		// 各标签页。
		for i, t := range s.workTabs {
			active := s.activeWork == i
			name := t.displayName()
			tab := ui.Row(c).PadLeft(12).PadRight(8).Height(32).Radius(theme.Metrics.RadiusControl).Gap(6).Items(ui.Center).
				Hit(ui.ID(c), fmt.Sprintf(i18n.T("common.workspaceTab"), name)).Hover(ui.Look{}.Background(theme.Hover)).
				Children(func() {
					if active {
						ui.Text(c, name).Color(theme.SelectionForeground)
					} else {
						ui.Text(c, name)
					}
					// 概览标签不可关闭，接口 / 用例标签可关闭。
					if t.kind != tabOverview {
						if tabCloseButton(c, *theme, name, active).Clicked() {
							s.closeWorkTab(i)
						}
					}
				})
			if active {
				tab.Background(theme.Selection)
			}
			if tab.Clicked() {
				s.activeWork = i
			}
		}

		// 新建接口按钮。
		if ui.Button(c, "").Icon(iconPlus).IconOnly().Ghost().Large().Clicked() {
			s.createNewInterfaceDraft(s.selectedFolderID())
		}

		ui.Spacer(c).Grow(1)

		// 环境选择下拉框。
		if len(s.project.Environments) > 0 {
			ui.Select(c, &s.envIndex, s.envIndices(), s.envLabelFn).Width(140).Label(i18n.T("workspace.envSelect"))
		}
		// 折叠菜单：编辑环境。
		if ui.Button(c, "").Icon(iconSettings).IconOnly().Ghost().Small().Clicked() {
			s.openEnvDrawer()
		}
	})
}

// ---------------------------------------------------------------------------
// 保存逻辑
// ---------------------------------------------------------------------------

// saveCurrent 保存当前编辑内容。
func (s *projectSession) saveCurrent() {
	tab := s.currentTab()
	if tab == nil {
		return
	}
	switch tab.kind {
	case tabInterface:
		if tab.isNew {
			// 新接口：弹窗选择名字与目标文件夹。
			s.saveName = tab.iface.Name
			s.saveDialogOpen = true
			return
		}
		s.persist()
	case tabCase:
		s.persist()
	}
}

// httpStatusText 返回状态码的常见描述。
func httpStatusText(code int) string {
	switch code {
	case 200:
		return "OK"
	case 201:
		return "Created"
	case 204:
		return "No Content"
	case 301:
		return "Moved Permanently"
	case 302:
		return "Found"
	case 304:
		return "Not Modified"
	case 400:
		return "Bad Request"
	case 401:
		return "Unauthorized"
	case 403:
		return "Forbidden"
	case 404:
		return "Not Found"
	case 405:
		return "Method Not Allowed"
	case 422:
		return "Unprocessable Entity"
	case 429:
		return "Too Many Requests"
	case 500:
		return "Internal Server Error"
	case 502:
		return "Bad Gateway"
	case 503:
		return "Service Unavailable"
	case 504:
		return "Gateway Timeout"
	}
	return ""
}

// humanBytes 把字节数格式化成可读文本。
func humanBytes(n int) string {
	switch {
	case n >= 1024*1024:
		return fmt.Sprintf("%.1f MB", float64(n)/(1024*1024))
	case n >= 1024:
		return fmt.Sprintf("%.1f KB", float64(n)/1024)
	default:
		return fmt.Sprintf("%d B", n)
	}
}

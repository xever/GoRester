package app

// 本文件是项目右侧主工作区：顶部标签页、URL 请求构造栏、请求参数配置区、
// 响应结果展示区以及底部状态栏。

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/codeeditor"

	"gorester/internal/highlight"
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
	respTabIndices    = []int{0, 1, 2}
	prettyIndices     = []int{0, 1}

	// 下拉框选项。
	prettyOptions     = []string{"Pretty", "Raw"}
	langOptions       = []string{"JSON", "XML", "文本"}
	encodingOptions   = []string{"utf-8", "GBK"}
	apiKeyInOptions   = []string{"header", "query"}
	formFieldTypes    = []string{"text", "file", "string", "number"}
	filterMethods     = []string{"", "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
	filterMethodNames = []string{"全部方法", "GET", "POST", "PUT", "DELETE", "PATCH", "HEAD", "OPTIONS"}
)

// 这些是给 Select / Tabs 用的标签函数，提为包级函数避免每次 paint 分配闭包。
func reqTabLabel(i int) string      { return reqTabNames[i] }
func bodyFormatLabel(i int) string  { return bodyFormatLabels[i] }
func respTabLabel(i int) string     { return respTabNames[i] }
func prettyLabel(i int) string      { return prettyOptions[i] }
func identityLabel(s string) string { return s }

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
		ui.Splitter(c, &s.split).Mins(200, 420).Label("调整侧栏宽度")
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
			return
		}
	}
	name := iface.Name
	if name == "" {
		name = "新接口"
	}
	s.workTabs = append(s.workTabs, &workTab{kind: tabInterface, id: iface.ID, name: name, isNew: isNew, iface: iface})
	s.activeWork = len(s.workTabs) - 1
}

// openCaseTab 打开（或切换到）一个用例的编辑标签页。
func (s *projectSession) openCaseTab(iface *model.Interface, cs *model.Case) {
	for i, t := range s.workTabs {
		if t.kind == tabCase && t.case_ == cs {
			s.activeWork = i
			return
		}
	}
	parentID := ""
	if iface != nil {
		parentID = iface.ID
	}
	s.workTabs = append(s.workTabs, &workTab{kind: tabCase, id: cs.ID, name: cs.Name, parentID: parentID, case_: cs})
	s.activeWork = len(s.workTabs) - 1
}

// createNewInterfaceDraft 新建一个未保存的接口草稿并打开编辑标签页。
// 用户编辑后在「保存」时再选择名称与目标文件夹。
func (s *projectSession) createNewInterfaceDraft(parentID string) {
	iface := &model.Interface{
		ID:      model.NewID(),
		Name:    "新接口",
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
		s.workTabs = append(s.workTabs, &workTab{kind: tabOverview, name: "项目概览"})
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
		s.workTabs = append(s.workTabs, &workTab{kind: tabOverview, name: "项目概览"})
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
		s.app.toaster.Show("保存失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("已保存", s.project.Name, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// OpenAPI 导入导出
// ---------------------------------------------------------------------------

// exportOpenAPI 把项目导出为 OpenAPI 3.0 文件。
func (s *projectSession) exportOpenAPI() {
	doc := openapi.Export(s.project)
	data, err := doc.Marshal()
	if err != nil {
		s.app.toaster.Show("导出失败", err.Error(), ui.ToastError)
		return
	}
	path, err := vivaapp.SaveFile(vivaapp.SaveFileOptions{
		Title:            "导出 OpenAPI",
		SuggestedName:    s.project.Name + ".openapi.json",
		DefaultExtension: "json",
		Filters:          []vivaapp.FileFilter{{Name: "OpenAPI JSON", Pattern: "*.json"}},
	})
	if err != nil {
		s.app.toaster.Show("导出取消", err.Error(), ui.ToastInfo)
		return
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		s.app.toaster.Show("导出失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("导出成功", path, ui.ToastSuccess)
}

// importOpenAPI 从文件导入 OpenAPI 3.0 到项目。
func (s *projectSession) importOpenAPI() {
	paths, err := vivaapp.OpenFiles(vivaapp.OpenFileOptions{
		Title:         "导入 OpenAPI",
		AllowMultiple: false,
		Filters:       []vivaapp.FileFilter{{Name: "OpenAPI JSON", Pattern: "*.json"}},
	})
	if err != nil || len(paths) == 0 {
		return
	}
	data, err := os.ReadFile(paths[0])
	if err != nil {
		s.app.toaster.Show("导入失败", err.Error(), ui.ToastError)
		return
	}
	doc, err := openapi.Parse(data)
	if err != nil {
		s.app.toaster.Show("导入失败", err.Error(), ui.ToastError)
		return
	}
	openapi.Import(s.project, doc, doc.Info.Title)
	s.markTreeDirty()
	s.persist()
	s.app.toaster.Show("导入成功", doc.Info.Title, ui.ToastSuccess)
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
		s.app.toaster.Show("无法发送", "请先选择 HTTP 方法", ui.ToastError)
		return
	}
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
			ui.Splitter(c, &s.vsplit).Mins(120, 160).Label("调整上下比例")
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
			ui.Text(c, fmt.Sprintf("接口数量：%d", len(ifaces))).Muted()
			ui.Text(c, fmt.Sprintf("文件夹数量：%d", countFolders(p.Folders))).Muted()
			ui.Text(c, fmt.Sprintf("用例数量：%d", caseCount)).Muted()
			ui.Text(c, fmt.Sprintf("环境数量：%d", len(p.Environments))).Muted()
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
			ui.Text(c, "请求中…").Muted()
		} else if s.response != nil {
			statusColor := gfx.RGB(34, 197, 94)
			if s.response.StatusCode >= 400 {
				statusColor = gfx.RGB(239, 68, 68)
			}
			ui.Text(c, fmt.Sprintf("状态: %d %s", s.response.StatusCode, httpStatusText(s.response.StatusCode))).Color(statusColor).Weight(gfx.WeightMedium)
			ui.Text(c, fmt.Sprintf("耗时: %s", s.response.Elapsed.Round(time.Millisecond))).Muted()
			ui.Text(c, fmt.Sprintf("大小: %s", humanBytes(len(s.response.Body)))).Muted()
		} else {
			ui.Text(c, "尚未发送请求").Muted()
		}
		ui.Spacer(c).Grow(1)
		if env := s.currentEnv(); env != nil {
			ui.Text(c, "环境: "+env.Name).Muted()
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
			tab := ui.Row(c).PadLeft(12).PadRight(8).Height(32).Radius(theme.Metrics.RadiusControl).Gap(6).Items(ui.Center).
				Hit(ui.ID(c), "工作区标签 "+t.name).Hover(ui.Look{}.Background(theme.Hover)).
				Children(func() {
					if active {
						ui.Text(c, t.name).Color(theme.SelectionForeground)
					} else {
						ui.Text(c, t.name)
					}
					// 概览标签不可关闭，接口 / 用例标签可关闭。
					if t.kind != tabOverview {
						if tabCloseButton(c, *theme, t.name, active).Clicked() {
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
			ui.Select(c, &s.envIndex, s.envIndices(), s.envLabelFn).Width(140).Label("环境")
		}
		// 折叠菜单：编辑环境。
		if ui.Button(c, "").Icon(iconSettings).IconOnly().Ghost().Small().Clicked() {
			s.openEnvDrawer()
		}
	})
}

// urlBar 渲染 URL 请求构造栏。
func (s *projectSession) urlBar(c ui.C) {
	theme := c.Theme()
	f := s.requestFields()
	ui.Row(c).FillWidth().Pad(12).Gap(8).Items(ui.Center).Children(func() {
		if f == nil {
			return
		}
		// —— 方法选择器：用 Row + 彩色文字实现，方法名显示对应颜色 ——
		methodBox := ui.Row(c).PadX(10).Height(theme.Metrics.ControlHeight).Radius(theme.Metrics.RadiusControl).Gap(4).Items(ui.Center).
			Background(theme.Card).Border(1, theme.Border).
			Hit(ui.ID(c), "请求方法").Hover(ui.Look{}.Background(theme.Hover)).
			Children(func() {
				ui.Text(c, *f.method).Color(methodColor(*f.method)).Weight(gfx.WeightSemibold)
				ui.Icon(c, iconChevronDown).Size(12, 12).Color(theme.MutedForeground)
			})
		if methodBox.Clicked() {
			s.methodMenuOpen = !s.methodMenuOpen
		}
		ui.Menu(c, methodBox, &s.methodMenuOpen, func(menu *ui.MenuItems) {
			for _, m := range methodStrings {
				if menu.Item(m).Clicked() {
					*f.method = m
				}
			}
		})

		// —— URL 输入框 ——
		ui.TextField(c, f.path, "请求路径", "/api/users").Grow(1)

		// —— 发送——
		if ui.Button(c, "发送").Icon(iconSend).Clicked() {
			s.doSend()
		}

		// —— 保存 / 保存为用例 ——
		if ui.Button(c, "保存").Icon(iconSave).Secondary().Clicked() {
			s.saveCurrent()
		}
		if ui.Button(c, "保存用例").Secondary().Clicked() {
			s.saveCaseName = ""
			s.saveCaseTarget = s.currentInterface()
			s.saveCaseOpen = true
		}
	})
}

// requestConfig 渲染请求参数配置区（Params / Body / Cookie / Header / Auth）。
func (s *projectSession) requestConfig(c ui.C) {
	f := s.requestFields()
	ui.Column(c).Fill().Children(func() {
		ui.Tabs(c, &s.reqTab, reqTabIndices, reqTabLabel).PadX(12)
		if f == nil {
			return
		}
		// Body 标签需要撑满高度（json/xml/raw 代码编辑器内部滚动），单独处理。
		if s.reqTab == 1 {
			s.bodyPanel(c, f.body)
			return
		}
		// 其它标签用可滚动区域。
		ui.Scroll(c).Grow(1).Pad(12).Children(func() {
			switch s.reqTab {
			case 0:
				kvEditor(c, f.params, kvColumns{valueLabel: "参数值"})
			case 2:
				kvEditor(c, f.cookies, kvColumns{valueLabel: "Cookie 值"})
			case 3:
				kvEditor(c, f.headers, kvColumns{valueLabel: "Header 值"})
			case 4:
				s.authPanel(c, f.auth)
			}
		})
	})
}

// bodyPanel 渲染请求体配置面板（格式子 Tab + 键值表 / 代码编辑器）。
func (s *projectSession) bodyPanel(c ui.C, body *model.Body) {
	ui.Column(c).Fill().Children(func() {
		// 格式子 Tab。
		ui.Tabs(c, &s.bodyFormat, bodyFormatIndices, bodyFormatLabel).
			Line().PadX(1)
		body.Mode = bodyFormatModes[s.bodyFormat]

		switch body.Mode {
		case "form-data":
			ui.Scroll(c).Grow(1).Pad(12).Children(func() {
				kvEditor(c, &body.FormData, kvColumns{withType: true, valueLabel: "示例值"})
			})
		case "x-www-form-urlencoded":
			ui.Scroll(c).Grow(1).Pad(12).Children(func() {
				kvEditor(c, &body.URLEncoded, kvColumns{withType: true, valueLabel: "示例值"})
			})
		case "json", "xml", "raw":
			s.bodyCodeEditor(c, body)
		default:
			ui.Text(c, "此接口没有请求体").Muted().Pad(12)
		}
	})
}

// bodyCodeEditor 渲染 json / xml / raw 的代码编辑器（撑满高度 + 语法高亮 + 内部滚动）。
func (s *projectSession) bodyCodeEditor(c ui.C, body *model.Body) {
	// 先同步：目标 body 变化时回写旧值并载入新值。
	s.bindBodyEditor(body)

	// 设置语言与高亮。
	s.bodyCode.Snapshot.Language = body.Mode
	s.bodyCode.Snapshot.Highlight = codeHighlight

	ui.Column(c).Fill().Pad(12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "内容").Muted().Small()
			ui.Spacer(c).Grow(1)
			if ui.Button(c, "格式化").Ghost().Small().Clicked() {
				// 以编辑器当前内容为准，格式化后写回 body 与编辑器。
				formatted := formatBody(s.bodyEditor.String(), body.Mode)
				body.Raw = formatted
				s.replaceEditorText(formatted)
			}
			if ui.Button(c, "压缩").Ghost().Small().Clicked() {
				compressed := minifyBody(s.bodyEditor.String(), body.Mode)
				body.Raw = compressed
				s.replaceEditorText(compressed)
			}
		})
		// 代码编辑器撑满剩余高度，内容超出时内部滚动。
		codeeditor.View(c, "请求体内容", &s.bodyCode).Fill()
	})
}

// authPanel 渲染认证配置面板。
func (s *projectSession) authPanel(c ui.C, auth *model.Auth) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Select(c, &auth.Type, authTypeNames, identityLabel).Width(160).Label("认证类型")
		switch auth.Type {
		case "basic":
			ui.TextField(c, &auth.Username, "用户名", "user").FillWidth()
			ui.TextField(c, &auth.Password, "密码", "").Secure().FillWidth()
		case "bearer":
			ui.TextField(c, &auth.Token, "Token", "Bearer token").FillWidth()
		case "api-key":
			ui.TextField(c, &auth.Key, "键名", "X-Api-Key").FillWidth()
			ui.TextField(c, &auth.Value, "值", "").FillWidth()
			ui.Select(c, &auth.In, apiKeyInOptions, identityLabel).Width(160).Label("位置")
		}
	})
}

// ---------------------------------------------------------------------------
// 响应结果展示区
// ---------------------------------------------------------------------------

// responsePanel 渲染响应结果区（结果 Tabs + 工具栏 + 内容）。
func (s *projectSession) responsePanel(c ui.C) {
	theme := *c.Theme()
	ui.Column(c).Fill().Children(func() {
		// 结果 Tabs + 工具栏。
		ui.Row(c).PadX(12).PadTop(4).Items(ui.Center).Children(func() {
			ui.Tabs(c, &s.respTab, respTabIndices, respTabLabel)

			ui.Spacer(c).Grow(1)

			// 左侧工具栏：格式化 / 语言 / 编码下拉。
			ui.Select(c, &s.respFormat, prettyIndices, prettyLabel).Width(90).Label("格式化")
			ui.Select(c, &s.respLang, langOptions, identityLabel).Width(80).Label("语言")
			ui.Select(c, &s.respEncoding, encodingOptions, identityLabel).Width(80).Label("编码")

			// 右侧工具图标：提取 / 下载 / 复制 / 搜索。
			if ui.Button(c, "提取").Icon(iconBookmark).IconOnly().Ghost().Small().Clicked() {
				s.app.toaster.Show("提取", "提取功能暂未实现", ui.ToastInfo)
			}
			if ui.Button(c, "下载").Icon(iconDownload).IconOnly().Ghost().Small().Clicked() {
				s.downloadResponse()
			}
			if ui.Button(c, "复制").Icon(iconCopy).IconOnly().Ghost().Small().Clicked() {
				s.copyResponse()
			}
			if ui.Button(c, "搜索").Icon(iconSearch).IconOnly().Ghost().Small().Clicked() {
				s.respFindOpen = !s.respFindOpen
			}
		})

		// 搜索栏。
		if s.respFindOpen {
			ui.Row(c).PadX(12).Gap(8).Items(ui.Center).Children(func() {
				ui.TextField(c, &s.respSearch, "查找", "输入关键字").Width(200)
				if ui.Button(c, "查找下一个").Ghost().Small().Clicked() {
					s.findNext()
				}
			})
		}

		ui.Divider(c)

		// 内容展示区。
		ui.Scroll(c).Grow(1).Children(func() {
			switch s.respTab {
			case 0:
				s.responseBody(c, theme)
			case 1:
				s.responseKVs(c, s.responseCookies())
			case 2:
				s.responseKVs(c, s.responseHeaders())
			}
		})
	})
}

// responseBody 渲染响应体（带行号、高亮）。
func (s *projectSession) responseBody(c ui.C, theme gfx.Theme) {
	if s.response == nil {
		ui.Column(c).Fill().Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "暂无响应").Muted()
		})
		return
	}
	if s.response.Err != nil {
		ui.Column(c).Pad(16).Children(func() {
			ui.Text(c, "请求出错").Color(gfx.RGB(239, 68, 68)).Weight(gfx.WeightMedium)
			ui.Text(c, s.response.Err.Error()).Muted()
		})
		return
	}

	// Raw 模式直接用原始字节按行展示。
	if s.respFormat == 1 {
		s.renderBodyLines(c, highlight.PlainLines(s.response.BodyString()), theme)
		return
	}
	s.renderBodyLines(c, s.respLines, theme)
}

// renderBodyLines 用 ui.List 渲染带行号的内容。
func (s *projectSession) renderBodyLines(c ui.C, lines []highlight.Line, theme gfx.Theme) {
	if len(lines) == 0 {
		ui.Text(c, "(空响应体)").Muted().Pad(12)
		return
	}
	ui.List(c, &s.respList, len(lines), func(row ui.ListRow) {
		line := lines[row.Index]
		row.Element.PadX(12).PadY(1)
		ui.Row(row.C).Gap(16).Items(ui.Start).Children(func() {
			// 行号。
			ui.Text(row.C, fmt.Sprintf("%d", row.Index+1)).Color(theme.MutedForeground).Mono().FontSize(12).Width(40).TextEnd()
			// 内容片段。
			ui.Row(row.C).Children(func() {
				for _, seg := range line.Segs {
					ui.Text(row.C, seg.Text).Color(seg.Color).Mono().FontSize(12)
				}
			})
		})
	}).Grow(1)
}

// responseKVs 渲染响应 Header / Cookie 键值对（只读展示）。
func (s *projectSession) responseKVs(c ui.C, kvs []model.KeyValue) {
	if len(kvs) == 0 {
		ui.Text(c, "(无数据)").Muted().Pad(12)
		return
	}
	ui.Column(c).Pad(12).Gap(4).Children(func() {
		for i := range kvs {
			kv := kvs[i]
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, kv.Key).Mono().FontSize(12).Weight(gfx.WeightMedium).Width(240)
				ui.Text(c, kv.Value).Mono().FontSize(12)
			})
		}
	})
}

// responseHeaders 返回响应头键值对。
func (s *projectSession) responseHeaders() []model.KeyValue {
	if s.response == nil {
		return nil
	}
	return s.response.Headers
}

// responseCookies 从响应头里解析 Set-Cookie。
func (s *projectSession) responseCookies() []model.KeyValue {
	if s.response == nil {
		return nil
	}
	var out []model.KeyValue
	for _, h := range s.response.Headers {
		if strings.EqualFold(h.Key, "Set-Cookie") {
			out = append(out, model.KeyValue{Key: "Set-Cookie", Value: h.Value, Enabled: true})
		}
	}
	return out
}

// findNext 在响应体里查找关键字并滚动到第一个匹配行。
func (s *projectSession) findNext() {
	if s.respSearch == "" || len(s.respLines) == 0 {
		return
	}
	needle := strings.ToLower(s.respSearch)
	for i, line := range s.respLines {
		for _, seg := range line.Segs {
			if strings.Contains(strings.ToLower(seg.Text), needle) {
				s.respList.ScrollIntoView(i)
				return
			}
		}
	}
	s.app.toaster.Show("未找到", "没有匹配「"+s.respSearch+"」的内容", ui.ToastInfo)
}

// copyResponse 把响应体复制到剪贴板。
func (s *projectSession) copyResponse() {
	if s.response == nil {
		return
	}
	if err := vivaapp.SetClipboardText(s.response.BodyString()); err != nil {
		s.app.toaster.Show("复制失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("已复制", "响应体已复制到剪贴板", ui.ToastSuccess)
}

// downloadResponse 把响应体保存到文件。
func (s *projectSession) downloadResponse() {
	if s.response == nil {
		return
	}
	ext := "txt"
	if strings.Contains(s.responseContentType(), "json") {
		ext = "json"
	} else if strings.Contains(s.responseContentType(), "xml") {
		ext = "xml"
	}
	path, err := vivaapp.SaveFile(vivaapp.SaveFileOptions{
		Title:            "下载响应",
		SuggestedName:    "response." + ext,
		DefaultExtension: ext,
	})
	if err != nil {
		return
	}
	if err := os.WriteFile(path, s.response.Body, 0o644); err != nil {
		s.app.toaster.Show("下载失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("已下载", path, ui.ToastSuccess)
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

// ---------------------------------------------------------------------------
// 请求字段访问
// ---------------------------------------------------------------------------

// currentInterface 返回当前标签页正在编辑的接口（编辑用例时返回其所属接口）。
func (s *projectSession) currentInterface() *model.Interface {
	tab := s.currentTab()
	if tab == nil {
		return nil
	}
	if tab.kind == tabInterface {
		return tab.iface
	}
	if tab.kind == tabCase {
		return s.project.FindInterface(tab.parentID)
	}
	return nil
}

// requestFields 返回当前标签页正在编辑的请求字段的指针。
// 编辑会直接写回模型里的接口 / 用例。
type requestFields struct {
	method  *string
	path    *string
	params  *[]model.KeyValue
	headers *[]model.KeyValue
	cookies *[]model.KeyValue
	body    *model.Body
	auth    *model.Auth
}

func (s *projectSession) requestFields() *requestFields {
	tab := s.currentTab()
	if tab == nil {
		return nil
	}
	if tab.kind == tabInterface && tab.iface != nil {
		i := tab.iface
		return &requestFields{&i.Method, &i.Path, &i.Params, &i.Headers, &i.Cookies, &i.Body, &i.Auth}
	}
	if tab.kind == tabCase && tab.case_ != nil {
		cc := tab.case_
		return &requestFields{&cc.Method, &cc.Path, &cc.Params, &cc.Headers, &cc.Cookies, &cc.Body, &cc.Auth}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// formatBody 根据模式格式化请求体文本。
func formatBody(raw, mode string) string {
	switch mode {
	case "json":
		if pretty, ok := highlight.PrettyJSON(raw); ok {
			return pretty
		}
	case "xml":
		return highlight.PrettyXML(raw)
	}
	return raw
}

// minifyBody 压缩请求体文本（去掉多余空白）。
func minifyBody(raw, mode string) string {
	if mode == "json" {
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				return string(b)
			}
		}
	}
	return strings.TrimSpace(raw)
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

package app

// 本文件是项目右侧主工作区的布局：顶层页面装配、标签栏、上下分栏、概览页与
// 状态栏。标签管理在 tab.go，目录树与增删改/持久化在 tree.go，侧栏弹窗与
// 导入导出在 sidebar.go，环境抽屉在 environment.go，请求发送在 send.go，
// 通用小工具在 util.go。

import (
	"fmt"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/highlight"
	"gorester/internal/i18n"
	"gorester/internal/model"
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

	// 各 Tab / 下拉框的下标切片。
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

// Package app 是 GoRester 的应用层：窗口入口、顶层状态、主页、项目工作区
// 以及所有界面组件。它组合了 model / storage / httpclient / openapi / highlight
// 这几个底层包，用 Viva 的 ui / gfx 把界面画出来。
package app

import (
	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/model"
	"gorester/internal/storage"
)

// ---------------------------------------------------------------------------
// 顶层状态
// ---------------------------------------------------------------------------

// appModel 是整个应用的顶层状态，持有项目列表和已打开的标签页。
type appModel struct {
	projects []storage.ProjectMeta // 主页展示的项目摘要
	tabs     []*projectSession     // 已打开的项目标签页
	active   int                   // 当前标签页下标，-1 表示主页

	// 主页「新建项目」弹窗。
	newProjectOpen bool
	newName        string
	newDesc        string

	toaster ui.Toaster // 全局轻提示
}

// newAppModel 创建应用状态，并加载磁盘上已有的项目列表。
func newAppModel() *appModel {
	m := &appModel{active: -1}
	m.reloadProjects()
	return m
}

// reloadProjects 重新扫描 ~/gorester/，刷新主页项目列表。
func (m *appModel) reloadProjects() {
	metas, err := storage.ListProjects()
	if err != nil {
		m.toaster.Show("加载项目失败", err.Error(), ui.ToastError)
		return
	}
	m.projects = metas
}

// ---------------------------------------------------------------------------
// 顶层页面
// ---------------------------------------------------------------------------

// page 是窗口的内容：顶部标签栏 + 下方内容区。
func (m *appModel) page(c ui.C) {
	ui.Column(c).Fill().Children(func() {
		m.tabBar(c)
		ui.Divider(c)
		// 内容区：主页或当前项目。
		if m.active < 0 {
			m.homePage(c)
		} else if m.active < len(m.tabs) {
			m.tabs[m.active].page(c)
		}
	})
	// 全局 toast，声明一次即可。
	ui.Toasts(c, &m.toaster)
}

// tabBar 绘制顶部标签栏：主页标签 + 各项目标签（带关闭按钮）。
func (m *appModel) tabBar(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Height(40).Background(theme.Background).PadX(8).Gap(2).Items(ui.Center).Children(func() {
		// 主页标签：用 Row 包住图标和文字，Hit 作为点击区域，图标与文字垂直居中。
		home := ui.Row(c).PadX(14).Height(34).Radius(theme.Metrics.RadiusControl).Gap(8).Items(ui.Center).
			Hit(ui.ID(c), "主页").Hover(ui.Look{}.Background(theme.Hover)).
			Children(func() {
				ui.Icon(c, iconHome).Size(18, 18).Color(theme.Primary)
				ui.Text(c, "主页")
			})
		if m.active < 0 {
			home.Background(theme.Card)
		}
		if home.Clicked() {
			m.active = -1
		}

		// 各项目标签。
		for i, s := range m.tabs {
			tab := ui.Row(c).PadLeft(12).PadRight(8).Height(34).Radius(theme.Metrics.RadiusControl).Gap(6).Items(ui.Center).
				Hit(ui.ID(c), "标签 "+s.project.Name).Hover(ui.Look{}.Background(theme.Hover)).
				Children(func() {
					ui.Text(c, s.project.Name)
					// 关闭按钮（灰色 x，靠左一点）。
					if tabCloseButton(c, *theme, s.project.Name).Clicked() {
						m.closeTab(i)
					}
				})
			if m.active == i {
				tab.Background(theme.Card)
			}
			if tab.Clicked() {
				m.active = i
			}
		}
	})
}

// tabCloseButton 渲染一个灰色、紧凑的标签页关闭按钮（返回可读 Clicked 的 Box）。
func tabCloseButton(c ui.C, theme gfx.Theme, label string) ui.Box {
	return ui.Row(c).Size(18, 18).Shrink(0).Center().Radius(theme.Metrics.RadiusControl).
		Hit(ui.ID(c), "关闭 "+label).
		Hover(ui.Look{}.Background(theme.Hover)).
		Children(func() {
			ui.Icon(c, iconClose).Size(12, 12).Color(theme.MutedForeground)
		})
}

// closeTab 关闭一个项目标签页，并同步调整当前激活下标。
func (m *appModel) closeTab(i int) {
	if i < 0 || i >= len(m.tabs) {
		return
	}
	m.tabs = append(m.tabs[:i], m.tabs[i+1:]...)
	if m.active == i {
		// 关闭的是当前标签，退到前一个；没有则回主页。
		m.active = i - 1
		if len(m.tabs) == 0 {
			m.active = -1
		}
	} else if m.active > i {
		m.active--
	}
}

// ---------------------------------------------------------------------------
// 项目打开 / 新建
// ---------------------------------------------------------------------------

// openProject 打开一个项目：创建会话并切换过去（若已打开则直接切过去）。
func (m *appModel) openProject(name string) {
	// 已打开就切过去，避免重复。
	for i, s := range m.tabs {
		if s.project.Name == name {
			m.active = i
			return
		}
	}
	p, err := storage.LoadProject(name)
	if err != nil {
		m.toaster.Show("打开项目失败", err.Error(), ui.ToastError)
		return
	}
	s := newProjectSession(m, p)
	m.tabs = append(m.tabs, s)
	m.active = len(m.tabs) - 1
}

// createProject 新建项目并立即打开。
func (m *appModel) createProject(name, desc string) {
	if name == "" {
		return
	}
	// 检查重名。
	for _, meta := range m.projects {
		if meta.Name == name {
			m.toaster.Show("创建失败", "已存在同名项目「"+name+"」", ui.ToastError)
			return
		}
	}
	p := model.NewProject(name, desc)
	if err := storage.SaveProject(p); err != nil {
		m.toaster.Show("保存失败", err.Error(), ui.ToastError)
		return
	}
	m.reloadProjects()
	s := newProjectSession(m, p)
	m.tabs = append(m.tabs, s)
	m.active = len(m.tabs) - 1
	m.toaster.Show("已创建项目", name, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// 入口
// ---------------------------------------------------------------------------

// Run 打开窗口并运行应用，阻塞直到窗口关闭。
func Run() {
	m := newAppModel()
	cfg := vivaapp.Config{
		Title:     "GoRester",
		Width:     1280,
		Height:    820,
		MinWidth:  960,
		MinHeight: 600,
	}
	// 跟随系统明暗主题（Theme 留空、开启 FollowSystemTheme）。
	page := ui.NewPage(ui.PageConfig{View: m.page, FollowSystemTheme: true})

	// 注意：必须用 RunCanvas（而非 app.Run）——RunCanvas 才会把页面的
	// 绘制回调（painter）挂到原生窗口上；app.Run 只开窗口不绘制，会黑屏。
	var canvas gfx.Canvas
	if err := vivaapp.RunCanvas(ui.Configure(page, cfg), &canvas); err != nil {
		panic(err)
	}
}

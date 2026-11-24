// Package app 是 GoRester 的应用层：窗口入口、顶层状态、主页、项目工作区
// 以及所有界面组件。它组合了 model / storage / httpclient / openapi / highlight
// 这几个底层包，用 Viva 的 ui / gfx 把界面画出来。
package app

import (
	"fmt"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/storage"
	"gorester/resource"
)

// 状态类型定义见 app.go；本文件负责应用生命周期与顶层页面。

// post 把 fn 投递到事件循环线程执行（在本轮绘制结束之后），用于文件对话框等
// 会阻塞当前线程的操作，避免在绘制过程中重入。无 application（例如测试）时
// 退化为同步执行。
func (m *appModel) post(fn func()) {
	if m.application != nil {
		m.application.Post(fn)
		return
	}
	fn()
}

// invalidate 请求一次重绘（后台工作改完状态后调用）。
func (m *appModel) invalidate() {
	m.inv.Invalidate()
}

// reloadProjects 重新扫描 ~/gorester/，刷新主页项目列表。
func (m *appModel) reloadProjects() {
	metas, err := storage.ListProjects()
	if err != nil {
		m.toaster.Show(i18n.T("toast.loadProjectsFailed"), err.Error(), ui.ToastError)
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
	// 设置抽屉与全局 toast，各声明一次即可。
	m.settingsDrawer(c)
	ui.Toasts(c, &m.toaster)
}

// tabBar 绘制顶部标签栏：主页标签 + 各项目标签（带关闭按钮）。
func (m *appModel) tabBar(c ui.C) {
	theme := c.Theme()
	ui.Row(c).FillWidth().Height(36).Background(theme.Background).PadX(8).Gap(2).Items(ui.Center).Children(func() {
		// 主页标签：用 Row 包住图标和文字，Hit 作为点击区域，图标与文字垂直居中。
		homeActive := m.active < 0
		home := ui.Row(c).PadX(12).Height(30).Radius(theme.Metrics.RadiusControl).Gap(8).Items(ui.Center).
			Hit(ui.ID(c), i18n.T("common.home")).Hover(ui.Look{}.Background(theme.Hover)).
			Children(func() {
				ui.Icon(c, iconHome).Size(16, 16).Color(theme.Primary)
				if homeActive {
					ui.Text(c, i18n.T("common.home")).Color(theme.SelectionForeground)
				} else {
					ui.Text(c, i18n.T("common.home"))
				}
			})
		if homeActive {
			home.Background(theme.Selection)
		}
		if home.Clicked() {
			m.active = -1
		}

		// 各项目标签。
		for i, s := range m.tabs {
			active := m.active == i
			tab := ui.Row(c).PadLeft(12).PadRight(8).Height(32).Radius(theme.Metrics.RadiusControl).Gap(6).Items(ui.Center).
				Hit(ui.ID(c), fmt.Sprintf(i18n.T("common.tab"), s.project.Name)).Hover(ui.Look{}.Background(theme.Hover)).
				Children(func() {
					if active {
						ui.Text(c, s.project.Name).Color(theme.SelectionForeground)
					} else {
						ui.Text(c, s.project.Name)
					}
					// 关闭按钮（x 标记）。
					if tabCloseButton(c, *theme, s.project.Name, active).Clicked() {
						m.closeTab(i)
					}
				})
			if active {
				tab.Background(theme.Selection)
			}
			if tab.Clicked() {
				m.active = i
			}
		}
		//右侧添加更多
		ui.Spacer(c)
		if ui.Button(c, "").Icon(iconMore).IconOnly().Tooltip(i18n.T("common.settings")).
			Label(i18n.T("common.settings")).Secondary().Clicked() {
			m.openSettings()
		}

		// 语言快捷切换：地球图标 + Viva 下拉菜单，当前偏好打勾。
		langBtn := ui.Button(c, "").Icon(iconGlobe).IconOnly().
			Tooltip(i18n.T("settings.langSwitch")).Label(i18n.T("settings.langSwitch")).Secondary()
		if langBtn.Clicked() {
			m.langMenuOpen = !m.langMenuOpen
		}
		ui.Menu(c, langBtn, &m.langMenuOpen, func(menu *ui.MenuItems) {
			//menu.Label(i18n.T("settings.language"))
			for _, value := range languageOptions {
				option := value
				if menu.Item(languageLabel(option)).Checked(m.langPref == option).Clicked() {
					m.applyLanguage(option)
				}
			}
		})
	})
}

// tabCloseButton 渲染一个紧凑的标签页关闭按钮（返回可读 Clicked 的 Box）。
// active 为 true 时，关闭图标用选中态前景色，保证在选中底色上清晰可见。
func tabCloseButton(c ui.C, theme gfx.Theme, label string, active bool) ui.Box {
	color := theme.MutedForeground
	if active {
		color = theme.SelectionForeground
	}
	return ui.Row(c).Size(18, 18).Shrink(0).Left(15).Radius(theme.Metrics.RadiusControl).
		Hit(ui.ID(c), fmt.Sprintf(i18n.T("common.closeTab"), label)).
		Hover(ui.Look{}.Background(theme.Hover)).
		Children(func() {
			ui.Icon(c, iconClose).Size(12, 12).Color(color)
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
		m.toaster.Show(i18n.T("toast.openProjectFailed"), err.Error(), ui.ToastError)
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
			m.toaster.Show(i18n.T("toast.createFailed"), fmt.Sprintf(i18n.T("toast.duplicateProject"), name), ui.ToastError)
			return
		}
	}
	p := model.NewProject(name, desc)
	if err := storage.SaveProject(p); err != nil {
		m.toaster.Show(i18n.T("toast.saveFailed"), err.Error(), ui.ToastError)
		return
	}
	m.reloadProjects()
	s := newProjectSession(m, p)
	m.tabs = append(m.tabs, s)
	m.active = len(m.tabs) - 1
	m.toaster.Show(i18n.T("toast.created"), name, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// 入口
// ---------------------------------------------------------------------------

// Run 打开窗口并运行应用，阻塞直到窗口关闭。
func Run() {
	// 安装崩溃日志，把 panic / 致命错误写到 ~/Documents/gorester/log/app.log。
	setupCrashLog()

	m := newAppModel()
	// 启动时按用户偏好选择界面语言；偏好为空则跟随系统语言。
	m.initLanguage()
	cfg := vivaapp.Config{
		Title:     "GoRester",
		Width:     1120,
		Height:    680,
		MinWidth:  960,
		MinHeight: 600,
	}
	// 运行时窗口图标（Windows 任务栏 / Alt+Tab、未打包时的 macOS Dock）。
	if size, rgba := resource.WindowIconRGBA(); size > 0 {
		cfg.Icon = vivaapp.WindowIcon{Size: size, RGBA: rgba}
	}
	// 让后台工作（文件对话框等）完成后能请求重绘。
	cfg.Invalidator = &m.inv

	// 用 RunApplication 拿到 *app.Application，这样文件对话框可以经 Post
	// 投递到本轮绘制之后，避免在绘制里打开模态对话框导致重入崩溃。
	if err := vivaapp.RunApplication(func(application *vivaapp.Application) error {
		m.application = application
		// 跟随系统明暗主题（Theme 留空、开启 FollowSystemTheme）。
		_, err := ui.OpenWindow(application, cfg, ui.PageConfig{View: m.page, FollowSystemTheme: true})
		return err
	}); err != nil {
		panic(err)
	}
}

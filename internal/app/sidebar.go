package app

// 本文件是项目左侧边栏：接口管理区。
// 顶部有搜索框、过滤按钮、新建按钮；下方是接口目录树；最右侧有导入导出。

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// methodBadge 渲染一个带字母的方法徽标（圆形小方块），颜色随方法变化。
func methodBadge(c ui.C, method string) {
	ui.Text(c, methodLetter(method)).
		Color(gfx.RGB(255, 255, 255)).
		Background(methodColor(method)).
		Size(20, 20).
		Radius(4).
		TextCenter().
		Weight(gfx.WeightSemibold).
		FontSize(11)
}

// sidebar 渲染左侧接口管理区。
func (s *projectSession) sidebar(c ui.C) {
	// —— 顶部：搜索 + 过滤 ——
	ui.Row(c).Pad(12).PadBottom(8).Gap(8).Items(ui.Center).Children(func() {
		search := ui.TextField(c, &s.sidebarSearch, "搜索", "搜索接口").Icon(iconSearch).Grow(1)
		if search.Changed() {
			s.markTreeDirty()
		}

		// 过滤按钮（漏斗）：按方法过滤。
		filterBtn := ui.Button(c, "过滤").Icon(iconFilter).IconOnly().Ghost()
		if filterBtn.Clicked() {
			s.filterMenuOpen = !s.filterMenuOpen
		}
		ui.Menu(c, filterBtn, &s.filterMenuOpen, func(menu *ui.MenuItems) {
			for i, method := range filterMethods {
				if menu.Item(filterMethodNames[i]).Clicked() {
					s.sidebarFilter = method
					s.markTreeDirty()
				}
			}
		})
	})

	// —— 新建 + 导入导出 ——
	ui.Row(c).PadX(12).PadBottom(8).Gap(8).Items(ui.Center).Children(func() {

		// 导入 / 导出 OpenAPI。
		if ui.Button(c, "").Icon(iconDownload).Ghost().Tooltip("导入OpenAPI").Small().Clicked() {
			runFileDialog(s.importOpenAPI)
		}
		if ui.Button(c, "").Icon(iconUpload).Ghost().Tooltip("导出").Small().Clicked() {
			runFileDialog(s.exportOpenAPI)
		}

		ui.Spacer(c).Grow(1)

		newBtn := ui.Button(c, "新建").Icon(iconPlus)
		if newBtn.Clicked() {
			s.newMenuOpen = !s.newMenuOpen
		}
		ui.Menu(c, newBtn, &s.newMenuOpen, func(menu *ui.MenuItems) {
			if menu.Item("新建文件夹").Clicked() {
				s.newFolderName = ""
				s.newFolderParent = s.selectedFolderID()
				s.newFolderOpen = true
			}
			if menu.Item("新建接口").Clicked() {
				s.createNewInterfaceDraft(s.selectedFolderID())
			}
		})

	})

	ui.Divider(c)

	// —— 目录树 ——
	if s.treeDirty {
		s.rebuildTree()
	}
	if len(s.flat) == 0 {
		ui.Column(c).Grow(1).Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "还没有接口").Muted()
			ui.Text(c, "点击上方「新建」创建接口或文件夹").Muted().Small()
		})
		return
	}

	ui.Tree(c, &s.tree, len(s.flat), s.labelFn, s.contentFn).Grow(1).Label("接口目录")

	// 处理展开 / 收起。
	if i, ok := s.tree.Toggled(); ok && i < len(s.flat) {
		e := s.flat[i]
		if e.kind == nodeFolder || e.kind == nodeInterface {
			s.expanded[e.id] = !s.expanded[e.id]
			s.markTreeDirty()
		}
	}
	// 处理点击选中。
	if i, ok := s.tree.Chosen(); ok && i < len(s.flat) {
		s.selectEntry(s.flat[i])
	}
}

// contentAt 渲染树的一行内容。
// 注意：这里从 TreeState 取当前 paint 的 C（Tree 在声明期间会临时保存它，
// 内容回调就在这个窗口内被调用），而不是把 C 存进自己的结构体。
func (s *projectSession) contentAt(i int) {
	if i < 0 || i >= len(s.flat) {
		return
	}
	e := s.flat[i]
	c := s.tree.GetC()
	theme := c.Theme()

	// 用一个占满整行的 Row 包住内容，并挂上右键菜单。
	// 用树行自身的 ID（s.tree.RowID(i)）作为 Hit 标识：既能让右键菜单有身份，
	// 又不会抢走树行的点击（同一 ID，树的 Chosen 仍能命中）。
	row := ui.Row(c).Grow(1).Gap(8).Items(ui.Center).Hit(s.tree.RowID(i), e.name).Children(func() {
		switch e.kind {
		case nodeFolder:
			ui.Icon(c, iconFolder).Color(theme.MutedForeground)
			ui.Text(c, e.name).Weight(gfx.WeightMedium)
		case nodeInterface:
			// 无用例的接口没有展开箭头，内容会比有用例的偏左
			// 这里补一个空白占位，让方法徽标与有用例的接口对齐。
			if len(e.iface.Cases) == 0 {
				ui.Icon(c, iconMore).Color(theme.Muted)
			}
			methodBadge(c, e.method)
			ui.Text(c, e.name)
		case nodeCase:
			ui.Icon(c, iconBookmark).Color(theme.MutedForeground).MarginLeft(10)
			ui.Text(c, e.name).Muted()
		}
	})

	// 右键菜单。
	ui.ContextMenu(c, row, func(menu *ui.MenuItems) {
		switch e.kind {
		case nodeFolder:
			if menu.Item("新建接口").Clicked() {
				s.createNewInterfaceDraft(e.id)
			}
			if menu.Item("新建文件夹").Clicked() {
				s.newFolderName = ""
				s.newFolderParent = e.id
				s.newFolderOpen = true
			}
			if menu.Item("删除文件夹").Clicked() {
				s.deleteFolder(e.folder)
			}
		case nodeInterface:
			if menu.Item("删除接口").Clicked() {
				s.deleteInterface(e.iface)
			}
			if menu.Item("另存为用例").Clicked() {
				s.saveCaseName = ""
				s.saveCaseTarget = e.iface
				s.saveCaseOpen = true
			}
		case nodeCase:
			if menu.Item("删除用例").Clicked() {
				s.deleteCase(e.iface, e.case_)
			}
		}
	})
}

// selectedFolderID 返回当前选中节点所属的文件夹 ID（无则返回空 = 根目录）。
func (s *projectSession) selectedFolderID() string {
	// 若选中的是文件夹，就建在它下面；否则回根目录。
	for _, e := range s.flat {
		if e.id == s.selectedID && e.kind == nodeFolder {
			return e.id
		}
	}
	return ""
}

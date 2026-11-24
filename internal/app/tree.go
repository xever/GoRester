package app

// 本文件实现左侧目录树的完整功能：树结构扁平化、搜索与按方法过滤、行渲染
// 与右键菜单、目录项增删改，以及项目持久化。sidebar.go 只保留侧栏外壳布局、
// 弹窗与 OpenAPI 导入导出。

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/storage"
)

// ---------------------------------------------------------------------------
// 树模型（扁平化）
// ---------------------------------------------------------------------------

// rebuildTree 重新扁平化目录树。只在结构变化时调用，避免每帧分配。
// 当搜索 / 过滤激活时，只展示匹配的接口及其祖先文件夹。
func (s *projectSession) rebuildTree() {
	s.flat = s.flat[:0]
	filtering := s.filterActive()
	for _, iface := range s.project.Interfaces {
		if !filtering || s.matchesInterface(iface) {
			s.appendInterface(iface, 0)
		}
	}
	s.appendFolders(s.project.Folders, 0, filtering)
	s.treeDirty = false
}

// appendFolders 递归追加文件夹及其内容（filtering 时跳过不含匹配项的文件夹）。
func (s *projectSession) appendFolders(folders []*model.Folder, depth int, filtering bool) {
	for _, f := range folders {
		if filtering && !s.folderMatches(f) {
			continue
		}
		s.flat = append(s.flat, treeEntry{kind: nodeFolder, depth: depth, id: f.ID, name: f.Name, folder: f})
		if s.expanded[f.ID] {
			for _, iface := range f.Interfaces {
				if !filtering || s.matchesInterface(iface) {
					s.appendInterface(iface, depth+1)
				}
			}
			s.appendFolders(f.Folders, depth+1, filtering)
		}
	}
}

// matchesInterface 判断接口是否命中搜索文本与方法过滤。
func (s *projectSession) matchesInterface(iface *model.Interface) bool {
	if s.sidebarFilter != "" && !strings.EqualFold(iface.Method, s.sidebarFilter) {
		return false
	}
	if s.sidebarSearch != "" && !strings.Contains(strings.ToLower(iface.Name), strings.ToLower(s.sidebarSearch)) {
		return false
	}
	return true
}

// folderMatches 判断文件夹（或其后代）是否含有匹配的接口。
func (s *projectSession) folderMatches(f *model.Folder) bool {
	for _, i := range f.Interfaces {
		if s.matchesInterface(i) {
			return true
		}
	}
	for _, sub := range f.Folders {
		if s.folderMatches(sub) {
			return true
		}
	}
	return false
}

// filterActive 返回搜索或过滤是否处于激活状态。
func (s *projectSession) filterActive() bool {
	return s.sidebarFilter != "" || s.sidebarSearch != ""
}

// appendInterface 追加一个接口；展开时继续追加它的用例。
func (s *projectSession) appendInterface(iface *model.Interface, depth int) {
	s.flat = append(s.flat, treeEntry{kind: nodeInterface, depth: depth, id: iface.ID, name: iface.Name, method: iface.Method, iface: iface})
	if s.expanded[iface.ID] {
		for _, cs := range iface.Cases {
			s.flat = append(s.flat, treeEntry{kind: nodeCase, depth: depth + 1, id: cs.ID, name: cs.Name, method: cs.Method, iface: iface, case_: cs})
		}
	}
}

// markTreeDirty 标记树结构已变化，下次 paint 时重建。
func (s *projectSession) markTreeDirty() { s.treeDirty = true }

// nodeAt 返回第 i 行的树节点描述（供 ui.Tree 使用）。
func (s *projectSession) nodeAt(i int) ui.TreeNode {
	e := s.flat[i]
	n := ui.TreeNode{Depth: e.depth, Selected: e.id == s.selectedID}
	switch e.kind {
	case nodeFolder:
		n.Branch = true
		n.Expanded = s.expanded[e.id]
	case nodeInterface:
		n.Branch = len(e.iface.Cases) > 0
		n.Expanded = s.expanded[e.id]
	}
	return n
}

// labelAt 返回第 i 行的可访问名称（也用于输入联想）。
func (s *projectSession) labelAt(i int) string { return s.flat[i].name }

// ---------------------------------------------------------------------------
// 树面板渲染（搜索 / 过滤 / 目录树）
// ---------------------------------------------------------------------------

// treeSearchBar 渲染顶部的搜索框与按方法过滤按钮。
func (s *projectSession) treeSearchBar(c ui.C) {
	ui.Row(c).Pad(12).PadBottom(8).Gap(8).Items(ui.Center).Children(func() {
		search := ui.TextField(c, &s.sidebarSearch, i18n.T("common.search"), i18n.T("sidebar.searchPlaceholder")).Icon(iconSearch).Grow(1)
		if search.Changed() {
			s.markTreeDirty()
		}

		// 过滤按钮（漏斗）：按方法过滤。
		filterBtn := ui.Button(c, i18n.T("common.filter")).Icon(iconFilter).IconOnly().Ghost()
		if filterBtn.Clicked() {
			s.filterMenuOpen = !s.filterMenuOpen
		}
		ui.Menu(c, filterBtn, &s.filterMenuOpen, func(menu *ui.MenuItems) {
			for i, method := range filterMethods {
				if menu.Item(filterMethodLabel(i)).Clicked() {
					s.sidebarFilter = method
					s.markTreeDirty()
				}
			}
		})
	})
}

// treeView 渲染接口目录树，并处理展开/收起与点击选中。
func (s *projectSession) treeView(c ui.C) {
	if s.treeDirty {
		s.rebuildTree()
	}
	if len(s.flat) == 0 {
		ui.Column(c).Grow(1).Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, i18n.T("sidebar.empty")).Muted()
			ui.Text(c, i18n.T("sidebar.emptyHint")).Muted().Small()
		})
		return
	}

	ui.Tree(c, &s.tree, len(s.flat), s.labelFn, s.contentFn).Grow(1).Label(i18n.T("sidebar.tree"))

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

	// 右键菜单。任何节点都可以重命名；删除统一走确认弹窗，并放在菜单最下方。
	ui.ContextMenu(c, row, func(menu *ui.MenuItems) {

		switch e.kind {
		case nodeFolder:
			if menu.Item(i18n.T("sidebar.newInterface")).Clicked() {
				s.createNewInterfaceDraft(e.id)
			}
			if menu.Item(i18n.T("sidebar.newFolder")).Clicked() {
				s.newFolderName = ""
				s.newFolderParent = e.id
				s.newFolderOpen = true
			}
		case nodeInterface:
			if menu.Item(i18n.T("sidebar.saveAsCase")).Clicked() {
				s.saveCaseName = ""
				s.saveCaseTarget = e.iface
				s.saveCaseOpen = true
			}
		}

		//重命名放下面，新加最上面 适合操作
		if menu.Item(i18n.T("common.rename")).Clicked() {
			s.startRename(e)
		}
		// 删除放最后，用分隔线与上面的操作分开。
		if e.kind != nodeCase {
			menu.Separator()
		}
		var label string
		switch e.kind {
		case nodeFolder:
			label = i18n.T("sidebar.deleteFolder")
		case nodeInterface:
			label = i18n.T("sidebar.deleteInterface")
		case nodeCase:
			label = i18n.T("sidebar.deleteCase")
		}
		if menu.Item(label).Destructive().Clicked() {
			s.confirmDelete(e)
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

// ---------------------------------------------------------------------------
// 目录项增删改与持久化
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
// 树节点删除确认 / 重命名弹窗（由 tree.go 的右键菜单触发）
// ---------------------------------------------------------------------------

// confirmDelete 记录待删除的树节点，弹出确认框。
func (s *projectSession) confirmDelete(e treeEntry) {
	s.deleteConfirmTarget = e
	s.deleteConfirmOpen = true
}

// deleteConfirmModal 在真正执行删除前请用户确认。
func (s *projectSession) deleteConfirmModal(c ui.C) {
	if !s.deleteConfirmOpen {
		return
	}
	e := s.deleteConfirmTarget
	// 弹窗打开期间节点可能已被别处删掉，避免误删或空指针。
	if !s.nodeStillExists(e) {
		s.deleteConfirmOpen = false
		return
	}
	title, desc := deleteConfirmText(e)
	ui.ModalWith(c, &s.deleteConfirmOpen, title, ui.ModalSpec{Description: desc}, nil,
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				s.deleteConfirmOpen = false
			}
			if ui.Button(c, i18n.T("common.delete")).Destructive().Clicked() {
				s.deleteConfirmOpen = false
				s.deleteNode(e)
			}
		},
	)
}

// deleteConfirmText 返回对应节点类型的确认标题与说明。
func deleteConfirmText(e treeEntry) (string, string) {
	switch e.kind {
	case nodeFolder:
		return i18n.T("dialog.confirmDeleteFolder"), fmt.Sprintf(i18n.T("dialog.confirmDeleteFolderDesc"), e.name)
	case nodeInterface:
		return i18n.T("dialog.confirmDeleteInterface"), fmt.Sprintf(i18n.T("dialog.confirmDeleteInterfaceDesc"), e.name)
	case nodeCase:
		return i18n.T("dialog.confirmDeleteCase"), fmt.Sprintf(i18n.T("dialog.confirmDeleteCaseDesc"), e.name)
	}
	return i18n.T("dialog.confirmDelete"), fmt.Sprintf(i18n.T("dialog.confirmDeleteDesc"), e.name)
}

// deleteNode 按节点种类执行真正的删除（实现在 tree.go）。
func (s *projectSession) deleteNode(e treeEntry) {
	switch e.kind {
	case nodeFolder:
		s.deleteFolder(e.folder)
	case nodeInterface:
		s.deleteInterface(e.iface)
	case nodeCase:
		s.deleteCase(e.iface, e.case_)
	}
}

// nodeStillExists 检查弹窗记录的节点是否还在项目里。
func (s *projectSession) nodeStillExists(e treeEntry) bool {
	switch e.kind {
	case nodeFolder:
		return e.folder != nil && s.project.FindFolder(e.id) != nil
	case nodeInterface:
		return e.iface != nil && s.project.FindInterface(e.id) != nil
	case nodeCase:
		return e.case_ != nil && s.project.FindCase(e.id) != nil
	}
	return false
}

// startRename 记录待重命名的树节点，打开重命名弹窗（文件夹 / 接口 / 用例通用）。
func (s *projectSession) startRename(e treeEntry) {
	s.renameTarget = e
	s.renameField.SetText(e.name)
	// 默认全选原名称，直接输入即可替换。
	s.renameField.Select(0, utf8.RuneCountInString(e.name))
	s.renameOpen = true
}

// renameModal 渲染重命名弹窗。
func (s *projectSession) renameModal(c ui.C) {
	if !s.renameOpen {
		return
	}
	e := s.renameTarget
	if !s.nodeStillExists(e) {
		s.renameOpen = false
		return
	}
	ui.Modal(c, &s.renameOpen, i18n.T("common.rename"),
		func() {
			field := ui.TextField(c, &s.renameField, i18n.T("dialog.renameName"), i18n.T("dialog.renameNamePlaceholder"))
			field.FillWidth().AutoFocus()
			if field.Submitted() { // 回车即提交
				s.commitRename()
			}
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				s.renameOpen = false
			}
			if ui.Button(c, i18n.T("common.rename")).Clicked() {
				s.commitRename()
			}
		},
	)
}

// commitRename 把新名字写回节点，并同步已打开的工作区标签标题。
func (s *projectSession) commitRename() {
	name := strings.TrimSpace(s.renameField.Text())
	if name == "" {
		return
	}
	e := s.renameTarget
	switch e.kind {
	case nodeFolder:
		if e.folder == nil {
			return
		}
		e.folder.Name = name
	case nodeInterface:
		if e.iface == nil {
			return
		}
		e.iface.Name = name
		for _, t := range s.workTabs {
			if t.kind == tabInterface && t.iface == e.iface {
				t.name = name
			}
		}
	case nodeCase:
		if e.case_ == nil {
			return
		}
		e.case_.Name = name
		for _, t := range s.workTabs {
			if t.kind == tabCase && t.case_ == e.case_ {
				t.name = name
			}
		}
	}
	s.renameOpen = false
	s.markTreeDirty()
	s.persist()
}

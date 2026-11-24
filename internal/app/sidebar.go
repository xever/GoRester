package app

// 本文件是左侧栏的外壳布局、OpenAPI 导入导出，以及新建文件夹/保存接口/
// 保存为用例/删除确认/重命名弹窗。树的渲染、搜索过滤、右键菜单与持久化
// 都在 tree.go。

import (
	"os"
	"strings"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/openapi"
)

// sidebar 渲染左侧接口管理区。
func (s *projectSession) sidebar(c ui.C) {
	// —— 顶部：搜索 + 过滤 ——
	s.treeSearchBar(c)

	// —— 新建 + 导入导出 ——
	ui.Row(c).PadX(12).PadBottom(8).Gap(8).Items(ui.Center).Children(func() {

		// 导入 / 导出 OpenAPI。
		if ui.Button(c, "").Icon(iconDownload).Ghost().Tooltip(i18n.T("sidebar.importOpenAPI")).Small().Clicked() {
			s.app.post(s.importOpenAPI)
		}
		if ui.Button(c, "").Icon(iconUpload).Ghost().Tooltip(i18n.T("sidebar.export")).Small().Clicked() {
			s.app.post(s.exportOpenAPI)
		}

		ui.Spacer(c).Grow(1)

		newBtn := ui.Button(c, i18n.T("common.new")).Icon(iconPlus)
		if newBtn.Clicked() {
			s.newMenuOpen = !s.newMenuOpen
		}
		ui.Menu(c, newBtn, &s.newMenuOpen, func(menu *ui.MenuItems) {
			if menu.Item(i18n.T("sidebar.newFolder")).Clicked() {
				s.newFolderName = ""
				s.newFolderParent = s.selectedFolderID()
				s.newFolderOpen = true
			}
			if menu.Item(i18n.T("sidebar.newInterface")).Clicked() {
				s.createNewInterfaceDraft(s.selectedFolderID())
			}
		})

	})

	ui.Divider(c)

	// —— 目录树 ——
	s.treeView(c)
}

// ---------------------------------------------------------------------------
// OpenAPI 导入导出（由侧边栏的导入/导出按钮触发）
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
// 弹窗（新建文件夹 / 保存新接口 / 保存为用例）
// ---------------------------------------------------------------------------

// dialogs 渲染当前项目会话的所有弹窗与抽屉。
func (s *projectSession) dialogs(c ui.C) {
	s.newFolderModal(c)
	s.saveDialog(c)
	s.saveCaseModal(c)
	s.deleteConfirmModal(c)
	s.renameModal(c)
	s.envDrawer(c)
}

func (s *projectSession) newFolderModal(c ui.C) {
	ui.Modal(c, &s.newFolderOpen, i18n.T("dialog.newFolder"),
		func() {
			ui.TextField(c, &s.newFolderName, i18n.T("dialog.folderName"), i18n.T("dialog.folderNamePlaceholder")).FillWidth()
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				s.newFolderOpen = false
			}
			if ui.Button(c, i18n.T("common.create")).Clicked() {
				if strings.TrimSpace(s.newFolderName) == "" {
					return
				}
				f := &model.Folder{ID: model.NewID(), Name: s.newFolderName}
				if s.newFolderParent == "" {
					s.project.Folders = append(s.project.Folders, f)
				} else if parent := s.project.FindFolder(s.newFolderParent); parent != nil {
					parent.Folders = append(parent.Folders, f)
					s.expanded[s.newFolderParent] = true
				}
				s.newFolderOpen = false
				s.markTreeDirty()
				s.persist()
			}
		},
	)
}

func (s *projectSession) saveDialog(c ui.C) {
	ui.Modal(c, &s.saveDialogOpen, i18n.T("dialog.saveInterface"),
		func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.TextField(c, &s.saveName, i18n.T("dialog.interfaceName"), i18n.T("dialog.interfaceNamePlaceholder")).FillWidth()
				s.folderPicker(c, &s.saveParent)
			})
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				s.saveDialogOpen = false
			}
			if ui.Button(c, i18n.T("common.save")).Clicked() {
				s.commitSaveDialog()
			}
		},
	)
}

// commitSaveDialog 把新接口草稿放入选中的文件夹并持久化。
func (s *projectSession) commitSaveDialog() {
	tab := s.currentTab()
	if tab == nil || !tab.isNew || tab.iface == nil {
		s.saveDialogOpen = false
		return
	}
	if strings.TrimSpace(s.saveName) == "" {
		return
	}
	target := s.project.TargetByPath(s.saveParent)
	tab.iface.Name = s.saveName
	if target.Folder == nil {
		s.project.Interfaces = append(s.project.Interfaces, tab.iface)
	} else {
		target.Folder.Interfaces = append(target.Folder.Interfaces, tab.iface)
		s.expanded[target.Folder.ID] = true
	}
	tab.isNew = false
	tab.name = s.saveName
	s.selectedID = tab.iface.ID
	s.saveDialogOpen = false
	s.markTreeDirty()
	s.persist()
}

// folderPathByID 返回文件夹 ID 对应的完整路径（供「目标文件夹」下拉框预选）。
// 空 ID 或找不到时返回根目录路径。
func (s *projectSession) folderPathByID(id string) string {
	targets := s.project.FolderTargets()
	if len(targets) == 0 {
		return ""
	}
	for _, t := range targets {
		if id != "" && t.Folder != nil && t.Folder.ID == id {
			return t.Path
		}
	}
	return targets[0].Path
}

// folderPicker 渲染一个「目标文件夹」下拉框。
func (s *projectSession) folderPicker(c ui.C, selected *string) {
	targets := s.project.FolderTargets()
	paths := make([]string, len(targets))
	for i, t := range targets {
		paths[i] = t.Path
	}
	// 保证当前值在选项内，否则 Select 会显示为空。
	found := false
	for _, p := range paths {
		if p == *selected {
			found = true
			break
		}
	}
	if !found {
		*selected = paths[0]
	}
	ui.Select(c, selected, paths, identityLabel).FillWidth().Label(i18n.T("dialog.targetFolder"))
}

func (s *projectSession) saveCaseModal(c ui.C) {
	ui.Modal(c, &s.saveCaseOpen, i18n.T("dialog.saveCase"),
		func() {
			ui.TextField(c, &s.saveCaseName, i18n.T("dialog.caseName"), i18n.T("dialog.caseNamePlaceholder")).FillWidth()
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				s.saveCaseOpen = false
			}
			if ui.Button(c, i18n.T("common.create")).Clicked() {
				s.commitSaveCase()
			}
		},
	)
}

// commitSaveCase 把当前接口保存为一个用例。
func (s *projectSession) commitSaveCase() {
	iface := s.saveCaseTarget
	if iface == nil {
		s.saveCaseOpen = false
		return
	}
	if strings.TrimSpace(s.saveCaseName) == "" {
		return
	}
	cs := &model.Case{
		ID:      model.NewID(),
		Name:    s.saveCaseName,
		Method:  iface.Method,
		Path:    iface.Path,
		Params:  cloneKVs(iface.Params),
		Headers: cloneKVs(iface.Headers),
		Cookies: cloneKVs(iface.Cookies),
		Body:    iface.Body,
		Auth:    iface.Auth,
	}
	iface.Cases = append(iface.Cases, cs)
	s.expanded[iface.ID] = true
	s.saveCaseOpen = false
	s.markTreeDirty()
	s.persist()
	s.app.toaster.Show(i18n.T("toast.caseSaved"), cs.Name, ui.ToastSuccess)
}

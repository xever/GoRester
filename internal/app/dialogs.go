package app

// 本文件包含所有弹窗（Modal）与抽屉（Drawer）：新建文件夹、保存新接口
// （选择名字与文件夹）、保存为用例、环境管理抽屉。

import (
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
)

// dialogs 渲染当前项目会话的所有弹窗与抽屉。
func (s *projectSession) dialogs(c ui.C) {
	s.newFolderModal(c)
	s.saveDialog(c)
	s.saveCaseModal(c)
	s.envDrawer(c)
}

// ---------------------------------------------------------------------------
// 新建文件夹
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 保存新接口（选择名称与目标文件夹）
// ---------------------------------------------------------------------------

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

// ---------------------------------------------------------------------------
// 保存为用例
// ---------------------------------------------------------------------------

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

// cloneKVs 深拷贝一个键值对切片，避免用例与接口共享底层数组。
func cloneKVs(src []model.KeyValue) []model.KeyValue {
	if src == nil {
		return nil
	}
	out := make([]model.KeyValue, len(src))
	copy(out, src)
	return out
}

// ---------------------------------------------------------------------------
// 环境管理抽屉
// ---------------------------------------------------------------------------

// openEnvDrawer 打开环境抽屉并同步编辑快照。
func (s *projectSession) openEnvDrawer() {
	s.syncEnvSnapshots()
	s.envDrawerOpen = true
}

// syncEnvSnapshots 把当前环境与全局的变量 / Header 复制到可编辑的切片。
func (s *projectSession) syncEnvSnapshots() {
	if env := s.currentEnv(); env != nil {
		s.envVarsEdit = mapToKV(env.Vars)
		s.envHeadersEdit = mapToKV(env.Headers)
		s.envTimeoutEdit = strconv.Itoa(env.TimeoutMs / 1000) // 以秒展示
	}
	s.globalVarsEdit = mapToKV(s.project.GlobalVars)
	s.globalHeadersEdit = mapToKV(s.project.GlobalHeaders)
}

// commitEnvSnapshots 把编辑快照写回环境与全局变量。
func (s *projectSession) commitEnvSnapshots() {
	if env := s.currentEnv(); env != nil {
		env.Vars = kvToMap(s.envVarsEdit)
		env.Headers = kvToMap(s.envHeadersEdit)
		if sec, err := strconv.Atoi(strings.TrimSpace(s.envTimeoutEdit)); err == nil && sec > 0 {
			env.TimeoutMs = sec * 1000
		}
	}
	s.project.GlobalVars = kvToMap(s.globalVarsEdit)
	s.project.GlobalHeaders = kvToMap(s.globalHeadersEdit)
	// 环境超时变化时同步到复用客户端。
	if env := s.currentEnv(); env != nil && env.TimeoutMs > 0 {
		s.client.SetTimeout(timeDurationMs(env.TimeoutMs))
	}
	s.persist()
}

func (s *projectSession) envDrawer(c ui.C) {
	ui.Drawer(c, &s.envDrawerOpen, i18n.T("dialog.envManage"), ui.DrawerSpec{Size: 480},
		func() {
			ui.Scroll(c).Fill().Pad(16).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					// —— 环境选择 / 增删 ——
					ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
						envs := s.project.Environments
						sel := ui.Select(c, &s.envIndex, s.envIndices(), s.envLabelFn).Grow(1).Label(i18n.T("dialog.selectEnv"))
						if sel.Changed() {
							s.syncEnvSnapshots()
						}
						if ui.Button(c, i18n.T("dialog.addEnv")).Icon(iconPlus).Ghost().Small().Clicked() {
							s.project.Environments = append(s.project.Environments, &model.Environment{Name: i18n.T("dialog.newEnv"), Vars: map[string]string{}, Headers: map[string]string{}, TimeoutMs: model.DefaultTimeoutMs})
							s.envIndex = len(s.project.Environments) - 1
							s.syncEnvSnapshots()
						}
						if len(envs) > 1 && ui.Button(c, i18n.T("dialog.deleteEnv")).Icon(iconTrash).Ghost().Small().Clicked() {
							s.project.Environments = append(envs[:s.envIndex], envs[s.envIndex+1:]...)
							if s.envIndex >= len(s.project.Environments) {
								s.envIndex = len(s.project.Environments) - 1
							}
							s.syncEnvSnapshots()
						}
					})

					// —— 当前环境基础配置 ——
					if env := s.currentEnv(); env != nil {
						ui.Text(c, i18n.T("dialog.basic")).Weight(gfx.WeightSemibold)
						ui.TextField(c, &env.Name, i18n.T("dialog.envName"), "").FillWidth()
						ui.TextField(c, &env.BaseURL, i18n.T("dialog.baseURL"), i18n.T("dialog.baseURLPlaceholder")).FillWidth()
						ui.TextField(c, &s.envTimeoutEdit, i18n.T("dialog.timeout"), "30").FillWidth()

						ui.Divider(c)
						ui.Text(c, i18n.T("dialog.envVars")).Weight(gfx.WeightSemibold)
						s.kvEditor(c, &s.envVarsEdit, kvColumns{valueLabel: i18n.T("common.value")})
						ui.Divider(c)
						ui.Text(c, i18n.T("dialog.envHeaders")).Weight(gfx.WeightSemibold)
						s.kvEditor(c, &s.envHeadersEdit, kvColumns{valueLabel: i18n.T("common.value")})
					}

					ui.Divider(c)
					ui.Text(c, i18n.T("dialog.globalVars")).Weight(gfx.WeightSemibold)
					s.kvEditor(c, &s.globalVarsEdit, kvColumns{valueLabel: i18n.T("common.value")})
					ui.Divider(c)
					ui.Text(c, i18n.T("dialog.globalHeaders")).Weight(gfx.WeightSemibold)
					s.kvEditor(c, &s.globalHeadersEdit, kvColumns{valueLabel: i18n.T("common.value")})
				})
			})
		},
		func() {
			if ui.Button(c, i18n.T("common.done")).Clicked() {
				s.commitEnvSnapshots()
				s.envDrawerOpen = false
			}
		},
	)
}

// ---------------------------------------------------------------------------
// map / 切片互转
// ---------------------------------------------------------------------------

// mapToKV 把 map[string]string 转成有序的 []model.KeyValue（键名排序，便于编辑）。
func mapToKV(m map[string]string) []model.KeyValue {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make([]model.KeyValue, 0, len(keys))
	for _, k := range keys {
		out = append(out, model.KeyValue{Key: k, Value: m[k], Enabled: true})
	}
	return out
}

// kvToMap 把 []model.KeyValue 转回 map[string]string（忽略空键）。
func kvToMap(kvs []model.KeyValue) map[string]string {
	m := map[string]string{}
	for _, kv := range kvs {
		if kv.Key != "" {
			m[kv.Key] = kv.Value
		}
	}
	return m
}

// timeDurationMs 把毫秒转成 time.Duration（避免在此处引入 time 包多次）。
func timeDurationMs(ms int) time.Duration {
	return time.Duration(ms) * time.Millisecond
}

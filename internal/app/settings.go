package app

// 本文件是全局设置：手风琴布局的语言设置 / 项目管理 / 保存目录说明，
// 以及抽屉底部的作者与版本信息、项目改名与删除弹窗。
// 语言偏好保存在 ~/Documents/gorester/config.json；"" 表示跟随系统语言。

import (
	"fmt"
	"strings"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/storage"
)

// authorURL 是设置抽屉里作者信息指向的主页。
const authorURL = "https://github.com/xever/"

// languageOptions 是设置里可选的语言值；空串表示跟随系统。
var languageOptions = []string{string(i18n.Auto), string(i18n.Chinese), string(i18n.English)}

// languageLabel 把语言值显示为当前界面语言下的名称。
func languageLabel(value string) string {
	switch i18n.Language(value) {
	case i18n.Chinese:
		return i18n.T("settings.lang.zh")
	case i18n.English:
		return i18n.T("settings.lang.en")
	default:
		return i18n.T("settings.lang.auto")
	}
}

// initLanguage 在应用启动时按保存的偏好（或系统语言）设置界面语言。
func (m *appModel) initLanguage() {
	i18n.Set(i18n.Resolve(m.langPref, vivaapp.PreferredLanguages()))
}

// openSettings 打开设置抽屉，并把当前偏好填入下拉框。
func (m *appModel) openSettings() {
	m.settingsLang = m.langPref
	m.settingsOpen = true
}

// applyLanguage 保存新的语言偏好并立即切换界面语言。
func (m *appModel) applyLanguage(pref string) {
	m.langPref = pref
	i18n.Set(i18n.Resolve(pref, vivaapp.PreferredLanguages()))
	if err := storage.SavePreferences(storage.Preferences{Language: pref}); err != nil {
		m.toaster.Show(i18n.T("toast.settingsSaveFailed"), err.Error(), ui.ToastError)
	}
}

// openAuthorPage 在系统浏览器里打开作者主页。打开文件对话框一样投递到绘制之后执行，
// 避免在绘制过程中调用会阻塞的系统 API。
func (m *appModel) openAuthorPage() {
	m.post(func() {
		if err := vivaapp.OpenURL(authorURL); err != nil {
			m.toaster.Show(i18n.T("toast.openLinkFailed"), err.Error(), ui.ToastError)
			m.invalidate()
		}
	})
}

// openDataDir 在系统文件管理器中打开项目保存目录。系统 API 会阻塞，投递到绘制之后。
func (m *appModel) openDataDir() {
	dir, err := storage.DataDir()
	if err != nil {
		m.toaster.Show(i18n.T("toast.openDirFailed"), err.Error(), ui.ToastError)
		return
	}
	m.post(func() {
		if err := vivaapp.RevealPath(dir); err != nil {
			m.toaster.Show(i18n.T("toast.openDirFailed"), err.Error(), ui.ToastError)
			m.invalidate()
		}
	})
}

// settingsDrawer 绘制设置抽屉：可滚动的手风琴主体 + 固定底部。
func (m *appModel) settingsDrawer(c ui.C) {
	theme := c.Theme()
	ui.Drawer(c, &m.settingsOpen, i18n.T("settings.title"), ui.DrawerSpec{Size: 480},
		func() {
			ui.Scroll(c).Fill().Pad(16).Children(func() {
				ui.Accordion(c, m.settingsAccordion, []ui.AccordionItem{
					{Title: i18n.T("settings.language"), Content: func() {
						ui.Column(c).Gap(8).Children(func() {
							ui.Text(c, i18n.T("settings.languageHint")).Muted()
							sel := ui.Select(c, &m.settingsLang, languageOptions, languageLabel).FillWidth().Label(i18n.T("settings.language"))
							if sel.Changed() {
								m.applyLanguage(m.settingsLang)
							}
						})
					}},
					{Title: i18n.T("settings.projects"), Content: func() {
						m.projectManager(c)
					}},
					{Title: i18n.T("settings.storage"), Content: func() {
						m.storageInfo(c)
					}},
				})
			})
		},
		func() {
			// —— 底部：作者信息与版本号 ——
			ui.Column(c).Gap(4).Items(ui.Center).Children(func() {
				author := ui.Text(c, i18n.T("settings.author")).
					Color(theme.Primary).TextCenter().
					Hit(ui.ID(c), i18n.T("settings.author")).
					Tooltip(i18n.T("settings.authorTooltip")).
					Hover(ui.Look{}.Opacity(0.75))
				if author.Clicked() {
					m.openAuthorPage()
				}
				ui.Text(c, fmt.Sprintf(i18n.T("settings.version"), appVersion)).Muted().Small().TextCenter()
			})
		},
	)

	// 项目管理弹窗：编辑项目、删除二次确认。
	m.editProjectModal(c)
	m.deleteProjectModal(c)
}

// ---------------------------------------------------------------------------
// 手风琴第二项：项目管理
// ---------------------------------------------------------------------------

// projectManager 列出已有项目，并提供编辑与删除入口。
func (m *appModel) projectManager(c ui.C) {
	if len(m.projects) == 0 {
		ui.Text(c, i18n.T("settings.noProjects")).Muted()
		return
	}
	theme := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		for i, meta := range m.projects {
			meta := meta
			// 用行下标作 Key，保证按钮在列表变化前后身份稳定。
			ui.Row(c.KeyInt(i)).FillWidth().Pad(10).Gap(8).Items(ui.Center).
				Background(theme.Card).Radius(theme.Metrics.RadiusControl).Children(func() {
				ui.Column(c).Grow(1).Gap(2).Children(func() {
					ui.Text(c, meta.Name).Weight(gfx.WeightMedium)
					ui.Text(c, meta.Description).Muted().Small().MaxLines(2)
				})
				if ui.Button(c, i18n.T("settings.editProject")).Small().Outline().Clicked() {
					m.openEditProject(meta.Name)
				}
				if ui.Button(c, i18n.T("common.delete")).Small().DestructiveOutline().Clicked() {
					m.deleteProjectName = meta.Name
					m.deleteProjectOpen = true
				}
			})
		}
	})
}

// openEditProject 用当前项目信息预填编辑弹窗。
func (m *appModel) openEditProject(name string) {
	m.editProjectOldName = name
	for _, meta := range m.projects {
		if meta.Name == name {
			m.editName = meta.Name
			m.editDesc = meta.Description
			break
		}
	}
	m.editProjectOpen = true
}

// editProjectModal 编辑项目名称与描述（字段同「新建项目」）。
func (m *appModel) editProjectModal(c ui.C) {
	ui.Modal(c, &m.editProjectOpen, i18n.T("dialog.editProject"),
		func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.TextField(c, &m.editName, i18n.T("home.name"), i18n.T("home.namePlaceholder")).FillWidth()
				ui.TextArea(c, &m.editDesc, i18n.T("home.desc"), i18n.T("home.descPlaceholder")).FillWidth().AutoGrow(5)
			})
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				m.editProjectOpen = false
			}
			if ui.Button(c, i18n.T("common.save")).Clicked() {
				m.commitEditProject()
			}
		},
	)
}

// commitEditProject 把改名 / 新描述写回磁盘，并同步已打开的标签页。
func (m *appModel) commitEditProject() {
	name := strings.TrimSpace(m.editName)
	if name == "" {
		return
	}
	old := m.editProjectOldName
	if name != old {
		for _, meta := range m.projects {
			if meta.Name == name {
				m.toaster.Show(i18n.T("toast.createFailed"), fmt.Sprintf(i18n.T("toast.duplicateProject"), name), ui.ToastError)
				return
			}
		}
	}
	p, err := storage.LoadProject(old)
	if err != nil {
		m.editProjectOpen = false
		m.toaster.Show(i18n.T("toast.openProjectFailed"), err.Error(), ui.ToastError)
		return
	}
	p.Name = name
	p.Description = m.editDesc
	if err := storage.UpdateProject(old, p); err != nil {
		m.toaster.Show(i18n.T("toast.saveFailed"), err.Error(), ui.ToastError)
		return
	}
	for _, s := range m.tabs {
		if s.project.Name == old {
			s.project.Name = name
			s.project.Description = m.editDesc
		}
	}
	m.editProjectOpen = false
	m.reloadProjects()
	m.toaster.Show(i18n.T("toast.saved"), name, ui.ToastSuccess)
}

// deleteProjectModal 删除项目前的二次确认。
func (m *appModel) deleteProjectModal(c ui.C) {
	if !m.deleteProjectOpen {
		return
	}
	ui.ModalWith(c, &m.deleteProjectOpen, i18n.T("dialog.confirmDeleteProject"),
		ui.ModalSpec{Description: fmt.Sprintf(i18n.T("dialog.confirmDeleteProjectDesc"), m.deleteProjectName)}, nil,
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				m.deleteProjectOpen = false
			}
			if ui.Button(c, i18n.T("common.delete")).Destructive().Clicked() {
				m.commitDeleteProject()
			}
		},
	)
}

// commitDeleteProject 删除项目目录并关闭其已打开的标签页。
func (m *appModel) commitDeleteProject() {
	name := m.deleteProjectName
	m.deleteProjectOpen = false
	if err := storage.DeleteProject(name); err != nil {
		m.toaster.Show(i18n.T("toast.deleteProjectFailed"), err.Error(), ui.ToastError)
		return
	}
	for i := len(m.tabs) - 1; i >= 0; i-- {
		if m.tabs[i].project.Name == name {
			m.closeTab(i)
		}
	}
	m.reloadProjects()
	m.toaster.Show(i18n.T("toast.projectDeleted"), name, ui.ToastSuccess)
}

// ---------------------------------------------------------------------------
// 手风琴第三项：保存目录说明
// ---------------------------------------------------------------------------

// storageInfo 展示保存位置说明；点击路径在系统文件管理器里打开该目录。
func (m *appModel) storageInfo(c ui.C) {
	theme := c.Theme()
	ui.Column(c).Gap(8).Children(func() {
		ui.Text(c, i18n.T("settings.storageHint")).Muted()
		dir, err := storage.DataDir()
		if err != nil {
			ui.Text(c, err.Error()).Color(theme.MutedForeground).Small()
			return
		}
		link := ui.Text(c, dir).Color(theme.Primary).Small().
			Hit(ui.ID(c), i18n.T("settings.openDataDir")).
			Tooltip(i18n.T("settings.openDataDir")).
			Hover(ui.Look{}.Opacity(0.75))
		if link.Clicked() {
			m.openDataDir()
		}
	})
}

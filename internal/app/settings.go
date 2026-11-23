package app

// 本文件是全局设置：界面语言切换、系统语言检测，以及抽屉底部的作者与版本信息。
// 语言偏好保存在 ~/Documents/gorester/config.json；"" 表示跟随系统语言。

import (
	"fmt"

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

// settingsDrawer 绘制设置抽屉，布局与环境管理抽屉一致：可滚动主体 + 固定底部。
func (m *appModel) settingsDrawer(c ui.C) {
	theme := c.Theme()
	ui.Drawer(c, &m.settingsOpen, i18n.T("settings.title"), ui.DrawerSpec{Size: 480},
		func() {
			ui.Scroll(c).Fill().Pad(16).Children(func() {
				ui.Column(c).Gap(12).Children(func() {
					// —— 界面语言 ——
					ui.Text(c, i18n.T("settings.language")).Weight(gfx.WeightSemibold)
					ui.Text(c, i18n.T("settings.languageHint")).Muted()
					sel := ui.Select(c, &m.settingsLang, languageOptions, languageLabel).FillWidth().Label(i18n.T("settings.language"))
					if sel.Changed() {
						m.applyLanguage(m.settingsLang)
					}
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
}

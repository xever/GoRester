package app

import (
	"math"
	"strings"
	"testing"
	"unicode"

	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/storage"
)

// TestLanguageSwitch 验证设置里的语言下拉能实时切换界面并持久化偏好。
func TestLanguageSwitch(t *testing.T) {
	useTempHome(t)
	oldLang := i18n.Current()
	t.Cleanup(func() { i18n.Set(oldLang) })
	i18n.Set(i18n.Chinese)

	m := newAppModel()
	driver := openDriver(t, m)
	if !hasLabel(driver, "我的项目") {
		t.Fatal("初始应为中文界面")
	}

	// 打开右上角的设置抽屉。
	clickLabel(t, driver, "设置")
	if !hasLabel(driver, "界面语言") {
		t.Fatal("设置抽屉应显示界面语言选项")
	}
	if !hasLabel(driver, "xever @GitHub") || !hasLabel(driver, "版本 v0.1.0") {
		t.Fatal("设置抽屉底部应显示作者信息与版本号")
	}
	if _, ok := findHit(driver, "xever @GitHub"); !ok {
		t.Fatal("作者信息应可点击")
	}

	// 展开语言下拉，选择 English。
	clickLabel(t, driver, "界面语言")
	if !hasLabel(driver, "English") {
		t.Fatal("语言下拉应包含 English 选项")
	}
	clickLabel(t, driver, "English")
	if !hasLabel(driver, "My Projects") {
		t.Fatal("选择 English 后主页应立即切换为英文")
	}
	if !hasLabel(driver, "Version v0.1.0") {
		t.Fatal("英文界面应显示英文版本号")
	}
	if pref := storage.LoadPreferences(); pref.Language != "en" {
		t.Fatalf("语言偏好未保存，got %q", pref.Language)
	}

	// 设置弹窗仍然打开，直接切回中文并确认偏好更新。
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "Interface Language")
	clickLabel(t, driver, "简体中文")
	if !hasLabel(driver, "我的项目") {
		t.Fatal("选择简体中文后主页应切回中文")
	}
	if pref := storage.LoadPreferences(); pref.Language != "zh" {
		t.Fatalf("语言偏好未更新，got %q", pref.Language)
	}
}

// TestLanguageQuickMenu 验证右上角地球图标的快捷菜单可以切换语言。
func TestLanguageQuickMenu(t *testing.T) {
	useTempHome(t)
	oldLang := i18n.Current()
	t.Cleanup(func() { i18n.Set(oldLang) })
	i18n.Set(i18n.Chinese)

	m := newAppModel()
	driver := openDriver(t, m)
	if _, ok := findHit(driver, "快速切换语言"); !ok {
		t.Fatal("设置按钮右侧应有语言快捷按钮")
	}

	clickLabel(t, driver, "快速切换语言")
	if !hasLabel(driver, "English") {
		t.Fatal("快捷菜单应列出 English 选项")
	}
	clickLabel(t, driver, "English")
	if !hasLabel(driver, "My Projects") {
		t.Fatal("快捷菜单选择 English 后界面应立即变英文")
	}
	if pref := storage.LoadPreferences(); pref.Language != "en" {
		t.Fatalf("语言偏好未保存，got %q", pref.Language)
	}
}

// TestEnglishWorkspaceHasNoChineseLabels 验证英文界面下工作区没有遗漏的中文文案。
func TestEnglishWorkspaceHasNoChineseLabels(t *testing.T) {
	useTempHome(t)
	oldLang := i18n.Current()
	t.Cleanup(func() { i18n.Set(oldLang) })
	i18n.Set(i18n.English)

	m := newAppModel()
	m.createProject("Demo", "Test")
	s := m.tabs[0]
	iface := &model.Interface{ID: model.NewID(), Name: "List Users", Method: "GET", Path: "/users"}
	s.project.Interfaces = append(s.project.Interfaces, iface)
	s.markTreeDirty()
	s.openInterfaceTab(iface, false)

	driver := openDriver(t, m)
	for _, label := range driver.Snapshot().Labels {
		for _, r := range label.Value {
			if unicode.In(r, unicode.Han) {
				t.Errorf("英文界面出现未翻译文案 %q", label.Value)
				break
			}
		}
	}
	if !hasLabel(driver, "Send") || !hasLabel(driver, "Save as Case") {
		t.Fatal("英文界面应显示 Send / Save as Case")
	}
}

// TestSettingsFooterCentered 验证作者信息与版本号在抽屉底部水平居中对齐。
func TestSettingsFooterCentered(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	m.openSettings()
	driver := openDriver(t, m)

	var authorX, versionX float32
	var authorFound, versionFound bool
	for _, label := range driver.Snapshot().Labels {
		center := label.Bounds.X + label.Bounds.Width/2
		switch {
		case strings.Contains(label.Value, "xever @GitHub"):
			authorX, authorFound = center, true
		case strings.HasPrefix(label.Value, "版本 v"):
			versionX, versionFound = center, true
		}
	}
	if !authorFound || !versionFound {
		t.Fatalf("未找到底部文案：author=%v version=%v", authorFound, versionFound)
	}
	if math.Abs(float64(authorX-versionX)) > 2 {
		t.Fatalf("作者与版本号未居中对齐：x=%.1f vs %.1f", authorX, versionX)
	}
}

// TestStartupLanguageUsesSavedPreference 验证启动时读取磁盘上的语言偏好。
func TestStartupLanguageUsesSavedPreference(t *testing.T) {
	useTempHome(t)
	oldLang := i18n.Current()
	t.Cleanup(func() { i18n.Set(oldLang) })

	if err := storage.SavePreferences(storage.Preferences{Language: "en"}); err != nil {
		t.Fatal(err)
	}
	m := newAppModel()
	m.initLanguage()
	if i18n.Current() != i18n.English {
		t.Fatalf("启动语言应为 English，got %q", i18n.Current())
	}
}

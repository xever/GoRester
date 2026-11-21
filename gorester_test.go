package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 让所有测试使用临时目录作为数据根目录，避免污染真实 ~/gorester。
func useTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// openDriver 在无窗口模式下渲染首页，返回 driver。
func openDriver(t *testing.T, m *appModel) *app.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := app.NewDriver(ui.Configure(page, app.Config{Width: 1280, Height: 820}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func hasLabel(driver *app.Driver, want string) bool {
	for _, l := range driver.Snapshot().Labels {
		if strings.Contains(l.Value, want) {
			return true
		}
	}
	return false
}

// TestHomeRenders 验证主页能正常渲染（无崩溃，且显示关键文字）。
func TestHomeRenders(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	driver := openDriver(t, m)

	if !hasLabel(driver, "我的项目") {
		t.Fatal("主页应显示「我的项目」")
	}
	if !hasLabel(driver, "新建") {
		t.Fatal("主页应显示「新建」卡片")
	}
}

// TestStorageRoundTrip 验证项目的保存 / 列表 / 读取 / 删除。
func TestStorageRoundTrip(t *testing.T) {
	useTempHome(t)

	p := NewProject("测试项目", "一个测试项目")
	if err := SaveProject(p); err != nil {
		t.Fatalf("保存项目失败: %v", err)
	}

	metas, err := ListProjects()
	if err != nil {
		t.Fatalf("列出项目失败: %v", err)
	}
	if len(metas) != 1 || metas[0].Name != "测试项目" {
		t.Fatalf("项目列表不正确: %+v", metas)
	}

	loaded, err := LoadProject("测试项目")
	if err != nil {
		t.Fatalf("读取项目失败: %v", err)
	}
	if loaded.Description != "一个测试项目" {
		t.Fatalf("项目说明不匹配: %q", loaded.Description)
	}

	// 数据应写入 ~/gorester/测试项目/project.json。
	if _, err := os.Stat(filepath.Join(os.Getenv("HOME"), "gorester", "测试项目", "project.json")); err != nil {
		t.Fatalf("应存在 project.json: %v", err)
	}

	if err := DeleteProject("测试项目"); err != nil {
		t.Fatalf("删除项目失败: %v", err)
	}
}

// TestOpenAPIRoundTrip 验证 OpenAPI 导出再导入不会丢失接口。
func TestOpenAPIRoundTrip(t *testing.T) {
	p := NewProject("API 项目", "")
	iface := &Interface{ID: newID(), Name: "获取用户", Method: "GET", Path: "/users", Params: []KeyValue{{Key: "page", Value: "1", Enabled: true}}}
	p.Folders = []*Folder{{ID: newID(), Name: "用户", Interfaces: []*Interface{iface}}}

	doc := ExportOpenAPI(p)
	data, err := doc.Marshal()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	parsed, err := ParseOpenAPI(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	p2 := NewProject("导入", "")
	ImportOpenAPI(p2, parsed, "导入")
	all := p2.allInterfaces()
	if len(all) != 1 {
		t.Fatalf("导入后应有 1 个接口，实际 %d", len(all))
	}
	if all[0].Method != "GET" || all[0].Path != "/users" {
		t.Fatalf("导入接口不匹配: %s %s", all[0].Method, all[0].Path)
	}
}

// TestHighlightJSON 验证 JSON 高亮能生成正确的行数。
func TestHighlightJSON(t *testing.T) {
	theme := ui.LightTheme()
	lines, ok := buildResponseLines([]byte(`{"a":1,"b":[true,null]}`), "application/json", theme)
	if !ok {
		t.Fatal("合法 JSON 应被高亮")
	}
	if len(lines) < 3 {
		t.Fatalf("格式化后的 JSON 应有多行，实际 %d", len(lines))
	}
}

// TestProjectRenders 验证打开项目后工作区（侧栏树 + 工作区）能正常渲染。
func TestProjectRenders(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	m.createProject("演示项目", "演示")

	s := m.tabs[0]
	iface := &Interface{ID: newID(), Name: "获取用户", Method: "GET", Path: "/users"}
	cs := &Case{ID: newID(), Name: "正常用例", Method: "GET", Path: "/users"}
	iface.Cases = append(iface.Cases, cs)
	f := &Folder{ID: newID(), Name: "用户模块", Interfaces: []*Interface{iface}}
	s.project.Folders = append(s.project.Folders, f)
	s.expanded[f.ID] = true
	s.markTreeDirty()

	driver := openDriver(t, m)
	if !hasLabel(driver, "演示项目") {
		t.Fatal("应显示项目名")
	}
	if !hasLabel(driver, "用户模块") {
		t.Fatal("应显示文件夹")
	}
	if !hasLabel(driver, "获取用户") {
		t.Fatal("应显示接口")
	}
	// 默认是「项目概览」标签页。
	if !hasLabel(driver, "接口数量") {
		t.Fatal("概览应显示接口数量")
	}

	// 选中接口后，应显示发送按钮。
	s.selectEntry(treeEntry{kind: nodeInterface, id: iface.ID, name: "获取用户", method: "GET", iface: iface})
	if err := driver.Paint(); err != nil {
		t.Fatalf("重绘失败: %v", err)
	}
	if !hasLabel(driver, "发送") {
		t.Fatal("编辑接口时应显示发送按钮")
	}
}

// findHit 按标签前缀查找命中区域，找不到返回 false。
func findHit(driver *app.Driver, label string) (gfx.Hit, bool) {
	for _, h := range driver.Snapshot().Hits {
		if strings.HasPrefix(h.Label, label) {
			return h, true
		}
	}
	return gfx.Hit{}, false
}

// clickLabel 点击指定标签的控件中心。
func clickLabel(t *testing.T, driver *app.Driver, label string) {
	t.Helper()
	h, ok := findHit(driver, label)
	if !ok {
		t.Fatalf("找不到控件 %q", label)
	}
	if err := driver.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
		t.Fatalf("点击 %q 失败: %v", label, err)
	}
}

// TestCreateProjectFlow 模拟真实交互：点击新建卡片、填名、创建，验证项目标签出现。
func TestCreateProjectFlow(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	driver := openDriver(t, m)

	// 点击「新建」卡片，应弹出新建项目弹窗。
	clickLabel(t, driver, "新建项目")
	if _, ok := findHit(driver, "项目名称"); !ok {
		t.Fatal("应显示项目名称输入框")
	}

	// 聚焦项目名称输入框并输入。
	clickLabel(t, driver, "项目名称")
	if err := driver.Type("我的接口项目"); err != nil {
		t.Fatalf("输入失败: %v", err)
	}

	// 点击「创建」。
	clickLabel(t, driver, "创建")
	if !hasLabel(driver, "我的接口项目") {
		t.Fatal("创建后应出现项目标签")
	}
}

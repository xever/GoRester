package app

import (
	"strings"
	"testing"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/model"
)

// 让所有测试使用临时目录作为数据根目录，避免污染真实 ~/gorester。
func useTempHome(t *testing.T) {
	t.Helper()
	t.Setenv("HOME", t.TempDir())
}

// openDriver 在无窗口模式下渲染应用，返回 driver。
func openDriver(t *testing.T, m *appModel) *vivaapp.Driver {
	t.Helper()
	page := ui.NewPage(ui.PageConfig{View: m.page})
	driver, err := vivaapp.NewDriver(ui.Configure(page, vivaapp.Config{Width: 1280, Height: 820}), nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(driver.Close)
	return driver
}

func hasLabel(driver *vivaapp.Driver, want string) bool {
	for _, l := range driver.Snapshot().Labels {
		if strings.Contains(l.Value, want) {
			return true
		}
	}
	return false
}

// findHit 按标签前缀查找命中区域，找不到返回 false。
func findHit(driver *vivaapp.Driver, label string) (gfx.Hit, bool) {
	for _, h := range driver.Snapshot().Hits {
		if strings.HasPrefix(h.Label, label) {
			return h, true
		}
	}
	return gfx.Hit{}, false
}

// clickLabel 点击指定标签的控件中心。
func clickLabel(t *testing.T, driver *vivaapp.Driver, label string) {
	t.Helper()
	h, ok := findHit(driver, label)
	if !ok {
		t.Fatalf("找不到控件 %q", label)
	}
	if err := driver.Click(h.Bounds.X+h.Bounds.Width/2, h.Bounds.Y+h.Bounds.Height/2); err != nil {
		t.Fatalf("点击 %q 失败: %v", label, err)
	}
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

// TestProjectRenders 验证打开项目后工作区（侧栏树 + 工作区）能正常渲染。
func TestProjectRenders(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	m.createProject("演示项目", "演示")

	s := m.tabs[0]
	iface := &model.Interface{ID: model.NewID(), Name: "获取用户", Method: "GET", Path: "/users"}
	cs := &model.Case{ID: model.NewID(), Name: "正常用例", Method: "GET", Path: "/users"}
	iface.Cases = append(iface.Cases, cs)
	f := &model.Folder{ID: model.NewID(), Name: "用户模块", Interfaces: []*model.Interface{iface}}
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

// TestJSONBodyEditorRenders 验证 json 请求体代码编辑器能正常渲染。
func TestJSONBodyEditorRenders(t *testing.T) {
	useTempHome(t)
	m := newAppModel()
	m.createProject("测试", "")
	s := m.tabs[0]

	iface := &model.Interface{ID: model.NewID(), Name: "创建用户", Method: "POST", Path: "/users",
		Body: model.Body{Mode: "json", Raw: `{"name":"张三"}`}}
	s.project.Interfaces = append(s.project.Interfaces, iface)
	s.markTreeDirty()
	s.openInterfaceTab(iface, false)
	s.reqTab = 1     // Body 标签
	s.bodyFormat = 3 // json

	driver := openDriver(t, m)
	if !hasLabel(driver, "格式化") {
		t.Fatal("json 编辑器应显示「格式化」按钮")
	}
	if !hasLabel(driver, "压缩") {
		t.Fatal("json 编辑器应显示「压缩」按钮")
	}
}

package app

import (
	"testing"

	"gorester/internal/i18n"
	"gorester/internal/model"
)

// withTreeSession 建一个带「文件夹 / 接口 / 用例」各一个的临时项目会话。
func withTreeSession(t *testing.T) (*appModel, *projectSession, *model.Folder, *model.Interface, *model.Case) {
	t.Helper()
	useTempHome(t)
	oldLang := i18n.Current()
	t.Cleanup(func() { i18n.Set(oldLang) })
	i18n.Set(i18n.Chinese)

	m := newAppModel()
	m.createProject("演示项目", "")
	s := m.tabs[0]
	cs := &model.Case{ID: model.NewID(), Name: "正常用例", Method: "GET"}
	iface := &model.Interface{ID: model.NewID(), Name: "获取用户", Method: "GET", Cases: []*model.Case{cs}}
	f := &model.Folder{ID: model.NewID(), Name: "用户模块", Interfaces: []*model.Interface{iface}}
	s.project.Folders = append(s.project.Folders, f)
	s.expanded[f.ID] = true
	s.expanded[iface.ID] = true
	s.markTreeDirty()
	return m, s, f, iface, cs
}

// TestDeleteAsksForConfirmation 验证树节点删除先弹确认框，取消不删、确认才删。
func TestDeleteAsksForConfirmation(t *testing.T) {
	m, s, f, _, _ := withTreeSession(t)
	driver := openDriver(t, m)

	s.confirmDelete(treeEntry{kind: nodeFolder, id: f.ID, name: f.Name, folder: f})
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver, "删除文件夹") || !hasLabel(driver, "确定删除文件夹「用户模块」") {
		t.Fatal("删除前应显示带节点名称的确认弹窗")
	}
	if s.project.FindFolder(f.ID) == nil {
		t.Fatal("弹窗打开时不应提前删除")
	}

	// 取消：节点仍在。
	clickLabel(t, driver, "取消")
	if s.project.FindFolder(f.ID) == nil {
		t.Fatal("取消后节点应保留")
	}

	// 再打开并确认：节点被删除。
	s.confirmDelete(treeEntry{kind: nodeFolder, id: f.ID, name: f.Name, folder: f})
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "删除")
	if s.project.FindFolder(f.ID) != nil {
		t.Fatal("确认后节点应被删除")
	}
}

// TestRenameTreeNode 验证重命名对接口生效，并同步工作区标签标题。
func TestRenameTreeNode(t *testing.T) {
	m, s, _, iface, _ := withTreeSession(t)
	s.openInterfaceTab(iface, false)
	driver := openDriver(t, m)

	s.startRename(treeEntry{kind: nodeInterface, id: iface.ID, name: iface.Name, iface: iface})
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "新名称")
	if err := driver.Type("获取用户列表"); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "重命名")
	if iface.Name != "获取用户列表" {
		t.Fatalf("接口名称未更新，got %q", iface.Name)
	}
	if tab := s.currentTab(); tab.name != "获取用户列表" {
		t.Fatalf("标签标题未同步，got %q", tab.name)
	}
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	if !hasLabel(driver, "获取用户列表") {
		t.Fatal("重命名后目录树应显示新名称")
	}

	// 用例同样可以重命名。
	cs := iface.Cases[0]
	s.startRename(treeEntry{kind: nodeCase, id: cs.ID, name: cs.Name, iface: iface, case_: cs})
	if err := driver.Paint(); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "新名称")
	if err := driver.Type("异常用例"); err != nil {
		t.Fatal(err)
	}
	clickLabel(t, driver, "重命名")
	if cs.Name != "异常用例" {
		t.Fatalf("用例名称未更新，got %q", cs.Name)
	}
}

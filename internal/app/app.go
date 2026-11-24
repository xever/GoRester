package app

// 本文件集中定义应用层的所有状态类型与构造器：
//   - appModel：应用顶层状态（项目列表、已打开的标签页、全局弹窗等）；
//   - projectSession：每个已打开项目的运行时状态；
//   - treeEntry / workTab：目录树扁平行与工作区标签页。
//
// 其他文件只负责“行为”（渲染、增删改、发送请求等），状态定义统一放在这里，
// 便于一眼看清应用持有哪些数据。

import (
	"context"
	"sync"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/codeeditor"

	"gorester/internal/highlight"
	"gorester/internal/httpclient"
	"gorester/internal/i18n"
	"gorester/internal/model"
	"gorester/internal/storage"
)

// ---------------------------------------------------------------------------
// 节点 / 标签页类型常量
// ---------------------------------------------------------------------------

// 树节点种类。
const (
	nodeFolder    = iota // 文件夹
	nodeInterface        // 接口
	nodeCase             // 用例
)

// 工作区标签页种类。
const (
	tabOverview  = iota // 项目概览
	tabInterface        // 编辑接口
	tabCase             // 编辑用例
)

// ---------------------------------------------------------------------------
// 应用顶层状态
// ---------------------------------------------------------------------------

// appModel 是整个应用的顶层状态，持有项目列表和已打开的标签页。
type appModel struct {
	projects []storage.ProjectMeta // 主页展示的项目摘要
	tabs     []*projectSession     // 已打开的项目标签页
	active   int                   // 当前标签页下标，-1 表示主页

	// 主页「新建项目」弹窗。
	newProjectOpen bool
	newName        string
	newDesc        string

	// 全局设置弹窗与界面语言偏好。langPref 为 "" 表示跟随系统。
	settingsOpen bool
	settingsLang string
	langPref     string
	langMenuOpen bool // 右上角地球图标的语言快捷菜单

	toaster ui.Toaster // 全局轻提示

	// application 是运行中的应用（RunApplication 启动回调里保存），用于把
	// 会阻塞的操作（文件对话框等）投递到本轮绘制之后执行。
	application *vivaapp.Application
	// inv 用于后台工作完成后请求一次重绘。
	inv vivaapp.Invalidator
}

// newAppModel 创建应用状态，并加载磁盘上已有的项目列表。
func newAppModel() *appModel {
	m := &appModel{active: -1, langPref: storage.LoadPreferences().Language}
	m.reloadProjects()
	return m
}

// ---------------------------------------------------------------------------
// 树节点 / 标签页类型
// ---------------------------------------------------------------------------

// treeEntry 是左侧目录树的一行（扁平化后的结果）。
type treeEntry struct {
	kind   int              // nodeFolder / nodeInterface / nodeCase
	depth  int              // 缩进层级
	id     string           // 节点 ID（接口 / 用例 / 文件夹）
	name   string           // 显示名
	method string           // 接口 / 用例的 HTTP 方法
	folder *model.Folder    // nodeFolder 指向的文件夹
	iface  *model.Interface // nodeInterface 指向的接口
	case_  *model.Case      // nodeCase 指向的用例
}

// workTab 是右侧工作区顶部的一个标签页。
type workTab struct {
	kind  int    // tabOverview / tabInterface / tabCase
	id    string // 接口或用例的 ID
	name  string // 标签页标题
	isNew bool   // 是否为「尚未保存的新接口」

	parentID string           // 用例所属接口的 ID（仅 tabCase 用）
	iface    *model.Interface // tabInterface 的编辑对象
	case_    *model.Case      // tabCase 的编辑对象
}

// displayName 返回标签页当前语言下的标题。概览标签的名字随界面语言实时变化。
func (t *workTab) displayName() string {
	if t.kind == tabOverview {
		return i18n.T("workspace.overview")
	}
	return t.name
}

// ---------------------------------------------------------------------------
// 项目会话
// ---------------------------------------------------------------------------

// projectSession 保存一个已打开项目的运行时状态。
// 每个项目复用一个 HTTP Client（见 httpclient 包）。
type projectSession struct {
	app     *appModel
	project *model.Project
	client  *httpclient.Client

	// —— 左侧目录树 ——
	tree           ui.TreeState
	flat           []treeEntry      // 扁平化后的可见行（缓存）
	expanded       map[string]bool  // 节点 ID -> 是否展开
	treeDirty      bool             // 结构变化后置 true，触发重建
	selectedID     string           // 当前选中的节点 ID
	labelFn        func(int) string // 树行名称（构造时绑定，避免每帧分配）
	contentFn      func(int)        // 树行内容（同上）
	sidebarSearch  string           // 搜索框文本
	sidebarFilter  string           // 按方法过滤（空 = 不过滤）
	newMenuOpen    bool             // 「新建」下拉菜单
	filterMenuOpen bool             // 「过滤」下拉菜单
	methodMenuOpen bool             // 方法选择下拉菜单

	// —— 右侧工作区 ——
	workTabs   []*workTab
	activeWork int

	// 左右分栏的拖拽分隔条位置。
	split ui.SplitterState
	// 右工作区上下（请求配置 / 响应结果）的分隔条，默认 3:2。
	vsplit ui.SplitterState

	// —— 环境 ——
	envIndex    int              // 当前选中的环境下标
	envIdxCache []int            // 环境下拉框的下标缓存（数量变化时重建）
	envLabelFn  func(int) string // 环境名称标签（构造时绑定）

	// —— 请求发送（异步）——
	send     *ui.EffectState[*httpclient.Result, int]
	sendKey  int
	sendOpts httpclient.RequestOptions
	sendMu   sync.Mutex
	sending  bool
	response *httpclient.Result     // 最近一次响应（供展示）
	lastReq  httpclient.RequestInfo // 最近一次实际发送的请求（供「请求」标签展示）

	// 响应体渲染缓存（语法高亮后的行）。
	respLines       []highlight.Line
	respHighlighted bool
	respList        ui.ListState // 响应体列表的滚动/焦点状态
	respFormat      int          // 0 Pretty 1 Raw
	respLang        string       // 语言下拉（展示用）
	respEncoding    string       // 编码下拉（展示用）
	respSearch      string       // 响应内搜索文本
	respFindOpen    bool         // 是否显示搜索栏

	// —— 弹窗状态 ——
	newFolderOpen   bool
	newFolderName   string
	newFolderParent string // 父文件夹 ID，"" = 根目录
	saveDialogOpen  bool   // 保存「新接口」时选择名字与文件夹
	saveName        string
	saveParent      string
	draftParent     string // 新建接口草稿的默认目标文件夹 ID
	saveCaseOpen    bool
	saveCaseName    string
	saveCaseTarget  *model.Interface // 要另存为用例的接口

	// —— 环境抽屉 ——
	envDrawerOpen     bool
	envEditIndex      int // 抽屉里正在编辑的环境下标
	envTimeoutEdit    string
	envVarsEdit       []model.KeyValue
	envHeadersEdit    []model.KeyValue
	globalVarsEdit    []model.KeyValue
	globalHeadersEdit []model.KeyValue

	// —— 请求体编辑面板 ——
	bodyFormat int // Body 子 Tab 下标
	reqTab     int // 请求配置 Tab 下标：0 Params 1 Body 2 Cookie 3 Header 4 Auth
	respTab    int // 响应结果 Tab 下标：0 Body 1 Cookie 2 Header

	// —— 请求体代码编辑器（json / xml / raw 语法高亮）——
	bodyEditor       *text.Editor      // 编辑器的文本模型
	bodyEditorTarget *model.Body       // 编辑器当前绑定的请求体
	bodyCode         codeeditor.Editor // 代码编辑器视图
}

// newProjectSession 为一个项目创建会话。
func newProjectSession(a *appModel, p *model.Project) *projectSession {
	s := &projectSession{
		app:      a,
		project:  p,
		client:   httpclient.NewClient(0),
		expanded: map[string]bool{},
		split:    ui.SplitterState{Pos: 280},
		vsplit:   ui.SplitterState{Ratio: 0.6},
	}

	// 打开时默认展开第一层文件夹，方便看到内容。
	for _, f := range p.Folders {
		s.expanded[f.ID] = true
	}

	// 默认选中当前环境（第一个）。
	if len(p.Environments) == 0 {
		p.Environments = model.NewProject(p.Name, p.Description).Environments
	}

	// 异步发送请求的 Effect：key 是发送计数，每次发送 +1。
	// run 在页面之外执行，只能读快照，不能碰 ui.C / Box。
	s.send = ui.NewEffectKeyed(func(ctx context.Context, key int) (*httpclient.Result, error) {
		s.sendMu.Lock()
		opts := s.sendOpts
		s.sendMu.Unlock()
		return s.client.Do(opts), nil
	}).Sequential()

	// 工作区默认打开「项目概览」标签页。
	s.workTabs = []*workTab{{kind: tabOverview, name: i18n.T("workspace.overview")}}
	s.treeDirty = true

	// 绑定树的行描述 / 名称 / 内容回调（避免每次 paint 重新分配闭包）。
	s.tree.Node = s.nodeAt
	s.labelFn = s.labelAt
	s.contentFn = s.contentAt
	s.envLabelFn = s.envNameAt

	// 请求体代码编辑器：文本模型 + 编辑命令 + 原生输入。
	// 原生输入用「全透明文字」：macOS 原生编辑器只接收输入（打字/输入法/剪贴板），
	// 不绘制文字/光标/选区，全部由 codeeditor 自己绘制（带语法高亮），所见即所得。
	s.bodyEditor = text.NewEditor("")
	s.bodyCode.Document = s.bodyEditor
	s.bodyCode.Key = s.bodyEditorKey
	s.bodyCode.Pointer = s.bodyEditorPointer
	s.bodyCode.Text = s.bodyEditorText
	s.bodyCode.Snapshot.LineNumbers = true
	s.bodyCode.Snapshot.Wrap = true
	s.bodyCode.Snapshot.ShowCaret = true
	return s
}

package main

// 本文件定义应用的顶层状态（appModel）、每个已打开项目的运行时状态
// （projectSession）、工作区标签页（workTab）以及树形结构的扁平化逻辑。

import (
	"context"
	"strings"
	"sync"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
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
// 树节点
// ---------------------------------------------------------------------------

// treeEntry 是左侧目录树的一行（扁平化后的结果）。
type treeEntry struct {
	kind   int        // nodeFolder / nodeInterface / nodeCase
	depth  int        // 缩进层级
	id     string     // 节点 ID（接口 / 用例 / 文件夹）
	name   string     // 显示名
	method string     // 接口 / 用例的 HTTP 方法
	folder *Folder    // nodeFolder 指向的文件夹
	iface  *Interface // nodeInterface 指向的接口
	case_  *Case      // nodeCase 指向的用例
}

// ---------------------------------------------------------------------------
// 工作区标签页
// ---------------------------------------------------------------------------

// workTab 是右侧工作区顶部的一个标签页。
type workTab struct {
	kind  int    // tabOverview / tabInterface / tabCase
	id    string // 接口或用例的 ID
	name  string // 标签页标题
	isNew bool   // 是否为「尚未保存的新接口」

	parentID string     // 用例所属接口的 ID（仅 tabCase 用）
	iface    *Interface // tabInterface 的编辑对象
	case_    *Case      // tabCase 的编辑对象
}

// ---------------------------------------------------------------------------
// 项目会话
// ---------------------------------------------------------------------------

// projectSession 保存一个已打开项目的运行时状态。
// 每个项目复用一个 HTTP Client（见 httpclient.go）。
type projectSession struct {
	app     *appModel
	project *Project
	client  *Client

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
	send     *ui.EffectState[*Result, int]
	sendKey  int
	sendOpts RequestOptions
	sendMu   sync.Mutex
	sending  bool
	response *Result // 最近一次响应（供展示）

	// 响应体渲染缓存（语法高亮后的行）。
	respLines       []respLine
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
	saveCaseTarget  *Interface // 要另存为用例的接口

	// —— 环境抽屉 ——
	envDrawerOpen     bool
	envEditIndex      int // 抽屉里正在编辑的环境下标
	envTimeoutEdit    string
	envVarsEdit       []KeyValue
	envHeadersEdit    []KeyValue
	globalVarsEdit    []KeyValue
	globalHeadersEdit []KeyValue

	// —— 请求体编辑面板 ——
	bodyFormat int // Body 子 Tab 下标
	reqTab     int // 请求配置 Tab 下标：0 Params 1 Body 2 Cookie 3 Header 4 Auth
	respTab    int // 响应结果 Tab 下标：0 Body 1 Cookie 2 Header
}

// newProjectSession 为一个项目创建会话。
func newProjectSession(app *appModel, p *Project) *projectSession {
	s := &projectSession{
		app:      app,
		project:  p,
		client:   NewClient(0),
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
		p.Environments = NewProject(p.Name, p.Description).Environments
	}

	// 异步发送请求的 Effect：key 是发送计数，每次发送 +1。
	// run 在页面之外执行，只能读快照，不能碰 ui.C / Box。
	s.send = ui.NewEffectKeyed(func(ctx context.Context, key int) (*Result, error) {
		s.sendMu.Lock()
		opts := s.sendOpts
		s.sendMu.Unlock()
		return s.client.Do(opts), nil
	}).Sequential()

	// 工作区默认打开「项目概览」标签页。
	s.workTabs = []*workTab{{kind: tabOverview, name: "项目概览"}}
	s.treeDirty = true

	// 绑定树的行描述 / 名称 / 内容回调（避免每次 paint 重新分配闭包）。
	s.tree.Node = s.nodeAt
	s.labelFn = s.labelAt
	s.contentFn = s.contentAt
	s.envLabelFn = s.envNameAt
	return s
}

// ---------------------------------------------------------------------------
// 树结构
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
func (s *projectSession) appendFolders(folders []*Folder, depth int, filtering bool) {
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
func (s *projectSession) matchesInterface(iface *Interface) bool {
	if s.sidebarFilter != "" && !strings.EqualFold(iface.Method, s.sidebarFilter) {
		return false
	}
	if s.sidebarSearch != "" && !strings.Contains(strings.ToLower(iface.Name), strings.ToLower(s.sidebarSearch)) {
		return false
	}
	return true
}

// folderMatches 判断文件夹（或其后代）是否含有匹配的接口。
func (s *projectSession) folderMatches(f *Folder) bool {
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
func (s *projectSession) appendInterface(iface *Interface, depth int) {
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
// 项目内的查找
// ---------------------------------------------------------------------------

// allInterfaces 返回项目里所有接口（根目录 + 所有文件夹，递归）。
func (p *Project) allInterfaces() []*Interface {
	var out []*Interface
	out = append(out, p.Interfaces...)
	var walk func(fs []*Folder)
	walk = func(fs []*Folder) {
		for _, f := range fs {
			out = append(out, f.Interfaces...)
			walk(f.Folders)
		}
	}
	walk(p.Folders)
	return out
}

// findInterface 按 ID 查找接口，找不到返回 nil。
func (p *Project) findInterface(id string) *Interface {
	for _, i := range p.Interfaces {
		if i.ID == id {
			return i
		}
	}
	for _, i := range p.allInterfaces() {
		if i.ID == id {
			return i
		}
	}
	return nil
}

// findCase 按 ID 查找用例，找不到返回 nil。
func (p *Project) findCase(id string) *Case {
	for _, iface := range p.allInterfaces() {
		for _, c := range iface.Cases {
			if c.ID == id {
				return c
			}
		}
	}
	return nil
}

// findFolder 按 ID 递归查找文件夹，找不到返回 nil。
func (p *Project) findFolder(id string) *Folder {
	var walk func(fs []*Folder) *Folder
	walk = func(fs []*Folder) *Folder {
		for _, f := range fs {
			if f.ID == id {
				return f
			}
			if r := walk(f.Folders); r != nil {
				return r
			}
		}
		return nil
	}
	return walk(p.Folders)
}

// ---------------------------------------------------------------------------
// 文件夹选择器
// ---------------------------------------------------------------------------

// folderTarget 是「保存到哪个文件夹」下拉框里的一项。
type folderTarget struct {
	path   string  // 显示路径，如 "A/B"
	folder *Folder // nil 表示项目根目录
}

// folderTargets 返回项目根目录 + 所有文件夹（用于下拉选择）。
func (p *Project) folderTargets() []folderTarget {
	out := []folderTarget{{path: "项目根目录"}}
	var walk func(fs []*Folder, prefix string)
	walk = func(fs []*Folder, prefix string) {
		for _, f := range fs {
			p := f.Name
			if prefix != "" {
				p = prefix + "/" + f.Name
			}
			out = append(out, folderTarget{path: p, folder: f})
			walk(f.Folders, p)
		}
	}
	walk(p.Folders, "")
	return out
}

// targetByPath 根据路径找到对应的 folderTarget（找不到返回根目录）。
func (p *Project) targetByPath(path string) folderTarget {
	for _, t := range p.folderTargets() {
		if t.path == path {
			return t
		}
	}
	return folderTarget{path: "项目根目录"}
}

// ---------------------------------------------------------------------------
// 环境工具
// ---------------------------------------------------------------------------

// currentEnv 返回当前选中的环境（越界时回退到第一个）。
func (s *projectSession) currentEnv() *Environment {
	envs := s.project.Environments
	if len(envs) == 0 {
		return nil
	}
	if s.envIndex < 0 || s.envIndex >= len(envs) {
		s.envIndex = 0
	}
	return envs[s.envIndex]
}

// envNameAt 返回第 i 个环境的名称（供环境下拉框标签使用）。
func (s *projectSession) envNameAt(i int) string {
	if i < 0 || i >= len(s.project.Environments) {
		return ""
	}
	return s.project.Environments[i].Name
}

// envIndices 返回环境下拉框的下标切片（环境数量变化时才重新分配）。
func (s *projectSession) envIndices() []int {
	n := len(s.project.Environments)
	if len(s.envIdxCache) != n {
		s.envIdxCache = make([]int, n)
		for i := range s.envIdxCache {
			s.envIdxCache[i] = i
		}
	}
	return s.envIdxCache
}

// requestVars 组装变量表：全局变量 + 环境变量（环境变量优先级更高）。
func (s *projectSession) requestVars() map[string]string {
	vars := map[string]string{}
	for k, v := range s.project.GlobalVars {
		vars[k] = v
	}
	if env := s.currentEnv(); env != nil {
		for k, v := range env.Vars {
			vars[k] = v
		}
	}
	return vars
}

// buildOptions 根据当前编辑的接口/用例 + 环境，构造发送请求所需的选项。
func (s *projectSession) buildOptions() RequestOptions {
	opts := RequestOptions{Vars: s.requestVars()}

	// 环境 base_url。
	if env := s.currentEnv(); env != nil {
		opts.BaseURL = env.BaseURL
	}

	// 全局 + 环境 Header 作为基础 Header。
	merged := map[string]string{}
	for k, v := range s.project.GlobalHeaders {
		merged[k] = v
	}
	if env := s.currentEnv(); env != nil {
		for k, v := range env.Headers {
			merged[k] = v
		}
	}

	// 当前标签页对应的请求定义。
	if tab := s.currentTab(); tab != nil {
		switch tab.kind {
		case tabInterface:
			if tab.iface != nil {
				fillOptsFromInterface(&opts, tab.iface)
			}
		case tabCase:
			if tab.case_ != nil {
				opts.Method = tab.case_.Method
				opts.Path = tab.case_.Path
				opts.Params = cloneKVs(tab.case_.Params)
				opts.Headers = cloneKVs(tab.case_.Headers)
				opts.Cookies = cloneKVs(tab.case_.Cookies)
				opts.Body = cloneBody(tab.case_.Body)
				opts.Auth = tab.case_.Auth
			}
		}
	}

	// 全局 / 环境 Header 放在接口自己的 Header 之后，接口自己的优先。
	// 用新切片合并，避免 append 破坏接口原来的 Header 底层数组。
	mergedHeaders := append([]KeyValue(nil), opts.Headers...)
	for k, v := range merged {
		exists := false
		for _, h := range mergedHeaders {
			if strings.EqualFold(h.Key, k) {
				exists = true
				break
			}
		}
		if !exists {
			mergedHeaders = append(mergedHeaders, KeyValue{Key: k, Value: v, Enabled: true})
		}
	}
	opts.Headers = mergedHeaders
	return opts
}

// fillOptsFromInterface 把接口的定义（深拷贝）填入选项，避免后台请求与编辑共享数据。
func fillOptsFromInterface(opts *RequestOptions, iface *Interface) {
	opts.Method = iface.Method
	opts.Path = iface.Path
	opts.Params = cloneKVs(iface.Params)
	opts.Headers = cloneKVs(iface.Headers)
	opts.Cookies = cloneKVs(iface.Cookies)
	opts.Body = cloneBody(iface.Body)
	opts.Auth = iface.Auth
}

// cloneBody 深拷贝请求体（Raw 字符串不可变，只拷贝两个键值对切片）。
func cloneBody(b Body) Body {
	b.FormData = cloneKVs(b.FormData)
	b.URLEncoded = cloneKVs(b.URLEncoded)
	return b
}

// currentTab 返回当前工作区标签页（越界返回 nil）。
func (s *projectSession) currentTab() *workTab {
	if s.activeWork < 0 || s.activeWork >= len(s.workTabs) {
		return nil
	}
	return s.workTabs[s.activeWork]
}

// ---------------------------------------------------------------------------
// 方法颜色
// ---------------------------------------------------------------------------

// methodColor 根据 HTTP 方法返回一个主题颜色（用于方法徽标与下拉框）。
// 采用业界通用配色：GET 绿、POST 橙、PUT 蓝、DELETE 红、PATCH 紫。
func methodColor(method string) gfx.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return gfx.RGB(34, 197, 94) // 绿色
	case "POST":
		return gfx.RGB(249, 115, 22) // 橙色
	case "PUT":
		return gfx.RGB(59, 130, 246) // 蓝色
	case "DELETE":
		return gfx.RGB(239, 68, 68) // 红色
	case "PATCH":
		return gfx.RGB(168, 85, 247) // 紫色
	default:
		return gfx.RGB(107, 114, 128) // 灰色
	}
}

// methodLetter 返回方法的单个字母（用于树里的圆形徽标）。
func methodLetter(method string) string {
	if method == "" {
		return "?"
	}
	return strings.ToUpper(method[:1])
}

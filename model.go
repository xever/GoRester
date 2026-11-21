package main

// 本文件定义 GoRester 的核心数据模型。
// 所有数据最终都会以 JSON 格式保存到用户目录 ~/gorester/ 下。
// 结构体字段后面的 `json:"..."` 标签，决定了保存成 JSON 文件时的字段名。

import "time"

// ---------------------------------------------------------------------------
// HTTP 方法
// ---------------------------------------------------------------------------

// HTTPMethod 表示一个 HTTP 请求方法，例如 GET、POST。
// 用字符串类型而不是普通 string，是为了让代码更安全、更易读：
// 只能赋值为下面定义的常量之一（虽然 Go 不会强制，但语义更清晰）。
type HTTPMethod string

// 所有支持的 HTTP 方法。
const (
	MethodGet     HTTPMethod = "GET"
	MethodPost    HTTPMethod = "POST"
	MethodPut     HTTPMethod = "PUT"
	MethodDelete  HTTPMethod = "DELETE"
	MethodHead    HTTPMethod = "HEAD"
	MethodOptions HTTPMethod = "OPTIONS"
	MethodPatch   HTTPMethod = "PATCH"
)

// AllMethods 是所有可选方法的切片，供 UI 下拉框使用。
var AllMethods = []HTTPMethod{
	MethodGet, MethodPut, MethodPost, MethodDelete, MethodHead, MethodOptions, MethodPatch,
}

// ---------------------------------------------------------------------------
// 键值对（参数 / Header / Cookie 等通用结构）
// ---------------------------------------------------------------------------

// KeyValue 是一个通用的“键值对”结构。
// 参数(Params)、请求头(Header)、Cookie 以及 Body 表单，都用它来表示一行数据。
type KeyValue struct {
	Key      string `json:"key"`                // 参数名 / 键
	Value    string `json:"value"`              // 参数值 / 示例值
	Type     string `json:"type,omitempty"`     // 类型（例如 form-data 里的 text / file）
	Desc     string `json:"desc,omitempty"`     // 说明
	Required bool   `json:"required,omitempty"` // 是否必填（界面上显示 * 号）
	Enabled  bool   `json:"enabled"`            // 是否启用（勾选后才参与请求）
}

// ---------------------------------------------------------------------------
// 请求体（Body）
// ---------------------------------------------------------------------------

// Body 描述一个接口的请求体。
// Mode 决定用哪种方式发送：none / form-data / x-www-form-urlencoded / json / xml / raw。
type Body struct {
	Mode string `json:"mode"` // 请求体格式

	Raw        string     `json:"raw,omitempty"`         // json / xml / raw 的原始文本内容
	FormData   []KeyValue `json:"form_data,omitempty"`   // form-data 键值对（支持文件）
	URLEncoded []KeyValue `json:"url_encoded,omitempty"` // x-www-form-urlencoded 键值对
}

// ---------------------------------------------------------------------------
// 认证（Auth）
// ---------------------------------------------------------------------------

// Auth 描述接口的认证方式。
// Type 可选：none / basic / bearer / api-key。
type Auth struct {
	Type     string `json:"type"`               // 认证类型
	Username string `json:"username,omitempty"` // Basic 认证的用户名
	Password string `json:"password,omitempty"` // Basic 认证的密码
	Token    string `json:"token,omitempty"`    // Bearer 认证的 token
	Key      string `json:"key,omitempty"`      // api-key 的键名
	Value    string `json:"value,omitempty"`    // api-key 的值
	In       string `json:"in,omitempty"`       // api-key 放在 header 还是 query
}

// ---------------------------------------------------------------------------
// 用例（Case）
// ---------------------------------------------------------------------------

// Case 表示某个接口的一个“用例”：一份已保存的、可复用的请求快照。
// 用例挂载在接口之下，点击用例即可打开并直接发送。
type Case struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Method  string     `json:"method"`
	Path    string     `json:"path"`
	Params  []KeyValue `json:"params,omitempty"`
	Headers []KeyValue `json:"headers,omitempty"`
	Cookies []KeyValue `json:"cookies,omitempty"`
	Body    Body       `json:"body"`
	Auth    Auth       `json:"auth"`
}

// ---------------------------------------------------------------------------
// 接口（Interface）
// ---------------------------------------------------------------------------

// Interface 表示一个 HTTP 接口的完整定义。
type Interface struct {
	ID      string     `json:"id"`
	Name    string     `json:"name"`
	Method  string     `json:"method"` // GET / POST / ...
	Path    string     `json:"path"`   // 接口路径，例如 /api/users
	Params  []KeyValue `json:"params,omitempty"`
	Headers []KeyValue `json:"headers,omitempty"`
	Cookies []KeyValue `json:"cookies,omitempty"`
	Body    Body       `json:"body"`
	Auth    Auth       `json:"auth"`
	Cases   []*Case    `json:"cases,omitempty"`
}

// ---------------------------------------------------------------------------
// 文件夹（Folder）
// ---------------------------------------------------------------------------

// Folder 是接口树上的一个文件夹节点，可以继续嵌套文件夹和接口。
type Folder struct {
	ID         string       `json:"id"`
	Name       string       `json:"name"`
	Folders    []*Folder    `json:"folders,omitempty"`    // 子文件夹
	Interfaces []*Interface `json:"interfaces,omitempty"` // 文件夹下的接口
}

// ---------------------------------------------------------------------------
// 环境（Environment）
// ---------------------------------------------------------------------------

// Environment 表示一套运行环境（开发 / 测试 / 生产等）。
// 每个环境有自己的 base_url、变量、全局 Header 和默认超时时间。
type Environment struct {
	Name      string            `json:"name"`
	BaseURL   string            `json:"base_url"`          // 基础地址，如 https://api.example.com
	Vars      map[string]string `json:"vars,omitempty"`    // 环境变量（可在 URL / 参数中引用）
	Headers   map[string]string `json:"headers,omitempty"` // 环境级全局 Header
	TimeoutMs int               `json:"timeout_ms"`        // 默认超时（毫秒），0 表示用全局默认 30 秒
}

// DefaultTimeoutMs 是接口请求的全局默认超时时间：30 秒。
const DefaultTimeoutMs = 30 * 1000

// ---------------------------------------------------------------------------
// 项目（Project）
// ---------------------------------------------------------------------------

// Project 是一个项目，对应磁盘上的一个文件夹（文件夹名 = 项目名）。
type Project struct {
	Name          string            `json:"name"`
	Description   string            `json:"description"`
	Interfaces    []*Interface      `json:"interfaces,omitempty"`     // 项目根目录下的接口（不属于任何文件夹）
	Folders       []*Folder         `json:"folders,omitempty"`        // 项目下的文件夹树
	Environments  []*Environment    `json:"environments"`             // 环境列表
	GlobalVars    map[string]string `json:"global_vars,omitempty"`    // 全局变量
	GlobalHeaders map[string]string `json:"global_headers,omitempty"` // 全局 Header
	CreatedAt     time.Time         `json:"created_at"`
	UpdatedAt     time.Time         `json:"updated_at"`
}

// NewProject 创建一个带默认环境的新项目。
func NewProject(name, description string) *Project {
	now := time.Now()
	return &Project{
		Name:        name,
		Description: description,
		Environments: []*Environment{
			{
				Name:      "开发环境",
				BaseURL:   "",
				Vars:      map[string]string{},
				Headers:   map[string]string{},
				TimeoutMs: DefaultTimeoutMs,
			},
		},
		GlobalVars:    map[string]string{},
		GlobalHeaders: map[string]string{},
		CreatedAt:     now,
		UpdatedAt:     now,
	}
}

// Package openapi 实现 OpenAPI 3.0 格式的导入 / 导出。
//
// 需求：接口支持按项目导入导出 OpenAPI 3.0 格式。
// 这里定义了一套最小但完整的 OpenAPI 3.0 数据结构，
// 并提供了「项目 -> OpenAPI 文档」和「OpenAPI 文档 -> 项目」两个方向的转换。
package openapi

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"

	"gorester/internal/model"
)

// ---------------------------------------------------------------------------
// OpenAPI 3.0 数据结构
// ---------------------------------------------------------------------------

// OpenAPI 是一份 OpenAPI 3.0 文档的根对象。
type OpenAPI struct {
	OpenAPI string              `json:"openapi"` // 固定 "3.0.x"
	Info    OpenAPIInfo         `json:"info"`    // 文档基本信息
	Paths   map[string]PathItem `json:"paths"`   // 路径 -> 该路径下的方法集合
}

// OpenAPIInfo 是文档标题、版本、说明等元信息。
type OpenAPIInfo struct {
	Title       string `json:"title"`
	Version     string `json:"version"`
	Description string `json:"description,omitempty"`
}

// PathItem 是一个路径下所有 HTTP 方法对应的操作。
// 每个方法用指针表示：nil 表示该路径不支持这个方法。
type PathItem struct {
	Get     *Operation `json:"get,omitempty"`
	Put     *Operation `json:"put,omitempty"`
	Post    *Operation `json:"post,omitempty"`
	Delete  *Operation `json:"delete,omitempty"`
	Options *Operation `json:"options,omitempty"`
	Head    *Operation `json:"head,omitempty"`
	Patch   *Operation `json:"patch,omitempty"`
}

// Operation 是一个 HTTP 方法的定义。
type Operation struct {
	Summary     string       `json:"summary,omitempty"`
	Description string       `json:"description,omitempty"`
	Parameters  []Parameter  `json:"parameters,omitempty"`
	RequestBody *RequestBody `json:"requestBody,omitempty"`
}

// Parameter 是一个参数（查询参数、路径参数、Header、Cookie 等）。
type Parameter struct {
	Name        string  `json:"name"`
	In          string  `json:"in"` // query / header / path / cookie
	Required    bool    `json:"required,omitempty"`
	Description string  `json:"description,omitempty"`
	Schema      *Schema `json:"schema,omitempty"`
}

// RequestBody 是请求体定义，Content 的键是 MIME 类型。
type RequestBody struct {
	Required bool                 `json:"required,omitempty"`
	Content  map[string]MediaType `json:"content"`
}

// MediaType 描述某一种 MIME 类型的内容结构和示例。
type MediaType struct {
	Schema  *Schema     `json:"schema,omitempty"`
	Example interface{} `json:"example,omitempty"`
}

// Schema 是对 JSON Schema 的极简子集，足够表达常见参数/请求体结构。
type Schema struct {
	Type        string             `json:"type,omitempty"`   // string / integer / number / boolean / array / object
	Format      string             `json:"format,omitempty"` // 例如 binary、date-time
	Example     interface{}        `json:"example,omitempty"`
	Description string             `json:"description,omitempty"`
	Properties  map[string]*Schema `json:"properties,omitempty"` // object 的字段
	Items       *Schema            `json:"items,omitempty"`      // array 的元素类型
	Required    []string           `json:"required,omitempty"`   // object 的必填字段
}

// ---------------------------------------------------------------------------
// 解析与序列化
// ---------------------------------------------------------------------------

// Parse 把 JSON 字节解析为 OpenAPI 文档。
func Parse(data []byte) (*OpenAPI, error) {
	var doc OpenAPI
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil, fmt.Errorf("解析 OpenAPI 文档失败: %w", err)
	}
	if doc.Paths == nil {
		return nil, fmt.Errorf("OpenAPI 文档缺少 paths 字段")
	}
	return &doc, nil
}

// Marshal 把 OpenAPI 文档序列化为带缩进的 JSON 字节。
func (d *OpenAPI) Marshal() ([]byte, error) {
	return json.MarshalIndent(d, "", "  ")
}

// ---------------------------------------------------------------------------
// 导出：项目 -> OpenAPI 文档
// ---------------------------------------------------------------------------

// Export 把项目里的所有接口导出为 OpenAPI 3.0 文档。
func Export(p *model.Project) *OpenAPI {
	doc := &OpenAPI{
		OpenAPI: "3.0.0",
		Info: OpenAPIInfo{
			Title:       p.Name,
			Version:     "1.0.0",
			Description: p.Description,
		},
		Paths: map[string]PathItem{},
	}

	// 递归遍历所有文件夹，把接口收集起来。
	var collect func(folders []*model.Folder)
	collect = func(folders []*model.Folder) {
		for _, f := range folders {
			for _, iface := range f.Interfaces {
				doc.addInterface(iface)
			}
			collect(f.Folders)
		}
	}
	collect(p.Folders)

	return doc
}

// addInterface 把单个接口写入文档的 paths。
func (d *OpenAPI) addInterface(iface *model.Interface) {
	path := iface.Path
	if path == "" {
		path = "/"
	}
	if !strings.HasPrefix(path, "/") {
		path = "/" + path
	}

	item := d.Paths[path]
	op := interfaceToOperation(iface)
	setOperation(&item, iface.Method, op)
	d.Paths[path] = item
}

// interfaceToOperation 把接口定义转成 OpenAPI Operation。
func interfaceToOperation(iface *model.Interface) *Operation {
	op := &Operation{
		Summary: iface.Name,
	}

	// 查询参数。
	for _, p := range iface.Params {
		if p.Key == "" {
			continue
		}
		op.Parameters = append(op.Parameters, Parameter{
			Name:        p.Key,
			In:          "query",
			Required:    p.Required,
			Description: p.Desc,
			Schema:      &Schema{Type: p.TypeOrDefault(), Example: p.Value},
		})
	}

	// 请求体。
	if body, mime := bodyToContent(iface.Body); mime != "" {
		op.RequestBody = &RequestBody{Content: map[string]MediaType{mime: body}}
	}
	return op
}

// bodyToContent 把 Body 转成 OpenAPI 的 content 定义。
// 返回 (MediaType, mime)；mime 为空表示没有请求体。
func bodyToContent(body model.Body) (MediaType, string) {
	switch body.Mode {
	case "json":
		ex := parseExampleJSON(body.Raw)
		return MediaType{Schema: &Schema{Type: "object"}, Example: ex}, "application/json"
	case "xml":
		return MediaType{Schema: &Schema{Type: "object"}, Example: body.Raw}, "application/xml"
	case "raw":
		return MediaType{Schema: &Schema{Type: "string"}, Example: body.Raw}, "text/plain"
	case "x-www-form-urlencoded":
		ex := map[string]interface{}{}
		for _, kv := range body.URLEncoded {
			if kv.Key != "" {
				ex[kv.Key] = kv.Value
			}
		}
		return MediaType{Schema: &Schema{Type: "object"}, Example: ex}, "application/x-www-form-urlencoded"
	case "form-data":
		return MediaType{Schema: &Schema{Type: "object"}}, "multipart/form-data"
	default:
		return MediaType{}, ""
	}
}

// setOperation 根据方法名把操作写入 PathItem 的对应字段。
func setOperation(item *PathItem, method string, op *Operation) {
	switch strings.ToUpper(method) {
	case "GET":
		item.Get = op
	case "POST":
		item.Post = op
	case "PUT":
		item.Put = op
	case "DELETE":
		item.Delete = op
	case "HEAD":
		item.Head = op
	case "OPTIONS":
		item.Options = op
	case "PATCH":
		item.Patch = op
	}
}

// ---------------------------------------------------------------------------
// 导入：OpenAPI 文档 -> 项目
// ---------------------------------------------------------------------------

// Import 把 OpenAPI 文档里的所有接口导入项目，
// 统一放进一个名为 folderName 的文件夹中。
func Import(p *model.Project, doc *OpenAPI, folderName string) {
	if folderName == "" {
		folderName = doc.Info.Title
		if folderName == "" {
			folderName = "导入的接口"
		}
	}
	folder := &model.Folder{ID: model.NewID(), Name: folderName}

	// paths 是 map，遍历顺序不稳定，先排序保证导入结果可复现。
	paths := make([]string, 0, len(doc.Paths))
	for path := range doc.Paths {
		paths = append(paths, path)
	}
	sort.Strings(paths)

	for _, path := range paths {
		item := doc.Paths[path]
		for _, entry := range operationsOf(item) {
			iface := operationToInterface(entry.method, path, entry.op)
			folder.Interfaces = append(folder.Interfaces, iface)
		}
	}

	p.Folders = append(p.Folders, folder)
}

// opEntry 是一个「方法名 + 操作」的组合。
type opEntry struct {
	method string
	op     *Operation
}

// operationsOf 把 PathItem 里非空的方法按固定顺序收集起来。
func operationsOf(item PathItem) []opEntry {
	var out []opEntry
	add := func(m string, op *Operation) {
		if op != nil {
			out = append(out, opEntry{m, op})
		}
	}
	add("GET", item.Get)
	add("POST", item.Post)
	add("PUT", item.Put)
	add("DELETE", item.Delete)
	add("PATCH", item.Patch)
	add("HEAD", item.Head)
	add("OPTIONS", item.Options)
	return out
}

// operationToInterface 把 OpenAPI Operation 转成接口定义。
func operationToInterface(method, path string, op *Operation) *model.Interface {
	iface := &model.Interface{
		ID:     model.NewID(),
		Name:   op.Summary,
		Method: method,
		Path:   path,
	}
	if iface.Name == "" {
		iface.Name = method + " " + path
	}

	// 参数。
	for _, param := range op.Parameters {
		switch param.In {
		case "query":
			iface.Params = append(iface.Params, model.KeyValue{
				Key: param.Name, Value: exampleString(param.Schema), Desc: param.Description, Required: param.Required, Enabled: true,
			})
		case "header":
			iface.Headers = append(iface.Headers, model.KeyValue{
				Key: param.Name, Value: exampleString(param.Schema), Desc: param.Description, Enabled: true,
			})
		case "cookie":
			iface.Cookies = append(iface.Cookies, model.KeyValue{
				Key: param.Name, Value: exampleString(param.Schema), Desc: param.Description, Enabled: true,
			})
		}
	}

	// 请求体。
	if op.RequestBody != nil && len(op.RequestBody.Content) > 0 {
		applyContent(iface, op.RequestBody.Content)
	}

	return iface
}

// applyContent 根据请求体的 content 设置接口的 Body。
func applyContent(iface *model.Interface, content map[string]MediaType) {
	// 优先处理 json，其次 xml / form。
	if mt, ok := content["application/json"]; ok {
		iface.Body.Mode = "json"
		iface.Body.Raw = exampleJSON(mt)
		return
	}
	if mt, ok := content["application/xml"]; ok {
		iface.Body.Mode = "xml"
		iface.Body.Raw = exampleString(mt.Schema)
		return
	}
	if mt, ok := content["application/x-www-form-urlencoded"]; ok {
		iface.Body.Mode = "x-www-form-urlencoded"
		iface.Body.URLEncoded = schemaToKeyValues(mt.Schema)
		return
	}
	if mt, ok := content["multipart/form-data"]; ok {
		iface.Body.Mode = "form-data"
		iface.Body.FormData = schemaToKeyValues(mt.Schema)
		return
	}
	// 其它类型当作 raw 处理。
	for _, mt := range content {
		iface.Body.Mode = "raw"
		iface.Body.Raw = exampleString(mt.Schema)
		return
	}
}

// schemaToKeyValues 把 object 类型的 schema 转成键值对列表。
func schemaToKeyValues(s *Schema) []model.KeyValue {
	if s == nil || s.Type != "object" || len(s.Properties) == 0 {
		return nil
	}
	var out []model.KeyValue
	required := map[string]bool{}
	for _, r := range s.Required {
		required[r] = true
	}
	// 排序保证顺序稳定。
	keys := make([]string, 0, len(s.Properties))
	for k := range s.Properties {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		ps := s.Properties[k]
		out = append(out, model.KeyValue{
			Key: k, Value: exampleString(ps), Type: ps.Type, Desc: ps.Description, Required: required[k], Enabled: true,
		})
	}
	return out
}

// ---------------------------------------------------------------------------
// 示例提取 / 生成工具
// ---------------------------------------------------------------------------

// parseExampleJSON 尝试把 JSON 字符串解析成对象作为示例；失败则原样返回字符串。
func parseExampleJSON(raw string) interface{} {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var v interface{}
	if err := json.Unmarshal([]byte(raw), &v); err != nil {
		return raw
	}
	return v
}

// exampleJSON 返回 media type 的示例 JSON 字符串。
func exampleJSON(mt MediaType) string {
	if mt.Example != nil {
		b, err := json.MarshalIndent(mt.Example, "", "  ")
		if err == nil {
			return string(b)
		}
	}
	if mt.Schema != nil {
		return generateSampleJSON(mt.Schema)
	}
	return ""
}

// exampleString 返回 schema 的示例值字符串。
func exampleString(s *Schema) string {
	if s == nil {
		return ""
	}
	if s.Example != nil {
		switch v := s.Example.(type) {
		case string:
			return v
		default:
			b, _ := json.Marshal(v)
			return string(b)
		}
	}
	// 根据类型给一个友好默认值。
	switch s.Type {
	case "integer":
		return "0"
	case "number":
		return "0.0"
	case "boolean":
		return "false"
	default:
		return ""
	}
}

// generateSampleJSON 根据 schema 生成一份示例 JSON 文本，方便导入后直接可发请求。
func generateSampleJSON(s *Schema) string {
	if s == nil {
		return "{}"
	}
	v := generateSample(s)
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return "{}"
	}
	return string(b)
}

// generateSample 递归生成 schema 对应的 Go 值。
func generateSample(s *Schema) interface{} {
	switch s.Type {
	case "object":
		m := map[string]interface{}{}
		if s.Properties != nil {
			for k, ps := range s.Properties {
				m[k] = generateSample(ps)
			}
		}
		return m
	case "array":
		if s.Items != nil {
			return []interface{}{generateSample(s.Items)}
		}
		return []interface{}{}
	case "integer":
		return 0
	case "number":
		return 0.0
	case "boolean":
		return false
	default:
		if s.Example != nil {
			return s.Example
		}
		return ""
	}
}

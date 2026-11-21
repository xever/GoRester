package main

// 本文件负责执行 HTTP 请求。
//
// 性能要求（见需求）：
//   - 每个项目复用同一个 http.Client（复用底层连接池，避免重复握手）
//   - 默认超时 30 秒
//   - 及时释放返回值内容（读取完响应体后立即关闭）

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// 请求输入 / 响应结果
// ---------------------------------------------------------------------------

// RequestOptions 描述一次请求所需的全部输入。
// 它由当前编辑的接口 / 用例 + 选中的环境拼接而成。
type RequestOptions struct {
	Method  string            // HTTP 方法
	BaseURL string            // 环境 base_url（与 Path 拼接成完整 URL）
	Path    string            // 接口路径
	Params  []KeyValue        // 查询参数（Query）
	Headers []KeyValue        // 请求头
	Cookies []KeyValue        // Cookie
	Body    Body              // 请求体
	Auth    Auth              // 认证信息
	Vars    map[string]string // 变量表：环境变量 + 全局变量，用于替换 {{name}}
}

// Result 是一次请求的完整结果。
type Result struct {
	StatusCode int           // 状态码，如 200
	Status     string        // 状态文本，如 "200 OK"
	Proto      string        // 协议版本，如 "HTTP/1.1"
	Headers    []KeyValue    // 响应头
	Body       []byte        // 响应体原始字节
	Elapsed    time.Duration // 从发出请求到读完响应的耗时
	Err        error         // 请求过程中发生的错误（nil 表示成功）
}

// BodyString 把响应体字节转成字符串，供界面展示。
func (r *Result) BodyString() string {
	if r == nil {
		return ""
	}
	return string(r.Body)
}

// ---------------------------------------------------------------------------
// 客户端
// ---------------------------------------------------------------------------

// Client 封装了一个可复用的 http.Client。
// 一个项目持有一个 Client，多次请求共享同一个连接池。
type Client struct {
	hc      *http.Client
	timeout time.Duration
}

// NewClient 创建一个 HTTP 客户端。timeout<=0 时使用默认 30 秒。
func NewClient(timeout time.Duration) *Client {
	if timeout <= 0 {
		timeout = DefaultTimeoutMs * time.Millisecond
	}
	// Transport 里的连接池参数让同一个项目的多次请求复用 TCP 连接。
	tr := &http.Transport{
		MaxIdleConns:        20,                                    // 最多保留 20 个空闲连接
		MaxIdleConnsPerHost: 10,                                    // 每个主机最多保留 10 个空闲连接
		IdleConnTimeout:     90 * time.Second,                      // 空闲连接超过 90 秒才关闭
		TLSClientConfig:     &tls.Config{InsecureSkipVerify: true}, // 允许自签名证书，方便调试
	}
	return &Client{
		hc: &http.Client{
			Transport: tr,
			Timeout:   timeout,
		},
		timeout: timeout,
	}
}

// SetTimeout 更新客户端超时时间（用户在环境里修改默认耗时时调用）。
func (c *Client) SetTimeout(timeout time.Duration) {
	if timeout <= 0 {
		timeout = DefaultTimeoutMs * time.Millisecond
	}
	c.timeout = timeout
	c.hc.Timeout = timeout
}

// Timeout 返回当前超时时间。
func (c *Client) Timeout() time.Duration {
	return c.timeout
}

// Do 根据 opts 构建并执行一个 HTTP 请求，返回结果。
// 无论成功失败都会返回一个 *Result，错误记录在 Result.Err 中。
func (c *Client) Do(opts RequestOptions) *Result {
	res := &Result{}

	// 1. 组装完整 URL（base_url + path + 查询参数）。
	req, err := c.buildRequest(opts)
	if err != nil {
		res.Err = err
		return res
	}

	// 2. 记录起始时间，执行请求。
	start := time.Now()
	resp, err := c.hc.Do(req)
	res.Elapsed = time.Since(start)
	if err != nil {
		res.Err = err
		return res
	}
	// 3. 无论后面怎么处理，最后一定要关闭响应体，释放连接和内存。
	defer resp.Body.Close()

	// 4. 读取响应体。
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		res.Err = err
		return res
	}

	// 5. 填充结果。
	res.StatusCode = resp.StatusCode
	res.Status = resp.Status
	res.Proto = resp.Proto
	res.Body = body
	res.Headers = headersToKeyValues(resp.Header)
	return res
}

// buildRequest 根据 opts 构造 *http.Request，并应用参数、Header、Cookie、Body 和 Auth。
func (c *Client) buildRequest(opts RequestOptions) (*http.Request, error) {
	// 变量替换：把 path、参数、header 里的 {{name}} 替换成变量值。
	vars := opts.Vars
	if vars == nil {
		vars = map[string]string{}
	}
	subst := func(s string) string { return substituteVars(s, vars) }

	fullPath := subst(opts.Path)

	// 拼接 base_url 和 path。
	base := strings.TrimRight(subst(opts.BaseURL), "/")
	fullURL := base + "/" + strings.TrimLeft(fullPath, "/")

	// 组装查询参数。
	u, err := url.Parse(fullURL)
	if err != nil {
		return nil, fmt.Errorf("URL 解析失败: %w", err)
	}
	q := u.Query()
	for _, p := range opts.Params {
		if !p.Enabled && p.Key == "" {
			continue
		}
		if p.Key == "" {
			continue
		}
		q.Add(subst(p.Key), subst(p.Value))
	}
	u.RawQuery = q.Encode()

	// 组装请求体（会顺带返回需要设置的内容类型）。
	bodyReader, contentType, err := buildBody(opts.Body, subst)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest(string(opts.Method), u.String(), bodyReader)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 设置请求头。
	for _, h := range opts.Headers {
		if h.Key == "" {
			continue
		}
		req.Header.Set(subst(h.Key), subst(h.Value))
	}
	// 如果 body 产生了内容类型且用户没有手动设置，则应用它。
	if contentType != "" && req.Header.Get("Content-Type") == "" {
		req.Header.Set("Content-Type", contentType)
	}

	// 设置 Cookie。
	if len(opts.Cookies) > 0 {
		cookies := make([]string, 0, len(opts.Cookies))
		for _, ck := range opts.Cookies {
			if ck.Key == "" {
				continue
			}
			cookies = append(cookies, subst(ck.Key)+"="+subst(ck.Value))
		}
		if len(cookies) > 0 {
			req.Header.Set("Cookie", strings.Join(cookies, "; "))
		}
	}

	// 应用认证信息。
	if err := applyAuth(req, opts.Auth, subst); err != nil {
		return nil, err
	}

	return req, nil
}

// ---------------------------------------------------------------------------
// 变量替换
// ---------------------------------------------------------------------------

// substituteVars 把字符串里的 {{name}} 占位符替换成 vars 中对应的值。
// 如果变量不存在，则保留原样（方便用户看到哪里没替换成功）。
func substituteVars(s string, vars map[string]string) string {
	if !strings.Contains(s, "{{") {
		return s
	}
	for k, v := range vars {
		s = strings.ReplaceAll(s, "{{"+k+"}}", v)
	}
	return s
}

// ---------------------------------------------------------------------------
// 请求体构建
// ---------------------------------------------------------------------------

// buildBody 根据 Body 的模式构造请求体读取器和对应的 Content-Type。
func buildBody(body Body, subst func(string) string) (io.Reader, string, error) {
	switch body.Mode {
	case "json":
		return strings.NewReader(subst(body.Raw)), "application/json", nil
	case "xml":
		return strings.NewReader(subst(body.Raw)), "application/xml", nil
	case "raw":
		return strings.NewReader(subst(body.Raw)), "text/plain", nil
	case "x-www-form-urlencoded":
		vals := url.Values{}
		for _, kv := range body.URLEncoded {
			if kv.Key == "" {
				continue
			}
			vals.Set(subst(kv.Key), subst(kv.Value))
		}
		return strings.NewReader(vals.Encode()), "application/x-www-form-urlencoded", nil
	case "form-data":
		return buildFormData(body.FormData, subst)
	default: // none 或未知模式：不发送请求体
		return nil, "", nil
	}
}

// buildFormData 构造 multipart/form-data 请求体。
// Type 为 "file" 的项，其 Value 会被当成文件路径读取并作为文件上传。
func buildFormData(items []KeyValue, subst func(string) string) (io.Reader, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, kv := range items {
		if kv.Key == "" {
			continue
		}
		key := subst(kv.Key)
		if kv.Type == "file" {
			// 文件类型：Value 是本地文件路径。
			path := subst(kv.Value)
			f, err := os.Open(path)
			if err != nil {
				return nil, "", fmt.Errorf("打开上传文件失败 %s: %w", path, err)
			}
			part, err := w.CreateFormFile(key, filepath.Base(path))
			if err != nil {
				f.Close()
				return nil, "", err
			}
			if _, err := io.Copy(part, f); err != nil {
				f.Close()
				return nil, "", err
			}
			f.Close()
		} else {
			// 普通文本字段。
			if err := w.WriteField(key, subst(kv.Value)); err != nil {
				return nil, "", err
			}
		}
	}

	// 必须调用 Close 写入 multipart 的结束边界。
	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return &buf, w.FormDataContentType(), nil
}

// ---------------------------------------------------------------------------
// 认证
// ---------------------------------------------------------------------------

// applyAuth 把认证信息写入请求。
func applyAuth(req *http.Request, auth Auth, subst func(string) string) error {
	switch auth.Type {
	case "basic":
		req.SetBasicAuth(subst(auth.Username), subst(auth.Password))
	case "bearer":
		req.Header.Set("Authorization", "Bearer "+subst(auth.Token))
	case "api-key":
		key := subst(auth.Key)
		value := subst(auth.Value)
		if auth.In == "query" {
			// 加到查询参数里。
			q := req.URL.Query()
			q.Set(key, value)
			req.URL.RawQuery = q.Encode()
		} else {
			// 默认加到 header 里。
			req.Header.Set(key, value)
		}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 响应头转换
// ---------------------------------------------------------------------------

// headersToKeyValues 把 http.Header 转成统一的 KeyValue 列表，方便表格展示。
func headersToKeyValues(h http.Header) []KeyValue {
	out := make([]KeyValue, 0, len(h))
	for k, vs := range h {
		for _, v := range vs {
			out = append(out, KeyValue{Key: k, Value: v, Enabled: true})
		}
	}
	return out
}

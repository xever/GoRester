package httpclient

import (
	"strings"
	"testing"

	"gorester/internal/model"
)

// TestDescribeResolvesRequest 验证 Describe 能把请求解析成可读的「实际请求」描述。
func TestDescribeResolvesRequest(t *testing.T) {
	client := NewClient(0)
	info := client.Describe(RequestOptions{
		Method:  "POST",
		BaseURL: "https://api.example.com",
		Path:    "/users/{{id}}",
		Params:  []model.KeyValue{{Key: "page", Value: "1", Enabled: true}},
		Headers: []model.KeyValue{{Key: "X-Token", Value: "abc", Enabled: true}},
		Cookies: []model.KeyValue{{Key: "sid", Value: "123", Enabled: true}},
		Auth:    model.Auth{Type: "bearer", Token: "secret"},
		Body:    model.Body{Mode: "json", Raw: `{"name":"张三"}`},
		Vars:    map[string]string{"id": "42"},
	})

	// URL：变量已替换、base_url 与 path 拼接、查询参数已加入。
	if !strings.Contains(info.URL, "https://api.example.com/users/42") {
		t.Fatalf("URL 应包含 base_url+path，实际 %q", info.URL)
	}
	if !strings.Contains(info.URL, "page=1") {
		t.Fatalf("URL 应包含查询参数 page=1，实际 %q", info.URL)
	}

	// 请求头：接口 header、Cookie、认证、Content-Type 都应有。
	got := map[string]string{}
	for _, h := range info.Headers {
		got[h.Key] = h.Value
	}
	if got["X-Token"] != "abc" {
		t.Fatalf("缺少接口 header X-Token: %v", got)
	}
	if got["Cookie"] != "sid=123" {
		t.Fatalf("Cookie 应展开: %v", got)
	}
	if got["Authorization"] != "Bearer secret" {
		t.Fatalf("Authorization 应展开: %v", got)
	}
	if got["Content-Type"] != "application/json" {
		t.Fatalf("Content-Type 应为 application/json: %v", got)
	}

	// 请求体。
	if !info.HasBody || info.Body != `{"name":"张三"}` {
		t.Fatalf("Body 应原样保留，实际 %q", info.Body)
	}
}

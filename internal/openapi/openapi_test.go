package openapi

import (
	"testing"

	"gorester/internal/model"
)

// TestRoundTrip 验证 OpenAPI 导出再导入不会丢失接口。
func TestRoundTrip(t *testing.T) {
	p := model.NewProject("API 项目", "")
	iface := &model.Interface{ID: model.NewID(), Name: "获取用户", Method: "GET", Path: "/users", Params: []model.KeyValue{{Key: "page", Value: "1", Enabled: true}}}
	p.Folders = []*model.Folder{{ID: model.NewID(), Name: "用户", Interfaces: []*model.Interface{iface}}}

	doc := Export(p)
	data, err := doc.Marshal()
	if err != nil {
		t.Fatalf("序列化失败: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	p2 := model.NewProject("导入", "")
	Import(p2, parsed, "导入")
	all := p2.AllInterfaces()
	if len(all) != 1 {
		t.Fatalf("导入后应有 1 个接口，实际 %d", len(all))
	}
	if all[0].Method != "GET" || all[0].Path != "/users" {
		t.Fatalf("导入接口不匹配: %s %s", all[0].Method, all[0].Path)
	}
}

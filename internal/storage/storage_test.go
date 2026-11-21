package storage

import (
	"os"
	"path/filepath"
	"testing"

	"gorester/internal/model"
)

// TestRoundTrip 验证项目的保存 / 列表 / 读取 / 删除。
func TestRoundTrip(t *testing.T) {
	// 使用临时目录作为 HOME，避免污染真实 ~/gorester。
	t.Setenv("HOME", t.TempDir())

	p := model.NewProject("测试项目", "一个测试项目")
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

package main

// 本文件负责把所有数据持久化到磁盘。
//
// 数据存放规则（见需求）：
//   - 所有数据保存到用户主目录下的 ~/gorester/ 目录
//   - 全部使用 JSON 格式
//   - 一个项目一个文件夹，文件夹名就是项目名称
//
// 因此磁盘结构形如：
//
//	~/gorester/
//	  ├── 项目A/
//	  │    └── project.json
//	  └── 项目B/
//	       └── project.json

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// dataFileName 是每个项目文件夹里保存数据的 JSON 文件名。
const dataFileName = "project.json"

// ---------------------------------------------------------------------------
// 目录与路径工具
// ---------------------------------------------------------------------------

// baseDir 返回数据根目录：~/gorester。
// 如果目录不存在会先创建。返回绝对路径。
func baseDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("获取用户主目录失败: %w", err)
	}
	dir := filepath.Join(home, "gorester")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("创建数据目录失败 %s: %w", dir, err)
	}
	return dir, nil
}

// projectDir 返回某个项目的文件夹路径。
func projectDir(name string) (string, error) {
	base, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, safeName(name)), nil
}

// projectFile 返回某个项目的 project.json 文件路径。
func projectFile(name string) (string, error) {
	dir, err := projectDir(name)
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, dataFileName), nil
}

// safeName 把项目名转成安全的目录名：替换路径分隔符和非法字符。
func safeName(name string) string {
	replacer := strings.NewReplacer("/", "_", "\\", "_", ":", "_", "*", "_", "?", "_", "\"", "_", "<", "_", ">", "_", "|", "_")
	return strings.TrimSpace(replacer.Replace(name))
}

// ---------------------------------------------------------------------------
// 项目列表与读写
// ---------------------------------------------------------------------------

// ProjectMeta 是主页格子上展示的项目摘要信息。
type ProjectMeta struct {
	Name        string    // 项目名（= 文件夹名）
	Description string    // 项目说明
	UpdatedAt   time.Time // 最后更新时间
}

// ListProjects 扫描 ~/gorester/ 下的所有项目文件夹，读取每个 project.json
// 的摘要信息，返回按更新时间倒序排列的项目列表。
func ListProjects() ([]ProjectMeta, error) {
	base, err := baseDir()
	if err != nil {
		return nil, err
	}

	entries, err := os.ReadDir(base)
	if err != nil {
		return nil, fmt.Errorf("读取数据目录失败: %w", err)
	}

	var metas []ProjectMeta
	for _, e := range entries {
		if !e.IsDir() {
			continue // 只关心文件夹，忽略其它文件
		}
		pf := filepath.Join(base, e.Name(), dataFileName)
		data, err := os.ReadFile(pf)
		if err != nil {
			continue // 文件夹里没有 project.json 就跳过
		}
		var p Project
		if err := json.Unmarshal(data, &p); err != nil {
			continue // 解析失败的脏数据直接跳过
		}
		metas = append(metas, ProjectMeta{
			Name:        p.Name,
			Description: p.Description,
			UpdatedAt:   p.UpdatedAt,
		})
	}

	// 按更新时间倒序，最近更新的排前面。
	sort.Slice(metas, func(i, j int) bool {
		return metas[i].UpdatedAt.After(metas[j].UpdatedAt)
	})
	return metas, nil
}

// LoadProject 根据项目名加载完整项目数据。
func LoadProject(name string) (*Project, error) {
	pf, err := projectFile(name)
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(pf)
	if err != nil {
		return nil, fmt.Errorf("读取项目失败 %s: %w", name, err)
	}
	var p Project
	if err := json.Unmarshal(data, &p); err != nil {
		return nil, fmt.Errorf("解析项目 JSON 失败 %s: %w", name, err)
	}
	// 兼容旧数据：若没有任何环境，补一个默认环境。
	if len(p.Environments) == 0 {
		p.Environments = NewProject(p.Name, p.Description).Environments
	}
	return &p, nil
}

// SaveProject 把项目数据写回磁盘（含更新时间戳）。
func SaveProject(p *Project) error {
	if p == nil {
		return errors.New("项目为空，无法保存")
	}
	p.UpdatedAt = time.Now()

	dir, err := projectDir(p.Name)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("创建项目目录失败: %w", err)
	}

	// json.MarshalIndent 让 JSON 文件带缩进，便于人工查看和 Git diff。
	data, err := json.MarshalIndent(p, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化项目失败: %w", err)
	}
	pf := filepath.Join(dir, dataFileName)
	if err := os.WriteFile(pf, data, 0o644); err != nil {
		return fmt.Errorf("写入项目失败: %w", err)
	}
	return nil
}

// DeleteProject 删除一个项目（连同其整个文件夹）。
func DeleteProject(name string) error {
	dir, err := projectDir(name)
	if err != nil {
		return err
	}
	if err := os.RemoveAll(dir); err != nil {
		return fmt.Errorf("删除项目失败: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// ID 生成
// ---------------------------------------------------------------------------

// newID 生成一个全局唯一的 ID，用于接口、用例、文件夹等节点。
// 使用加密安全随机数生成 8 字节，转成 16 位十六进制字符串。
func newID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 极端情况下随机源失败，退回时间戳，保证不返回空 ID。
		return fmt.Sprintf("id%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

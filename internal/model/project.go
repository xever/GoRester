package model

// 本文件包含 Project 上的查询方法、文件夹选择器，以及 ID 生成工具。

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"
)

// NewID 生成一个全局唯一的 ID，用于接口、用例、文件夹等节点。
// 使用加密安全随机数生成 8 字节，转成 16 位十六进制字符串。
func NewID() string {
	var b [8]byte
	if _, err := rand.Read(b[:]); err != nil {
		// 极端情况下随机源失败，退回时间戳，保证不返回空 ID。
		return fmt.Sprintf("id%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b[:])
}

// ---------------------------------------------------------------------------
// 项目内的查找
// ---------------------------------------------------------------------------

// AllInterfaces 返回项目里所有接口（根目录 + 所有文件夹，递归）。
func (p *Project) AllInterfaces() []*Interface {
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

// FindInterface 按 ID 查找接口，找不到返回 nil。
func (p *Project) FindInterface(id string) *Interface {
	for _, i := range p.Interfaces {
		if i.ID == id {
			return i
		}
	}
	for _, i := range p.AllInterfaces() {
		if i.ID == id {
			return i
		}
	}
	return nil
}

// FindCase 按 ID 查找用例，找不到返回 nil。
func (p *Project) FindCase(id string) *Case {
	for _, iface := range p.AllInterfaces() {
		for _, c := range iface.Cases {
			if c.ID == id {
				return c
			}
		}
	}
	return nil
}

// FindFolder 按 ID 递归查找文件夹，找不到返回 nil。
func (p *Project) FindFolder(id string) *Folder {
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

// FolderTarget 是「保存到哪个文件夹」下拉框里的一项。
type FolderTarget struct {
	Path   string  // 显示路径，如 "A/B"
	Folder *Folder // nil 表示项目根目录
}

// FolderTargets 返回项目根目录 + 所有文件夹（用于下拉选择）。
func (p *Project) FolderTargets() []FolderTarget {
	out := []FolderTarget{{Path: "项目根目录"}}
	var walk func(fs []*Folder, prefix string)
	walk = func(fs []*Folder, prefix string) {
		for _, f := range fs {
			path := f.Name
			if prefix != "" {
				path = prefix + "/" + f.Name
			}
			out = append(out, FolderTarget{Path: path, Folder: f})
			walk(f.Folders, path)
		}
	}
	walk(p.Folders, "")
	return out
}

// TargetByPath 根据路径找到对应的 FolderTarget（找不到返回根目录）。
func (p *Project) TargetByPath(path string) FolderTarget {
	for _, t := range p.FolderTargets() {
		if t.Path == path {
			return t
		}
	}
	return FolderTarget{Path: "项目根目录"}
}

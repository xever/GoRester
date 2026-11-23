package storage

// 本文件保存用户偏好（目前只有界面语言）。与项目数据一样放在
// ~/Documents/gorester/ 下，文件名为 config.json。

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// configFileName 是用户偏好文件名。
const configFileName = "config.json"

// Preferences 是用户可修改的偏好设置。
type Preferences struct {
	// Language 为 "" 表示跟随系统语言，否则为 "zh" 或 "en"。
	Language string `json:"language,omitempty"`
}

// configFile 返回偏好文件路径。
func configFile() (string, error) {
	base, err := baseDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, configFileName), nil
}

// LoadPreferences 读取用户偏好；文件不存在或内容损坏时返回空偏好（跟随系统）。
func LoadPreferences() Preferences {
	path, err := configFile()
	if err != nil {
		return Preferences{}
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return Preferences{}
	}
	var pref Preferences
	if err := json.Unmarshal(data, &pref); err != nil {
		return Preferences{}
	}
	return pref
}

// SavePreferences 把用户偏好写回磁盘。
func SavePreferences(pref Preferences) error {
	path, err := configFile()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(pref, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0o644)
}

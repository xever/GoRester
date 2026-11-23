// Package i18n 保存 GoRester 的界面语言资源与翻译入口。
//
// 所有用户可见的文案都通过 T(key) 获取；具体语言的词条分别放在本目录下的
// zh.go / en.go 中。启动时根据系统语言（或用户在设置里保存的选择）决定当前
// 语言，切换语言后页面在下一帧自动使用新的词条。
package i18n

import (
	"strings"
	"sync"
)

// Language 是支持的语言。空字符串 Auto 表示跟随系统语言。
type Language string

const (
	Auto    Language = ""
	Chinese Language = "zh"
	English Language = "en"
)

var (
	mu      sync.RWMutex
	current Language = Chinese
	bundles          = map[Language]map[string]string{}
)

// register 由各语言资源文件在 init 中调用，把词条登记到对应语言。
func register(lang Language, table map[string]string) {
	if bundles[lang] == nil {
		bundles[lang] = make(map[string]string, len(table))
	}
	for key, value := range table {
		bundles[lang][key] = value
	}
}

// Current 返回当前界面语言。
func Current() Language {
	mu.RLock()
	defer mu.RUnlock()
	return current
}

// Set 切换界面语言。未知语言回退到中文。
func Set(lang Language) {
	if lang != Chinese && lang != English {
		lang = Chinese
	}
	mu.Lock()
	current = lang
	mu.Unlock()
}

// Resolve 根据用户的偏好和系统语言列表决定实际使用的语言。
// pref 为 Auto 时按系统语言的优先顺序选择了 zh / en；都不是时用中文。
func Resolve(pref string, system []string) Language {
	switch Language(pref) {
	case Chinese, English:
		return Language(pref)
	}
	for _, tag := range system {
		lower := strings.ToLower(tag)
		switch {
		case strings.HasPrefix(lower, "zh"):
			return Chinese
		case strings.HasPrefix(lower, "en"):
			return English
		}
	}
	return Chinese
}

// T 返回当前语言下 key 对应的文案。词条缺失时返回 key 本身，便于发现遗漏。
func T(key string) string {
	mu.RLock()
	lang := current
	mu.RUnlock()
	if table := bundles[lang]; table != nil {
		if value, ok := table[key]; ok {
			return value
		}
	}
	return key
}

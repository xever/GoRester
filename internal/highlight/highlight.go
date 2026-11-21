// Package highlight 负责响应体的格式化与语法高亮。
//
// 需求：JSON / XML 响应展示时带行号、格式化与语法高亮；内容过长时不格式化、
// 不高亮（避免大文本拖慢界面）。这里把响应体拆成「一行一个或多个着色片段」，
// 供响应区用 ui.List 渲染。
package highlight

import (
	"bytes"
	"encoding/json"
	"strings"

	"github.com/vivaui/viva/gfx"
)

// maxHighlightBytes 超过这个字节数就不再格式化 / 高亮，直接原文展示。
const maxHighlightBytes = 256 * 1024 // 256 KB

// Seg 是响应体里一段带颜色的文本。
type Seg struct {
	Text  string
	Color gfx.Color
}

// Line 是响应区的一行，由若干着色片段组成。
type Line struct {
	Segs []Seg
}

// BuildResponseLines 根据响应体和内容类型，生成带行号/高亮的行列表。
// 返回 (lines, highlighted)。highlighted 表示是否做了语法高亮。
func BuildResponseLines(body []byte, contentType string, theme gfx.Theme) ([]Line, bool) {
	text := string(body)

	// 内容过长：不格式化、不高亮，直接按行拆分。
	if len(body) > maxHighlightBytes {
		return PlainLines(text), false
	}

	if strings.Contains(contentType, "json") {
		if pretty, ok := PrettyJSON(text); ok {
			return highlightJSON(pretty, theme), true
		}
		return PlainLines(text), false
	}
	if strings.Contains(contentType, "xml") {
		pretty := PrettyXML(text)
		return highlightXML(pretty, theme), true
	}
	return PlainLines(text), false
}

// PlainLines 把文本按行拆分，每行一个普通颜色片段。
func PlainLines(text string) []Line {
	if text == "" {
		return nil
	}
	raw := strings.Split(text, "\n")
	lines := make([]Line, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, Line{Segs: []Seg{{Text: l}}})
	}
	return lines
}

// ---------------------------------------------------------------------------
// JSON 格式化
// ---------------------------------------------------------------------------

// PrettyJSON 用两空格缩进格式化 JSON 文本，返回 (结果, 是否成功)。
func PrettyJSON(s string) (string, bool) {
	var buf bytes.Buffer
	if err := json.Indent(&buf, []byte(s), "", "  "); err != nil {
		return s, false
	}
	return buf.String(), true
}

// ---------------------------------------------------------------------------
// JSON 语法高亮
// ---------------------------------------------------------------------------

// highlightJSON 把（已格式化的）JSON 文本拆成带颜色的行。
func highlightJSON(s string, theme gfx.Theme) []Line {
	colors := jsonColors{theme: theme}
	segs := colors.tokenize(s)

	// 把连续片段按换行分组。
	var lines []Line
	var cur Line
	flush := func() {
		if len(cur.Segs) > 0 {
			lines = append(lines, cur)
			cur = Line{}
		}
	}
	for _, seg := range segs {
		// 片段里可能包含换行，逐段处理。
		parts := strings.Split(seg.Text, "\n")
		for i, p := range parts {
			if i > 0 {
				flush() // 遇到换行，结束当前行
			}
			if p != "" {
				cur.Segs = append(cur.Segs, Seg{Text: p, Color: seg.Color})
			}
		}
	}
	flush()
	return lines
}

// jsonColors 保存 JSON 高亮用到的颜色。
type jsonColors struct {
	theme gfx.Theme
}

// tokenize 把 JSON 文本切成着色片段（整体一个平坦列表）。
func (jc jsonColors) tokenize(s string) []Seg {
	key := jc.theme.Accent
	str := jc.theme.Success
	num := jc.theme.Primary
	keyword := gfx.RGB(168, 85, 247) // 紫
	punct := jc.theme.MutedForeground

	var segs []Seg
	i, n := 0, len(s)
	for i < n {
		ch := s[i]
		switch {
		case ch == '"':
			// 读取一个字符串，判断是否为 key（后面跟冒号）。
			j := i + 1
			for j < n {
				if s[j] == '\\' {
					j += 2
					continue
				}
				if s[j] == '"' {
					break
				}
				j++
			}
			end := j + 1
			text := s[i:end]
			// 向前跳过空白，看是不是 key。
			k := end
			for k < n && (s[k] == ' ' || s[k] == '\t' || s[k] == '\n') {
				k++
			}
			if k < n && s[k] == ':' {
				segs = append(segs, Seg{Text: text, Color: key})
			} else {
				segs = append(segs, Seg{Text: text, Color: str})
			}
			i = end

		case ch == '{' || ch == '}' || ch == '[' || ch == ']' || ch == ':' || ch == ',':
			segs = append(segs, Seg{Text: s[i : i+1], Color: punct})
			i++

		case ch >= '0' && ch <= '9' || ch == '-' || ch == '+':
			j := i + 1
			for j < n {
				c := s[j]
				if (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '-' || c == '+' {
					j++
				} else {
					break
				}
			}
			segs = append(segs, Seg{Text: s[i:j], Color: num})
			i = j

		case isAlpha(ch):
			j := i
			for j < n && isAlpha(s[j]) {
				j++
			}
			segs = append(segs, Seg{Text: s[i:j], Color: keyword})
			i = j

		default:
			// 空白及其它字符按普通颜色保留，一次吞掉连续的一串。
			j := i
			for j < n {
				c := s[j]
				if c == '"' || c == '{' || c == '}' || c == '[' || c == ']' || c == ':' || c == ',' ||
					(c >= '0' && c <= '9') || c == '-' || c == '+' || isAlpha(c) {
					break
				}
				j++
			}
			segs = append(segs, Seg{Text: s[i:j], Color: jc.theme.Foreground})
			i = j
		}
	}
	return segs
}

func isAlpha(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

// ---------------------------------------------------------------------------
// XML 格式化与高亮（极简实现）
// ---------------------------------------------------------------------------

// PrettyXML 做一个极简的 XML 缩进：每个标签另起一行并按层级缩进。
func PrettyXML(s string) string {
	var b strings.Builder
	indent := 0
	inContent := false
	contentStart := 0

	for i := 0; i < len(s); i++ {
		ch := s[i]
		switch ch {
		case '<':
			if inContent && strings.TrimSpace(s[contentStart:i]) != "" {
				b.WriteString(strings.TrimSpace(s[contentStart:i]))
				b.WriteByte('\n')
				writeIndent(&b, indent)
			}
			inContent = false
			if i+1 < len(s) && s[i+1] == '/' {
				// 闭合标签。
				indent--
				if indent < 0 {
					indent = 0
				}
				writeIndent(&b, indent)
			} else {
				writeIndent(&b, indent)
			}
			b.WriteByte('<')
		case '>':
			inContent = false
			b.WriteByte('>')
			b.WriteByte('\n')
			// 自闭合标签不缩进层级；普通开标签缩进 +1。
			if i > 0 && s[i-1] != '/' {
				line := lastTag(s, i)
				if !strings.HasPrefix(line, "</") && !strings.HasPrefix(line, "<?") && !strings.HasPrefix(line, "<!--") {
					indent++
				}
			}
			inContent = true
			contentStart = i + 1
		}
	}
	return b.String()
}

func lastTag(s string, gtIndex int) string {
	start := gtIndex
	for start >= 0 && s[start] != '<' {
		start--
	}
	if start < 0 {
		return ""
	}
	return s[start : gtIndex+1]
}

func writeIndent(b *strings.Builder, n int) {
	for i := 0; i < n; i++ {
		b.WriteString("  ")
	}
}

// highlightXML 给 XML 做简单高亮：标签名/属性名用强调色，字符串属性值用绿色。
func highlightXML(s string, theme gfx.Theme) []Line {
	raw := strings.Split(s, "\n")
	lines := make([]Line, 0, len(raw))
	for _, l := range raw {
		lines = append(lines, Line{Segs: highlightXMLSegs(l, theme)})
	}
	return lines
}

func highlightXMLSegs(line string, theme gfx.Theme) []Seg {
	// 极简：把 <...> 内的属性值字符串涂绿，其余标签部分涂强调色。
	var segs []Seg
	i, n := 0, len(line)
	for i < n {
		if line[i] == '"' {
			j := i + 1
			for j < n && line[j] != '"' {
				j++
			}
			if j < n {
				j++
				segs = append(segs, Seg{Text: line[i:j], Color: theme.Success})
				i = j
				continue
			}
		}
		j := i + 1
		for j < n && line[j] != '"' {
			j++
		}
		segs = append(segs, Seg{Text: line[i:j], Color: theme.Foreground})
		i = j
	}
	return segs
}

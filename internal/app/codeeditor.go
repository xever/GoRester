package app

// 本文件实现请求体（json / xml / raw）的代码编辑器：语法高亮、编辑命令
// 与原生输入，以及编辑器文本模型与 model.Body.Raw 之间的双向同步。

import (
	"unicode/utf8"

	"github.com/vivaui/viva/components"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/text"
	"github.com/vivaui/viva/window"

	"gorester/internal/model"
)

// ---------------------------------------------------------------------------
// 编辑器与 body.Raw 的同步
// ---------------------------------------------------------------------------

// bindBodyEditor 把代码编辑器绑定到指定的请求体；切换目标时先回写旧值再载入新值。
func (s *projectSession) bindBodyEditor(body *model.Body) {
	if s.bodyEditorTarget == body {
		return
	}
	// 把上一个 body 里用户还没保存的编辑写回，避免丢失。
	if s.bodyEditorTarget != nil {
		s.bodyEditorTarget.Raw = s.bodyEditor.String()
	}
	s.bodyEditorTarget = body
	s.replaceEditorText(body.Raw)
}

// flushBodyEditor 把编辑器里的最新内容写回当前请求体（发送 / 保存前调用）。
func (s *projectSession) flushBodyEditor() {
	if s.bodyEditorTarget != nil {
		s.bodyEditorTarget.Raw = s.bodyEditor.String()
	}
}

// replaceEditorText 用给定文本整体替换编辑器内容（保持编辑器的实例与回调）。
func (s *projectSession) replaceEditorText(str string) {
	end := utf8.RuneCountInString(s.bodyEditor.String())
	s.bodyEditor.ReplaceRange(text.Range{Start: 0, End: end}, str)
}

// ---------------------------------------------------------------------------
// 编辑命令与原生输入
// ---------------------------------------------------------------------------

// bodyEditorKey 处理代码编辑器的按键：编辑命令与光标移动。
func (s *projectSession) bodyEditorKey(event gfx.KeyEvent) gfx.EventResult {
	if command, ok := components.CodeEditorCommandForKey(event); ok && command.Apply(s.bodyEditor) {
		return gfx.EventResult{Handled: true, Repaint: true}
	}
	movement, extend := components.TextareaMovementForKey(event.Key, event.Modifiers)
	if movement == components.TextareaMoveNone {
		return gfx.EventResult{}
	}
	value := s.bodyEditor.String()
	to := s.bodyCode.Code.Move(value, s.bodyEditor.ByteOffset(s.bodyEditor.Caret()), movement)
	s.bodyEditor.MoveTo(utf8.RuneCountInString(value[:to]), extend)
	return gfx.EventResult{Handled: true, Repaint: true}
}

// bodyEditorText 提供原生文本输入（打字、输入法、剪贴板），但把文字颜色设为
// 全透明：macOS 的原生编辑器只接收输入、不绘制任何东西（文字/光标/选区/输入法
// 组合都不画）。文字、光标、选区、语法高亮全部由 codeeditor 自己绘制，
// 从而实现所见即所得，也避免了原生输入框与绘制内容错位、失焦隐藏的问题。
func (s *projectSession) bodyEditorText(context window.TextInputContext) *window.TextInput {
	return &window.TextInput{
		Editor: s.bodyEditor,
		Color:  gfx.Color{Red: 255, Green: 255, Blue: 255, Alpha: 0}, // 全透明：只输入、不绘制
	}
}

// ---------------------------------------------------------------------------
// 语法高亮
// ---------------------------------------------------------------------------

// codeHighlight 是代码编辑器的单行语法高亮函数，给 JSON / XML 文本行着色。
// 返回的 CodeEditorRun 用字节范围标记行内的着色区间。
func codeHighlight(language, line string, dst []components.CodeEditorRun) []components.CodeEditorRun {
	dst = dst[:0]
	if language == "raw" {
		return dst // raw 不染色
	}

	key := gfx.RGB(110, 168, 255) // 键名 / 标签：蓝
	str := gfx.RGB(106, 190, 110) // 字符串：绿
	num := gfx.RGB(255, 166, 87)  // 数字：橙
	kw := gfx.RGB(210, 130, 255)  // true / false / null：紫
	tag := gfx.RGB(110, 168, 255) // XML 标签：蓝

	i, n := 0, len(line)
	for i < n {
		c := line[i]
		switch {
		case c == '"':
			// 字符串字面量。
			j := i + 1
			for j < n {
				if line[j] == '\\' {
					j += 2
					continue
				}
				if line[j] == '"' {
					break
				}
				j++
			}
			if j >= n {
				return dst // 未闭合，停止
			}
			end := j + 1
			// 后面跟着冒号则是 JSON 的键名。
			k := end
			for k < n && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			color := str
			if k < n && line[k] == ':' {
				color = key
			}
			dst = append(dst, components.CodeEditorRun{Start: i, End: end, Color: color})
			i = end

		case c == '<' && language == "xml":
			// XML 标签名。
			j := i + 1
			if j < n && line[j] == '/' {
				j++
			}
			start := j
			for j < n && isTagChar(line[j]) {
				j++
			}
			if j > start {
				dst = append(dst, components.CodeEditorRun{Start: start, End: j, Color: tag})
			}
			i = j

		case c >= '0' && c <= '9' || c == '-':
			j := i + 1
			for j < n && isNumChar(line[j]) {
				j++
			}
			dst = append(dst, components.CodeEditorRun{Start: i, End: j, Color: num})
			i = j

		case isAlphaByte(c):
			j := i
			for j < n && isAlphaByte(line[j]) {
				j++
			}
			word := line[i:j]
			if word == "true" || word == "false" || word == "null" {
				dst = append(dst, components.CodeEditorRun{Start: i, End: j, Color: kw})
			}
			i = j

		default:
			i++
		}
	}
	return dst
}

func isNumChar(c byte) bool {
	return (c >= '0' && c <= '9') || c == '.' || c == 'e' || c == 'E' || c == '+' || c == '-'
}

func isAlphaByte(c byte) bool {
	return (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
}

func isTagChar(c byte) bool {
	return isAlphaByte(c) || (c >= '0' && c <= '9') || c == '-' || c == '_' || c == ':'
}

package app

// 本文件是响应区：响应结果展示（Body / Cookie / Header / 请求）以及
// 复制、搜索、保存响应等操作。

import (
	"fmt"
	"os"
	"strings"

	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/highlight"
	"gorester/internal/model"
)

// ---------------------------------------------------------------------------
// 响应结果展示区
// ---------------------------------------------------------------------------

// responsePanel 渲染响应结果区（结果 Tabs + 工具栏 + 内容）。
func (s *projectSession) responsePanel(c ui.C) {
	theme := *c.Theme()
	ui.Column(c).Fill().Children(func() {
		// 结果 Tabs + 工具栏。
		ui.Row(c).PadX(12).PadTop(4).Items(ui.Center).Children(func() {
			ui.Tabs(c, &s.respTab, respTabIndices, respTabLabel)

			ui.Spacer(c).Grow(1)

			// 左侧工具栏：格式化 / 语言 / 编码下拉。
			ui.Select(c, &s.respFormat, prettyIndices, prettyLabel).Width(90).Label("格式化")
			ui.Select(c, &s.respLang, langOptions, identityLabel).Width(80).Label("语言")
			ui.Select(c, &s.respEncoding, encodingOptions, identityLabel).Width(80).Label("编码")

			// 右侧工具图标：提取 / 下载 / 复制 / 搜索。
			if ui.Button(c, "提取").Icon(iconBookmark).IconOnly().Ghost().Small().Clicked() {
				s.app.toaster.Show("提取", "提取功能暂未实现", ui.ToastInfo)
			}
			if ui.Button(c, "保存").Icon(iconDownload).IconOnly().Ghost().Small().Clicked() {
				s.app.post(s.saveResponse)
			}
			if ui.Button(c, "复制").Icon(iconCopy).IconOnly().Ghost().Small().Clicked() {
				s.copyResponse()
			}
			if ui.Button(c, "搜索").Icon(iconSearch).IconOnly().Ghost().Small().Clicked() {
				s.respFindOpen = !s.respFindOpen
			}
		})

		// 搜索栏。
		if s.respFindOpen {
			ui.Row(c).PadX(12).Gap(8).Items(ui.Center).Children(func() {
				ui.Spacer(c)
				ui.TextField(c, &s.respSearch, "查找", "输入关键字").Width(200)
				if ui.Button(c, "查找下一个").Ghost().Small().Clicked() {
					s.findNext()
				}
			})
		}

		ui.Divider(c)

		// 内容展示区。
		ui.Scroll(c).Grow(1).Children(func() {
			switch s.respTab {
			case 0:
				s.responseBody(c, theme)
			case 1:
				s.responseKVs(c, s.responseCookies())
			case 2:
				s.responseKVs(c, s.responseHeaders())
			case 3:
				s.requestView(c, theme)
			}
		})
	})
}

// requestView 渲染「实际请求」：最近一次实际发送的请求的方法、URL、请求头与请求体。
func (s *projectSession) requestView(c ui.C, theme gfx.Theme) {
	if s.lastReq.Method == "" {
		ui.Column(c).Fill().Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "尚未发送请求").Muted()
		})
		return
	}
	ui.Column(c).Fill().Pad(16).Gap(8).Children(func() {
		// —— 方法 + URL ——
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			methodBadge(c, s.lastReq.Method)
			ui.Text(c, s.lastReq.Method).Color(methodColor(s.lastReq.Method)).Weight(gfx.WeightSemibold)
			ui.Text(c, s.lastReq.URL).Mono().FontSize(13)
		})

		ui.Divider(c)

		// —— 请求头 ——
		ui.Text(c, "请求头").Weight(gfx.WeightSemibold)
		if len(s.lastReq.Headers) == 0 {
			ui.Text(c, "(无)").Muted()
		} else {
			for _, h := range s.lastReq.Headers {
				ui.Row(c).Gap(12).Children(func() {
					ui.Text(c, h.Key).Mono().FontSize(12).Weight(gfx.WeightMedium).Width(200)
					ui.Text(c, h.Value).Mono().FontSize(12)
				})
			}
		}

		ui.Divider(c)

		// —— 请求体 ——
		ui.Text(c, "请求体").Weight(gfx.WeightSemibold)
		if !s.lastReq.HasBody {
			ui.Text(c, "(无请求体)").Muted()
			return
		}
		if s.lastReq.ContentType != "" {
			ui.Text(c, "类型："+s.lastReq.ContentType).Muted().Small()
		}
		// 有文件时显示文件路径与大小。
		for _, f := range s.lastReq.Files {
			ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
				ui.Icon(c, iconFile).Color(theme.MutedForeground)
				ui.Text(c, f.Path).Mono().FontSize(12)
				ui.Text(c, humanBytes(int(f.Size))).Muted().Small()
			})
		}
		if s.lastReq.Body != "" {
			ui.TextArea(c, &s.lastReq.Body, "", "").
				Mono().FontSize(14).ReadOnly(true).FillWidth().AutoGrow(5)
		}
	})
}

// responseBody 渲染响应体（带行号、高亮）。
func (s *projectSession) responseBody(c ui.C, theme gfx.Theme) {
	if s.response == nil {
		ui.Column(c).Fill().Items(ui.Center).Justify(ui.Center).Children(func() {
			ui.Text(c, "暂无响应").Muted()
		})
		return
	}
	if s.response.Err != nil {
		ui.Column(c).Pad(16).Children(func() {
			ui.Text(c, "请求出错").Color(gfx.RGB(239, 68, 68)).Weight(gfx.WeightMedium)
			ui.Text(c, s.response.Err.Error()).Muted()
		})
		return
	}

	// Raw 模式直接用原始字节按行展示。
	if s.respFormat == 1 {
		s.renderBodyLines(c, highlight.PlainLines(s.response.BodyString()), theme)
		return
	}
	s.renderBodyLines(c, s.respLines, theme)
}

// renderBodyLines 用 ui.List 渲染带行号的内容。
func (s *projectSession) renderBodyLines(c ui.C, lines []highlight.Line, theme gfx.Theme) {
	if len(lines) == 0 {
		ui.Text(c, "(空响应体)").Muted().Pad(12)
		return
	}
	needle := s.respSearch
	ui.List(c, &s.respList, len(lines), func(row ui.ListRow) {
		line := lines[row.Index]
		row.Element.PadX(12).PadY(1)
		ui.Row(row.C).Gap(16).Items(ui.Start).Children(func() {
			// 行号。
			ui.Text(row.C, fmt.Sprintf("%d", row.Index+1)).Color(theme.MutedForeground).Mono().FontSize(14).Width(40).TextEnd()
			// 内容片段。
			ui.Row(row.C).Children(func() {
				for _, seg := range line.Segs {
					if needle == "" {
						ui.Text(row.C, seg.Text).Color(seg.Color).Mono().FontSize(14)
						continue
					}
					// 有搜索词时，把片段按命中位置拆开，命中的部分加浅黄色底色、
					// 文字改用深色以保证在浅黄底上清晰可读。
					for _, part := range splitSearch(seg.Text, needle) {
						if part.matched {
							ui.Text(row.C, part.text).Color(gfx.RGB(25, 25, 25)).Background(gfx.RGB(255, 235, 120)).Mono().FontSize(14)
						} else {
							ui.Text(row.C, part.text).Color(seg.Color).Mono().FontSize(14)
						}
					}
				}
			})
		})
	}).Grow(1)
}

// searchPart 是响应内容里被搜索拆出的一段：text 为文本，matched 表示是否命中关键字。
type searchPart struct {
	text    string
	matched bool
}

// splitSearch 把 text 按 needle（大小写不敏感）拆成命中/未命中的片段。
func splitSearch(text, needle string) []searchPart {
	if needle == "" {
		return []searchPart{{text: text}}
	}
	lower := strings.ToLower(text)
	lowerNeedle := strings.ToLower(needle)
	var parts []searchPart
	start := 0
	for start <= len(text) {
		rel := strings.Index(lower[start:], lowerNeedle)
		if rel < 0 {
			if start < len(text) {
				parts = append(parts, searchPart{text: text[start:]})
			}
			break
		}
		idx := start + rel
		if idx > start {
			parts = append(parts, searchPart{text: text[start:idx]})
		}
		parts = append(parts, searchPart{text: text[idx : idx+len(needle)], matched: true})
		start = idx + len(needle)
	}
	if len(parts) == 0 {
		return []searchPart{{text: text}}
	}
	return parts
}

// responseKVs 渲染响应 Header / Cookie 键值对（只读展示）。
func (s *projectSession) responseKVs(c ui.C, kvs []model.KeyValue) {
	if len(kvs) == 0 {
		ui.Text(c, "(无数据)").Muted().Pad(12)
		return
	}
	ui.Column(c).Pad(12).Gap(4).Children(func() {
		for i := range kvs {
			kv := kvs[i]
			ui.Row(c).Gap(12).Children(func() {
				ui.Text(c, kv.Key).Mono().FontSize(12).Weight(gfx.WeightMedium).Width(240)
				ui.Text(c, kv.Value).Mono().FontSize(12)
			})
		}
	})
}

// responseHeaders 返回响应头键值对。
func (s *projectSession) responseHeaders() []model.KeyValue {
	if s.response == nil {
		return nil
	}
	return s.response.Headers
}

// responseCookies 从响应头里解析 Set-Cookie。
func (s *projectSession) responseCookies() []model.KeyValue {
	if s.response == nil {
		return nil
	}
	var out []model.KeyValue
	for _, h := range s.response.Headers {
		if strings.EqualFold(h.Key, "Set-Cookie") {
			out = append(out, model.KeyValue{Key: "Set-Cookie", Value: h.Value, Enabled: true})
		}
	}
	return out
}

// findNext 在响应体里查找关键字并滚动到第一个匹配行。
func (s *projectSession) findNext() {
	if s.respSearch == "" || len(s.respLines) == 0 {
		return
	}
	needle := strings.ToLower(s.respSearch)
	for i, line := range s.respLines {
		for _, seg := range line.Segs {
			if strings.Contains(strings.ToLower(seg.Text), needle) {
				s.respList.ScrollIntoView(i)
				return
			}
		}
	}
	s.app.toaster.Show("未找到", "没有匹配「"+s.respSearch+"」的内容", ui.ToastInfo)
}

// copyResponse 把响应体复制到剪贴板。
func (s *projectSession) copyResponse() {
	if s.response == nil {
		return
	}
	if err := vivaapp.SetClipboardText(s.response.BodyString()); err != nil {
		s.app.toaster.Show("复制失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("已复制", "响应体已复制到剪贴板", ui.ToastSuccess)
}

// saveResponse 把响应体保存到文件。
func (s *projectSession) saveResponse() {
	defer s.app.invalidate() // 保存后重绘，让 toast 显示
	if s.response == nil {
		return
	}
	ext := "txt"
	if strings.Contains(s.responseContentType(), "json") {
		ext = "json"
	} else if strings.Contains(s.responseContentType(), "xml") {
		ext = "xml"
	}
	path, err := vivaapp.SaveFile(vivaapp.SaveFileOptions{
		Title:            "下载响应",
		SuggestedName:    "response." + ext,
		DefaultExtension: ext,
	})
	if err != nil {
		return
	}
	if err := os.WriteFile(path, s.response.Body, 0o644); err != nil {
		s.app.toaster.Show("下载失败", err.Error(), ui.ToastError)
		return
	}
	s.app.toaster.Show("已下载", path, ui.ToastSuccess)
}

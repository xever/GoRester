package app

// 本文件是请求区：URL 请求构造栏、请求参数配置区（Params / Body / Cookie /
// Header / Auth）以及请求字段访问与请求体格式化工具。

import (
	"encoding/json"
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
	"github.com/vivaui/viva/ui/codeeditor"

	"gorester/internal/highlight"
	"gorester/internal/model"
)

// urlBar 渲染 URL 请求构造栏。
func (s *projectSession) urlBar(c ui.C) {
	theme := c.Theme()
	f := s.requestFields()
	ui.Row(c).FillWidth().Pad(12).Gap(8).Items(ui.Center).Children(func() {
		if f == nil {
			return
		}
		// —— 方法选择器：用 Row + 彩色文字实现，方法名显示对应颜色 ——
		methodBox := ui.Row(c).PadX(10).Height(theme.Metrics.ControlHeight).Radius(theme.Metrics.RadiusControl).Gap(4).Items(ui.Center).
			Background(theme.Card).Border(1, theme.Border).
			Hit(ui.ID(c), "请求方法").Hover(ui.Look{}.Background(theme.Hover)).
			Children(func() {
				ui.Text(c, *f.method).Color(methodColor(*f.method)).Weight(gfx.WeightSemibold)
				ui.Icon(c, iconChevronDown).Size(12, 12).Color(theme.MutedForeground)
			})
		if methodBox.Clicked() {
			s.methodMenuOpen = !s.methodMenuOpen
		}
		ui.Menu(c, methodBox, &s.methodMenuOpen, func(menu *ui.MenuItems) {
			for _, m := range methodStrings {
				if menu.Item(m).Clicked() {
					*f.method = m
				}
			}
		})

		// —— URL 输入框 ——
		ui.TextField(c, f.path, "请求路径", "/api/users").Grow(1)

		// —— 发送——
		if ui.Button(c, "发送").Icon(iconSend).Clicked() {
			s.doSend()
		}

		// —— 保存 / 保存为用例 ——
		if ui.Button(c, "保存").Icon(iconSave).Secondary().Clicked() {
			s.saveCurrent()
		}
		if ui.Button(c, "保存用例").Secondary().Clicked() {
			s.saveCaseName = ""
			s.saveCaseTarget = s.currentInterface()
			s.saveCaseOpen = true
		}
	})
}

// requestConfig 渲染请求参数配置区（Params / Body / Cookie / Header / Auth）。
func (s *projectSession) requestConfig(c ui.C) {
	f := s.requestFields()
	ui.Column(c).Fill().Children(func() {
		ui.Tabs(c, &s.reqTab, reqTabIndices, reqTabLabel).PadX(12)
		if f == nil {
			return
		}
		// Body 标签需要撑满高度（json/xml/raw 代码编辑器内部滚动），单独处理。
		if s.reqTab == 1 {
			s.bodyPanel(c, f.body)
			return
		}
		// 其它标签用可滚动区域。
		ui.Scroll(c).Grow(1).Pad(12).Children(func() {
			switch s.reqTab {
			case 0:
				s.kvEditor(c, f.params, kvColumns{valueLabel: "参数值"})
			case 2:
				s.kvEditor(c, f.cookies, kvColumns{valueLabel: "Cookie 值"})
			case 3:
				s.kvEditor(c, f.headers, kvColumns{valueLabel: "Header 值"})
			case 4:
				s.authPanel(c, f.auth)
			}
		})
	})
}

// bodyPanel 渲染请求体配置面板（格式子 Tab + 键值表 / 代码编辑器）。
func (s *projectSession) bodyPanel(c ui.C, body *model.Body) {
	ui.Column(c).Fill().Children(func() {
		// 格式子 Tab。
		ui.Tabs(c, &s.bodyFormat, bodyFormatIndices, bodyFormatLabel).
			Line().PadX(1)
		body.Mode = bodyFormatModes[s.bodyFormat]

		switch body.Mode {
		case "form-data":
			ui.Scroll(c).Grow(1).Pad(12).Children(func() {
				s.kvEditor(c, &body.FormData, kvColumns{withType: true, valueLabel: "示例值"})
			})
		case "x-www-form-urlencoded":
			ui.Scroll(c).Grow(1).Pad(12).Children(func() {
				s.kvEditor(c, &body.URLEncoded, kvColumns{withType: true, valueLabel: "示例值"})
			})
		case "json", "xml", "raw":
			s.bodyCodeEditor(c, body)
		default:
			ui.Text(c, "此接口没有请求体").Muted().Pad(12)
		}
	})
}

// bodyCodeEditor 渲染 json / xml / raw 的代码编辑器（撑满高度 + 语法高亮 + 内部滚动）。
func (s *projectSession) bodyCodeEditor(c ui.C, body *model.Body) {
	// 先同步：目标 body 变化时回写旧值并载入新值。
	s.bindBodyEditor(body)

	// 设置语言与高亮。
	s.bodyCode.Snapshot.Language = body.Mode
	s.bodyCode.Snapshot.Highlight = codeHighlight

	ui.Column(c).Fill().Pad(12).Gap(8).Children(func() {
		ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
			ui.Text(c, "内容").Muted().Small()
			ui.Spacer(c).Grow(1)
			if ui.Button(c, "格式化").Ghost().Small().Clicked() {
				// 以编辑器当前内容为准，格式化后写回 body 与编辑器。
				formatted := formatBody(s.bodyEditor.String(), body.Mode)
				body.Raw = formatted
				s.replaceEditorText(formatted)
			}
			// if ui.Button(c, "压缩").Ghost().Small().Clicked() {
			// 	compressed := minifyBody(s.bodyEditor.String(), body.Mode)
			// 	body.Raw = compressed
			// 	s.replaceEditorText(compressed)
			// }
		})
		// 代码编辑器撑满剩余高度，内容超出时内部滚动。
		codeeditor.View(c, "请求体内容", &s.bodyCode).Fill()
	})
}

// authPanel 渲染认证配置面板。
func (s *projectSession) authPanel(c ui.C, auth *model.Auth) {
	ui.Column(c).Gap(8).Children(func() {
		ui.Select(c, &auth.Type, authTypeNames, identityLabel).Width(160).Label("认证类型")
		switch auth.Type {
		case "basic":
			ui.TextField(c, &auth.Username, "用户名", "user").FillWidth()
			ui.TextField(c, &auth.Password, "密码", "").Secure().FillWidth()
		case "bearer":
			ui.TextField(c, &auth.Token, "Token", "Bearer token").FillWidth()
		case "api-key":
			ui.TextField(c, &auth.Key, "键名", "X-Api-Key").FillWidth()
			ui.TextField(c, &auth.Value, "值", "").FillWidth()
			ui.Select(c, &auth.In, apiKeyInOptions, identityLabel).Width(160).Label("位置")
		}
	})
}

// ---------------------------------------------------------------------------
// 请求字段访问
// ---------------------------------------------------------------------------

// currentInterface 返回当前标签页正在编辑的接口（编辑用例时返回其所属接口）。
func (s *projectSession) currentInterface() *model.Interface {
	tab := s.currentTab()
	if tab == nil {
		return nil
	}
	if tab.kind == tabInterface {
		return tab.iface
	}
	if tab.kind == tabCase {
		return s.project.FindInterface(tab.parentID)
	}
	return nil
}

// requestFields 返回当前标签页正在编辑的请求字段的指针。
// 编辑会直接写回模型里的接口 / 用例。
type requestFields struct {
	method  *string
	path    *string
	params  *[]model.KeyValue
	headers *[]model.KeyValue
	cookies *[]model.KeyValue
	body    *model.Body
	auth    *model.Auth
}

func (s *projectSession) requestFields() *requestFields {
	tab := s.currentTab()
	if tab == nil {
		return nil
	}
	if tab.kind == tabInterface && tab.iface != nil {
		i := tab.iface
		return &requestFields{&i.Method, &i.Path, &i.Params, &i.Headers, &i.Cookies, &i.Body, &i.Auth}
	}
	if tab.kind == tabCase && tab.case_ != nil {
		cc := tab.case_
		return &requestFields{&cc.Method, &cc.Path, &cc.Params, &cc.Headers, &cc.Cookies, &cc.Body, &cc.Auth}
	}
	return nil
}

// ---------------------------------------------------------------------------
// 小工具
// ---------------------------------------------------------------------------

// formatBody 根据模式格式化请求体文本。
func formatBody(raw, mode string) string {
	switch mode {
	case "json":
		if pretty, ok := highlight.PrettyJSON(raw); ok {
			return pretty
		}
	case "xml":
		return highlight.PrettyXML(raw)
	}
	return raw
}

// minifyBody 压缩请求体文本（去掉多余空白）。
func minifyBody(raw, mode string) string {
	if mode == "json" {
		var v interface{}
		if err := json.Unmarshal([]byte(raw), &v); err == nil {
			if b, err := json.Marshal(v); err == nil {
				return string(b)
			}
		}
	}
	return strings.TrimSpace(raw)
}

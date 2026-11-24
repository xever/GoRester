package app

// 本文件实现可复用的「键值对表格」编辑器，用于请求参数(Params)、
// 请求头(Header)、Cookie 以及 Body 表单(form-data / x-www-form-urlencoded)。
//
// 每一行直接绑定到 []model.KeyValue 的某个元素上，用户编辑即写回模型；
// 「添加一行」/「删除」就地修改切片。表头与列宽用固定宽度对齐。

import (
	vivaapp "github.com/vivaui/viva/app"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
	"gorester/internal/model"
)

// 列宽常量（保证各行对齐）。
const (
	checkboxWidth = 20.0
	kvKeyWidth    = 130.0
	kvValueWidth  = 220.0
	kvTypeWidth   = 90.0
	kvDescWidth   = 220.0
)

// kvColumns 描述一张键值对表要显示哪些列。
type kvColumns struct {
	withType   bool   // 是否显示「类型」列（body 表单用）
	valueLabel string // 值列的表头文字（参数值 / 示例值）
}

// kvEditor 渲染一张可编辑的键值对表。
// rows 是 *[]model.KeyValue，直接修改它即可。
func (s *projectSession) kvEditor(c ui.C, rows *[]model.KeyValue, cols kvColumns) {
	// —— 表头 ——
	// 列宽与 kvRow 的输入框一一对应。Spacer 会吸收剩余空间，占位不能用它，
	// 否则窗口变化时表头会被推到右边；用固定宽度的 Label 占位，并禁止文字收缩，
	// 这样表头与数据行始终对齐。
	ui.Row(c).Gap(8).Items(ui.Center).PadBottom(4).Children(func() {
		ui.Label(c, "").Width(checkboxWidth) // 复选框列占位
		ui.Text(c, i18n.T("kv.name")).Muted().Small().Width(kvKeyWidth).Shrink(0).TextCenter()
		if cols.withType {
			ui.Text(c, i18n.T("kv.type")).Muted().Small().Width(kvTypeWidth).Shrink(0).TextCenter()
		}
		ui.Text(c, cols.valueLabel).Muted().Small().Width(kvValueWidth).Shrink(0).TextCenter()
		ui.Text(c, i18n.T("kv.description")).Muted().Small().Width(kvDescWidth).Shrink(0).TextCenter()
		if cols.withType {
			ui.Label(c, "").Width(20) // 「必填」复选框占位
		}
		ui.Label(c, "").Width(36) // 删除按钮占位
	})

	// —— 数据行 ——
	delIdx := -1
	for i := range *rows {
		kv := &(*rows)[i]
		if s.kvRow(c, kv, cols) {
			delIdx = i
		}
	}
	// 点击删除后统一移除，避免循环中修改下标。
	if delIdx >= 0 {
		*rows = append((*rows)[:delIdx], (*rows)[delIdx+1:]...)
	}

	// —— 添加一行 ——
	if ui.Button(c, i18n.T("kv.addRow")).Icon(iconPlus).Ghost().Clicked() {
		*rows = append(*rows, model.KeyValue{Enabled: true})
	}
}

// kvRow 渲染一行键值对，返回 true 表示点了删除。
func (s *projectSession) kvRow(c ui.C, kv *model.KeyValue, cols kvColumns) bool {
	deleted := false
	ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
		// 启用开关。
		ui.Checkbox(c, &kv.Enabled, "").Width(checkboxWidth)
		// 参数名。
		ui.TextField(c, &kv.Key, i18n.T("kv.name"), "key").
			MinWidth(kvKeyWidth).MaxWidth(kvKeyWidth * 2)
		// 类型（可选列）。
		if cols.withType {
			ui.Select(c, &kv.Type, formFieldTypes, identityLabel).
				Width(kvTypeWidth).Label(i18n.T("kv.type"))
		}
		// 值 / 示例值。
		valueField := ui.TextField(c, &kv.Value, cols.valueLabel, "value").Width(kvValueWidth)
		// form-data 类型为 file 时，右侧加一个文件选择图标；点击弹出系统文件选择框，
		// 选中后把文件路径写回值字段（发送请求时按文件路径上传）。
		if cols.withType && kv.Type == "file" {
			valueField.Action(iconFolder, i18n.T("kv.selectFile"))
			if valueField.ActionClicked() {
				// 文件对话框会阻塞，经 Post 投递到本轮绘制之后，避免重入崩溃。
				s.app.post(func() {
					if path, ok := pickFilePath(); ok {
						kv.Value = path
						s.app.invalidate()
					}
				})
			}
		}
		// 说明。
		ui.TextField(c, &kv.Desc, i18n.T("kv.description"), "").Width(kvDescWidth)
		// 必填（body 表单）—— 用一个带星号的复选框表示。
		if cols.withType {
			ui.Checkbox(c, &kv.Required, "*").Tooltip(i18n.T("kv.required"))
		}
		// 删除。只留图标，去掉i18n.T("common.delete")
		if ui.Button(c, "").Icon(iconTrash).IconOnly().Ghost().Small().Clicked() {
			deleted = true
		}
	})
	return deleted
}

// pickFilePath 弹出系统文件选择框，返回选中的文件路径；取消或失败返回 (空, false)。
func pickFilePath() (string, bool) {
	paths, err := vivaapp.OpenFiles(vivaapp.OpenFileOptions{
		Title:         i18n.T("kv.selectFile"),
		AllowMultiple: false,
	})
	if err != nil || len(paths) == 0 {
		return "", false
	}
	return paths[0], true
}

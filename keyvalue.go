package main

// 本文件实现可复用的「键值对表格」编辑器，用于请求参数(Params)、
// 请求头(Header)、Cookie 以及 Body 表单(form-data / x-www-form-urlencoded)。
//
// 每一行直接绑定到 []KeyValue 的某个元素上，用户编辑即写回模型；
// 「添加一行」/「删除」就地修改切片。表头与列宽用固定宽度对齐。

import (
	"github.com/vivaui/viva/ui"
)

// 列宽常量（保证各行对齐）。
const (
	kvKeyWidth   = 180.0
	kvValueWidth = 220.0
	kvTypeWidth  = 90.0
	kvDescWidth  = 180.0
)

// kvColumns 描述一张键值对表要显示哪些列。
type kvColumns struct {
	withType   bool   // 是否显示「类型」列（body 表单用）
	valueLabel string // 值列的表头文字（参数值 / 示例值）
}

// kvEditor 渲染一张可编辑的键值对表。
// rows 是 *[]KeyValue，直接修改它即可。
func kvEditor(c ui.C, rows *[]KeyValue, cols kvColumns) {
	// —— 表头 ——
	ui.Row(c).Gap(8).PadBottom(4).Children(func() {
		ui.Spacer(c).Width(20) // 复选框占位
		ui.Text(c, "参数名").Muted().Small().Width(kvKeyWidth)
		if cols.withType {
			ui.Text(c, "类型").Muted().Small().Width(kvTypeWidth)
		}
		ui.Text(c, cols.valueLabel).Muted().Small().Width(kvValueWidth)
		ui.Text(c, "说明").Muted().Small().Width(kvDescWidth)
		ui.Spacer(c).Width(56) // 必填 + 删除按钮占位
	})

	// —— 数据行 ——
	delIdx := -1
	for i := range *rows {
		kv := &(*rows)[i]
		if kvRow(c, kv, cols) {
			delIdx = i
		}
	}
	// 点击删除后统一移除，避免循环中修改下标。
	if delIdx >= 0 {
		*rows = append((*rows)[:delIdx], (*rows)[delIdx+1:]...)
	}

	// —— 添加一行 ——
	if ui.Button(c, "添加一行").Icon(iconPlus).Ghost().Clicked() {
		*rows = append(*rows, KeyValue{Enabled: true})
	}
}

// kvRow 渲染一行键值对，返回 true 表示点了删除。
func kvRow(c ui.C, kv *KeyValue, cols kvColumns) bool {
	deleted := false
	ui.Row(c).Gap(8).Items(ui.Center).Children(func() {
		// 启用开关。
		ui.Checkbox(c, &kv.Enabled, "")
		// 参数名。
		ui.TextField(c, &kv.Key, "参数名", "key").Width(kvKeyWidth)
		// 类型（可选列）。
		if cols.withType {
			ui.Select(c, &kv.Type, formFieldTypes, identityLabel).Width(kvTypeWidth).Label("类型")
		}
		// 值 / 示例值。
		ui.TextField(c, &kv.Value, cols.valueLabel, "value").Width(kvValueWidth)
		// 说明。
		ui.TextField(c, &kv.Desc, "说明", "").Width(kvDescWidth)
		// 必填（body 表单）—— 用一个带星号的复选框表示。
		if cols.withType {
			ui.Checkbox(c, &kv.Required, "*").Tooltip("是否必填")
		}
		// 删除。
		if ui.Button(c, "删除").Icon(iconTrash).IconOnly().Ghost().Small().Clicked() {
			deleted = true
		}
	})
	return deleted
}

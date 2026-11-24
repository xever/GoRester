package app

// 本文件集中 HTTP 方法相关的展示辅助：方法配色、首字母徽标，以及
// 树行/实际请求视图里共用的方法徽标组件。

import (
	"strings"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// methodColor 根据 HTTP 方法返回一个主题颜色（用于方法徽标与下拉框）。
// 采用业界通用配色：GET 绿、POST 橙、PUT 蓝、DELETE 红、PATCH 紫。
func methodColor(method string) gfx.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return gfx.RGB(34, 197, 94) // 绿色
	case "POST":
		return gfx.RGB(249, 115, 22) // 橙色
	case "PUT":
		return gfx.RGB(59, 130, 246) // 蓝色
	case "DELETE":
		return gfx.RGB(239, 68, 68) // 红色
	case "PATCH":
		return gfx.RGB(168, 85, 247) // 紫色
	default:
		return gfx.RGB(107, 114, 128) // 灰色
	}
}

// methodLetter 返回方法的单个字母（用于树里的圆形徽标）。
func methodLetter(method string) string {
	if method == "" {
		return "?"
	}
	return strings.ToUpper(method[:1])
}

// methodBadge 渲染一个带字母的方法徽标（圆形小方块），颜色随方法变化。
func methodBadge(c ui.C, method string) {
	ui.Text(c, methodLetter(method)).
		Color(gfx.RGB(255, 255, 255)).
		Background(methodColor(method)).
		Size(20, 20).
		Radius(4).
		TextCenter().
		Weight(gfx.WeightSemibold).
		FontSize(11)
}

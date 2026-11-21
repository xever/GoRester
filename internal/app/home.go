package app

// 本文件是主页：以方格卡片展示所有项目，最左上角是「新建」卡片。

import (
	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"
)

// 主页卡片尺寸。
const (
	homeCardWidth  = 200
	homeCardHeight = 140
)

// homePage 渲染主页。
func (m *appModel) homePage(c ui.C) {
	theme := c.Theme()

	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Fill().Pad(28).Gap(20).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, "我的项目").FontSize(theme.Metrics.FontSizeL).Weight(gfx.WeightSemibold)
				ui.Text(c, "点击项目卡片打开，或点击「新建」创建一个项目。").Muted()
			})

			ui.Flow(c).Gap(16).Children(func() {
				// —— 新建项目卡片（左上角，蓝色）——
				newCard := ui.Column(c).Size(homeCardWidth, homeCardHeight).
					Radius(theme.Metrics.RadiusSurface).
					Background(gfx.RGB(59, 130, 246)).
					Hit(ui.ID(c), "新建项目").Hover(ui.Look{}.Opacity(0.9)).Transition().
					Items(ui.Center).Justify(ui.Center).Gap(8).
					Children(func() {
						ui.Icon(c, iconPlus).Color(gfx.RGB(255, 255, 255))
						ui.Text(c, "新建").Color(gfx.RGB(255, 255, 255)).Weight(gfx.WeightMedium)
					})
				if newCard.Clicked() {
					m.newName = ""
					m.newDesc = ""
					m.newProjectOpen = true
				}

				// —— 已有项目卡片 ——
				for i := range m.projects {
					meta := m.projects[i]
					card := ui.Column(c).Size(homeCardWidth, homeCardHeight).
						Radius(theme.Metrics.RadiusSurface).
						Background(theme.Card).
						Hit(ui.ID(c), "项目 "+meta.Name).
						Hover(ui.Look{}.Opacity(0.85)).Transition().
						Pad(16).Gap(6).
						Children(func() {
							ui.Text(c, meta.Name).Weight(gfx.WeightMedium)
							ui.Text(c, meta.Description).Muted().MaxLines(3)
						})
					if card.Clicked() {
						m.openProject(meta.Name)
					}
				}
			})
		})
	})

	// —— 新建项目弹窗 ——
	ui.Modal(c, &m.newProjectOpen, "新建项目",
		func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.TextField(c, &m.newName, "项目名称", "例如：用户中心").FillWidth()
				ui.TextField(c, &m.newDesc, "项目说明", "一句话描述这个项目").FillWidth()
			})
		},
		func() {
			if ui.Button(c, "取消").Outline().Clicked() {
				m.newProjectOpen = false
			}
			if ui.Button(c, "创建").Clicked() {
				m.newProjectOpen = false
				m.createProject(m.newName, m.newDesc)
			}
		},
	)
}

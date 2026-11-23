package app

// 本文件是主页：以方格卡片展示所有项目，最左上角是「新建」卡片。

import (
	"fmt"

	"github.com/vivaui/viva/gfx"
	"github.com/vivaui/viva/ui"

	"gorester/internal/i18n"
)

// 主页卡片尺寸。
const (
	homeCardWidth  = 200
	homeCardHeight = 120
)

// homePage 渲染主页。
func (m *appModel) homePage(c ui.C) {
	theme := c.Theme()

	ui.Scroll(c).Fill().Children(func() {
		ui.Column(c).Fill().Pad(28).Gap(20).Children(func() {
			ui.Column(c).Gap(4).Children(func() {
				ui.Text(c, i18n.T("home.title")).FontSize(theme.Metrics.FontSizeL).Weight(gfx.WeightSemibold)
				ui.Text(c, i18n.T("home.hint")).Muted()
			})

			ui.Flow(c).Gap(16).Children(func() {
				// —— 新建项目卡片（左上角，蓝色）——
				newCard := ui.Column(c).Size(homeCardWidth, homeCardHeight).
					Radius(theme.Metrics.RadiusSurface).
					Background(gfx.RGB(59, 130, 246)).
					Hit(ui.ID(c), i18n.T("home.newCard")).Hover(ui.Look{}.Opacity(0.9)).Transition().
					Items(ui.Center).Justify(ui.Center).Gap(8).
					Children(func() {
						ui.Icon(c, iconPlus).Color(gfx.RGB(255, 255, 255))
						ui.Text(c, i18n.T("common.new")).Color(gfx.RGB(255, 255, 255)).Weight(gfx.WeightMedium)
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
						Background(theme.Card).Border(0.5, theme.Ring).Shadow(gfx.ShadowMD).
						Hit(ui.ID(c), fmt.Sprintf(i18n.T("common.project"), meta.Name)).
						Hover(ui.Look{}.Opacity(0.85)).Transition().
						Pad(16).Gap(6).
						Children(func() {
							ui.Text(c, meta.Name).Weight(gfx.WeightMedium).Center()
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
	ui.Modal(c, &m.newProjectOpen, i18n.T("home.newCard"),
		func() {
			ui.Column(c).Gap(12).Children(func() {
				ui.TextField(c, &m.newName, i18n.T("home.name"), i18n.T("home.namePlaceholder")).FillWidth()
				ui.TextArea(c, &m.newDesc, i18n.T("home.desc"), i18n.T("home.descPlaceholder")).FillWidth().AutoGrow(5)
			})
		},
		func() {
			if ui.Button(c, i18n.T("common.cancel")).Outline().Clicked() {
				m.newProjectOpen = false
			}
			if ui.Button(c, i18n.T("common.create")).Clicked() {
				m.newProjectOpen = false
				m.createProject(m.newName, m.newDesc)
			}
		},
	)
}

// Package home 实现工作区首页。
package home

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制工作区首页。
func View(c *ui.Context) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c)
	})
}

func content(c *ui.Context) {
	m := theme.Metrics
	v1.PageHeader(c, locales.WorkspaceTitle(), locales.WorkspaceSubtitle())
	ui.Row(c).Gap(m.Space3).Padding(0, m.PagePaddingX).
		AlignItems(ui.Stretch).Children(func() {
		card(c, locales.NewRequest(), locales.NewRequestDesc())
		card(c, locales.ImportCollection(), locales.ImportCollectionDesc())
	})
}

func card(c *ui.Context, title, desc string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Height(m.CardHeight).Gap(m.Space1).
		Padding(m.Space4).Radius(m.RadiusMD).
		Background(tk.BgSurface).Border(m.Border, tk.Border).
		Children(func() {
			ui.Text(c, title).FontSize(m.FontMD).Bold().TextColor(tk.TextPrimary)
			ui.Text(c, desc).FontSize(m.FontSM).TextColor(tk.TextSecondary)
		})
}

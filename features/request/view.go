// Package request 实现请求编辑与响应查看。
package request

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/internal/nav"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制请求页：URL 栏、请求配置区与响应区。
func View(c *ui.Context, n nav.Navigator) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c)
	})
}

func content(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	v1.PageHeader(c, locales.RequestPageTitle(), locales.RequestPageSubtitle())

	ui.Row(c).Padding(0, m.PagePaddingX).Gap(m.Space2).
		AlignItems(ui.Center).Children(func() {
		v1.Tag(c, "GET")
		ui.Row(c).Grow(1).Height(m.SizeLG).PaddingX(m.Space2).
			AlignItems(ui.Center).Radius(m.RadiusMD).
			Background(tk.BgSurface).Border(m.Border, tk.Border).
			Children(func() {
				ui.Text(c, "https://api.example.com/users/me").FontSize(m.FontSM).
					TextColor(tk.TextSecondary).SingleLine()
			})
		ui.PrimaryButton(c, locales.Send())
	})

	ui.Row(c).Gap(m.Space4).Padding(m.Space3, m.PagePaddingX).Children(func() {
		for i, name := range []string{"Params", "Authorization", "Headers", "Body", "Scripts", "Tests"} {
			txt := ui.Text(c, name).FontSize(m.FontSM).TextColor(tk.TextMuted)
			if i == 0 {
				txt.TextColor(tk.TextPrimary).Bold()
			}
		}
	})

	ui.Column(c).Height(200).Padding(m.Space3).Radius(m.RadiusMD).
		Margin(0, m.PagePaddingX).
		Background(tk.BgSurface).Border(m.Border, tk.Border).
		Children(func() {
			ui.Text(c, locales.RequestConfigArea()).FontSize(m.FontSM).TextColor(tk.TextMuted)
		})

	ui.Row(c).Padding(m.Space3, m.PagePaddingX).Gap(m.Space2).
		AlignItems(ui.Center).Children(func() {
		ui.Text(c, locales.Response()).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
		ui.Text(c, "200 OK").FontSize(m.FontXS).TextColor(tk.Success)
	})
	ui.Column(c).Height(240).Padding(m.Space3).Radius(m.RadiusMD).
		Margin(0, m.PagePaddingX, m.PagePaddingX, m.PagePaddingX).
		Background(tk.BgSurface).Border(m.Border, tk.Border).
		Children(func() {
			ui.Text(c, locales.ResponseBodyArea()).FontSize(m.FontSM).TextColor(tk.TextMuted)
		})
}

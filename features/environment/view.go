// Package environment 实现环境变量管理。
package environment

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/internal/nav"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制环境变量页：变量表。
func View(c *ui.Context, n nav.Navigator) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c)
	})
}

func content(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	v1.PageHeader(c, locales.EnvironmentTitle(), locales.EnvironmentSubtitle())

	ui.Row(c).Padding(0, m.PagePaddingX, m.Space3, m.PagePaddingX).Children(func() {
		ui.PrimaryButton(c, locales.NewEnvironment())
	})

	ui.Column(c).Margin(0, m.PagePaddingX).Background(tk.BgSurface).
		Border(m.Border, tk.Border).Radius(m.RadiusMD).Children(func() {
		ui.Row(c).Gap(m.Space3).Padding(m.Space2, m.Space3).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.VariableName()).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(180)
			ui.Text(c, locales.InitialValue()).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(180)
			ui.Text(c, locales.CurrentValue()).FontSize(m.FontXS).TextColor(tk.TextMuted).Grow(1)
		})
		for _, it := range []struct{ name, value string }{
			{"baseUrl", "https://api.example.com"},
			{"accessToken", "••••••"},
			{"tenantId", "team-01"},
		} {
			ui.Row(c).Gap(m.Space3).Padding(m.Space2, m.Space3).
				BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
				ui.Text(c, it.name).FontSize(m.FontSM).TextColor(tk.TextPrimary).Width(180)
				ui.Text(c, it.value).FontSize(m.FontSM).TextColor(tk.TextSecondary).
					Width(180).SingleLine()
				ui.Text(c, it.value).FontSize(m.FontSM).TextColor(tk.TextSecondary).
					Grow(1).SingleLine()
			})
		}
	})
}

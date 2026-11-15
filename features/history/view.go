// Package history 实现请求历史。
package history

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/internal/nav"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制历史记录页：列表。
func View(c *ui.Context, n nav.Navigator) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c)
	})
}

func content(c *ui.Context) {
	m := theme.Metrics
	v1.PageHeader(c, locales.HistoryTitle(), locales.HistorySubtitle())

	ui.Column(c).Gap(1).Padding(0, m.PagePaddingX).Children(func() {
		for _, it := range []struct{ method, path, status, at string }{
			{"GET", "/v1/users/me", "200", "2 分钟前"},
			{"POST", "/v1/orders", "201", "18 分钟前"},
			{"DELETE", "/v1/orders/1024", "204", "昨天 18:42"},
		} {
			row(c, it.method, it.path, it.status, it.at)
		}
	})
}

func row(c *ui.Context, method, path, status, at string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Gap(m.Space3).Padding(m.Space2, 0).
		AlignItems(ui.Center).Children(func() {
		v1.Tag(c, method)
		ui.Text(c, path).FontSize(m.FontSM).TextColor(tk.TextPrimary).Grow(1).SingleLine()
		ui.Text(c, status).FontSize(m.FontXS).TextColor(tk.Success)
		ui.Text(c, at).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(96)
	})
}

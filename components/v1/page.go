package v1

import (
	"rddpostman/components/theme"

	"github.com/egoist/mygo/ui"
)

// PageHeader 是主区域顶部的页面标题与说明。
func PageHeader(c *ui.Context, title, subtitle string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Gap(m.Space1).Padding(m.PagePaddingY, m.PagePaddingX).Children(func() {
		ui.Text(c, title).FontSize(m.FontXL).Bold().TextColor(tk.TextPrimary)
		if subtitle != "" {
			ui.Text(c, subtitle).FontSize(m.FontSM).TextColor(tk.TextSecondary)
		}
	})
}

// EmptyState 是页面内容尚未实现时的占位。
func EmptyState(c *ui.Context, text string) ui.Element {
	m := theme.Metrics
	tk := theme.Of(c)
	return ui.Box(c).Padding(m.Space6).Children(func() {
		ui.Text(c, text).FontSize(m.FontSM).TextColor(tk.TextMuted)
	})
}

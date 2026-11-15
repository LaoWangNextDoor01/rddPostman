package collection

import (
	"strconv"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// collectionTree 用 TreeBase 画集合树，点击行选中并打开对应 tab。
func collectionTree(c *ui.Context, w *workspace, onSelect func(string)) {
	m := theme.Metrics
	tk := theme.Of(c)
	tree := ui.TreeBase(c)
	tree.Root.Grow(1).Gap(1).Children(func() {
		var build func(n *node)
		build = func(n *node) {
			c.Key(n.ID)
			var open *bool
			if n.expandable() {
				open = &n.open
			}
			item := tree.Item(open)
			row := item.Row.Height(m.SizeLG).Gap(m.Space1).
				Padding(0, m.Space1).
				PaddingStart(m.Space1 + 14*float32(item.Depth())).
				Radius(m.RadiusMD)
			switch {
			case w.selected == n.ID:
				row.Background(tk.AccentSoft)
			case row.Hovered():
				row.Background(tk.HoverSurface)
			}
			if row.Clicked() {
				onSelect(n.ID)
			}
			row.Children(func() {
				if n.expandable() {
					arrow := ui.Icon(c, icons.Get("chevron-down")).Size(12, 12).
						TextColor(tk.TextMuted)
					if !n.open {
						arrow.Rotate(-90)
					}
					if arrow.Clicked() {
						n.open = !n.open
					}
				} else {
					ui.Box(c).Width(12)
				}

				if n.Kind == kindRequest {
					ui.Text(c, n.Method).FontSize(m.FontXS).Width(40).TextColor(methodColor(tk, n.Method)).Bold()
				} else if n.Kind == kindCollection {
					ui.Icon(c, icons.Get("library")).FontSize(m.IconSM).TextColor(tk.TextSecondary)
				} else {
					ui.Icon(c, icons.Get("folder")).FontSize(m.IconSM).TextColor(tk.TextSecondary)
				}

				text := ui.Text(c, n.Name).FontSize(m.FontSM).Grow(1).SingleLine()
				if w.selected == n.ID {
					text.TextColor(tk.Accent).Bold()
				} else {
					text.TextColor(tk.TextPrimary)
				}

				if n.expandable() && len(n.Children) > 0 {
					ui.Text(c, strconv.Itoa(requestCount(n))).FontSize(10).
						TextColor(tk.TextMuted)
				}
			})
			if n.expandable() {
				item.Group(func() {
					for _, ch := range n.Children {
						build(ch)
					}
				})
			}
		}
		for _, r := range w.roots {
			build(r)
		}
	})
}

func nodeIcon(k nodeKind) *ui.SVG {
	if k == kindRequest {
		return icons.Get("file")
	}
	return icons.Get("folder")
}

// requestCount 统计子树里的请求数量，用于目录右侧的计数。
func requestCount(n *node) int {
	if n.Kind == kindRequest {
		return 1
	}
	total := 0
	for _, ch := range n.Children {
		total += requestCount(ch)
	}
	return total
}

func methodColor(tk theme.Tokens, method string) ui.Color {
	switch method {
	case "GET":
		return tk.Success
	case "POST":
		return tk.Accent
	case "PUT":
		return tk.Info
	case "DELETE":
		return tk.Danger
	case "PATCH":
		// 设计稿里 PATCH 用固定紫 #7C5CE0，没有对应的主题变量。
		return ui.Hex("#7C5CE0")
	default:
		return tk.TextMuted
	}
}

package collection

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func sidebar(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	col := ui.Column(c).Grow(1).Background(tk.BgSurface)
	search := ui.Local(col, "search", func() string { return "" })
	col.Children(func() {
		// 顶部工具行：搜索、新建、环境、更多。
		ui.Row(c).Padding(m.Space1, m.Space2).Gap(6).
			AlignItems(ui.Center).Children(func() {
			ui.Row(c).Fill().Height(28).PaddingX(m.Space2).Gap(m.Space1).
				AlignItems(ui.Center).Radius(m.RadiusMD).
				Background(tk.BgElevated).Children(func() {
				ui.Icon(c, icons.Get("search")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
				ui.TextInputBase(c, search).Grow(1).FontSize(m.FontXS).
					Placeholder(locales.SearchRequests())
			})

			addMenuButton(c, props)
			environmentMenuButton(c, props)
			v1.MiniButton(c, v1.RdButtonOptions{
				Key:      "more",
				Label:    "more",
				Icon:     icons.Get("ellipsis"),
				Size:     m.SizeLG,
				IconSize: m.IconMD,
				Radius:   m.RadiusMD,
			})
		})

		ui.Divider(c)

		// 空提示
		if len(props.collections) == 0 {
			emptySidebar(c, props)
			return
		}

		// 请求树形结构
		actions := treeActions{
			create: func(row *treeNode, kind treeKind) {
				domainKind := domain.KindFolder
				if kind == treeRequest {
					domainKind = domain.KindRequest
				}
				props.actions.createNode(domain.ID(row.id), domainKind)
			},
			remove: func(row *treeNode) {
				props.actions.deleteRow(row)
			},
			open: func(row *treeNode) {
				props.actions.openTab(row.id)
			},
			copy: func(text string) {
				c.Services().WriteClipboard(text)
			},
		}
		ui.Scroll(c).Grow(1).Padding(m.Space1).Children(func() {
			collectionTree(c, props.state, buildTree(props.collections), actions)
		})
	})
}

// emptySidebar 是设计稿里的空侧栏：图标、标题、说明与新建按钮，垂直居中。
func emptySidebar(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(174).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Box(c).Size(56, 56).Radius(m.RadiusLG).
				Background(tk.BgElevated).Children(func() {
				ui.Box(c).Width(14).Height(5).Absolute().Left(21).Top(5).
					Background(tk.TextMuted)
				ui.Box(c).Width(28).Height(20).Absolute().Left(14).Top(10).
					Background(tk.TextMuted)
				ui.Box(c).Width(28).Height(15).Absolute().Left(14).Top(15).
					Background(tk.TextSecondary)
			})
			ui.Text(c, locales.EmptyCollectionsTitle()).FontSize(m.FontMD).
				Bold().TextColor(tk.TextPrimary)
			ui.Text(c, locales.EmptyCollectionsDesc()).Width(174).FontSize(m.FontXS).
				TextColor(tk.TextSecondary).TextAlign(ui.Center)

			btn := ui.ButtonBase(c.Key("empty-new-collection")).Height(34).
				PaddingX(m.Space3).Gap(m.Space1).AlignItems(ui.Center).
				Radius(m.RadiusMD).Background(tk.Accent).TextColor(tk.OnAccent).
				Label(locales.NewCollection())
			if btn.Hovered() {
				btn.Background(tk.Accent.Mix(tk.TextPrimary, 0.06))
			}
			if btn.Clicked() {
				props.actions.createCollection()
			}
			btn.Children(func() {
				ui.Icon(c, icons.Get("plus")).FontSize(m.IconXS).
					TextColor(tk.OnAccent)
				ui.Text(c, locales.NewCollection()).FontSize(m.FontSM).
					TextColor(tk.OnAccent)
			})
		})
	})
}

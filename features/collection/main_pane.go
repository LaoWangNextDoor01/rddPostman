package collection

import (
	"runtime"

	"rddpostman/components/theme"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func mainPane(c *ui.Context, props pageProps) {
	tk := theme.Of(c)
	state := props.state

	ui.Column(c).Grow(1).Background(tk.BgSurface).Children(func() {
		// 没有任何打开的 tab 时，整条 tab 区域一起隐藏，只留空白工作区。
		if len(state.tabs) > 0 {
			tabsBar(c, props)
		}
		coll, node := findNode(props.collections, state.active)
		switch {
		case node != nil && node.Kind == domain.KindRequest:
			requestTab(c, props, coll, node, state.active)
		case coll != nil:
			overviewTab(c, props, coll, node)
		default:
			emptyWorkspace(c, props)
		}
	})
}

// emptyWorkspace 是「没有打开的请求」时的右侧空白态，按设计稿 75 实现。
func emptyWorkspace(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Background(tk.BgSurface).Justify(ui.Center).
		AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(420).AlignItems(ui.Center).Children(func() {
			ui.Box(c).Size(148, 148).Radius(34).
				Background(tk.Accent.Alpha(0.13)).Center().Children(func() {
				ui.Text(c, "R").FontSize(86).FontWeight(600).
					TextColor(tk.OnAccent.Alpha(0.85))
			})
			ui.Box(c).Height(20)
			ui.Text(c, locales.EmptyWorkspaceTitle()).FontSize(18).
				FontWeight(600).TextColor(tk.TextPrimary)
			ui.Box(c).Height(8)
			desc := locales.EmptyWorkspaceDesc()
			if len(props.collections) == 0 {
				desc = locales.EmptyCollectionsDesc()
			}
			ui.Text(c, desc).FontSize(m.FontSM).TextColor(tk.TextMuted)
			ui.Box(c).Height(28)
			shortcutHints(c, tk, m)
			ui.Box(c).Height(24)
			ui.Box(c).Width(360).Height(1).Background(tk.Border)
			ui.Box(c).Height(16)
			emptyQuickLinks(c, props, tk, m)
			ui.Box(c).Height(18)
			ui.Text(c, locales.EmptyWorkspaceTip(keycap("", "K"))).
				FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
	})
}

// shortcutHints 是空白态里的快捷键提示列表，按设计稿的 6 行、26 高、8 间距绘制。
func shortcutHints(c *ui.Context, tk theme.Tokens, m theme.Metric) {
	rows := []struct{ label, key string }{
		{locales.NewRequest(), keycap("", "N")},
		{locales.NewCollection(), keycap("", "B")},
		{locales.GlobalSearch(), keycap("", "K")},
		{locales.ImportCollection(), keycap("", "I")},
		{locales.SwitchEnvironment(), keycap("", "E")},
		{locales.ShowAllCommands(), keycap("shift", "P")},
	}
	ui.Column(c).Width(360).Gap(m.Space2).Children(func() {
		for _, row := range rows {
			ui.Row(c).FillWidth().Height(26).AlignItems(ui.Center).Children(func() {
				ui.Text(c, row.label).FontSize(m.FontSM).
					TextColor(tk.TextSecondary).Grow(1)
				ui.Box(c).Padding(2, 6).Radius(m.RadiusSM).
					Background(tk.BgElevated).Children(func() {
					ui.Text(c, row.key).FontSize(m.FontXS).
						TextColor(tk.TextMuted)
				})
			})
		}
	})
}

// emptyQuickLinks 是空白态底部的快捷入口。
func emptyQuickLinks(c *ui.Context, props pageProps, tk theme.Tokens, m theme.Metric) {
	link := func(label string, fn func()) {
		text := ui.Text(c, label).FontSize(m.FontXS).TextColor(tk.Accent)
		if text.Clicked() {
			fn()
		}
	}
	sep := func() {
		ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
	}
	ui.Row(c).Width(360).Justify(ui.Center).AlignItems(ui.Center).
		Gap(m.Space1).Children(func() {
		link(locales.NewRequest(), func() {
			if len(props.collections) > 0 {
				props.actions.createNode(domain.ID(props.collections[0].ID), domain.KindRequest)
				return
			}
			props.state.newCollOpen = true
			props.state.newCollName = ""
			props.state.newCollDesc = ""
			props.state.newCollErr = ""
		})
		sep()
		link(locales.ImportCollection(), func() {
			props.actions.navigate(routes.Collections)
		})
		sep()
		link(locales.HistoryTitle(), func() {
			props.actions.navigate(routes.History)
		})
	})
}

// keycap 按平台返回快捷键的显示文本。
func keycap(mods, key string) string {
	if runtime.GOOS == "darwin" {
		if mods == "shift" {
			return "⇧⌘" + key
		}
		return "⌘" + key
	}
	if mods == "shift" {
		return "Ctrl+Shift+" + key
	}
	return "Ctrl+" + key
}

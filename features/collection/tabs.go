package collection

import (
	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// tabBar 是主区顶部的 Segment 风格 tab 条：白底 36 高、内边距 4、
// 间距 3、底部 1px 分割线。
func tabBar(c *ui.Context) ui.Element {
	m := theme.Metrics
	tk := theme.Of(c)
	return ui.Row(c).Height(36).Padding(m.Space1).Gap(3).
		AlignItems(ui.Center).Background(tk.BgSurface).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
}

// tabBarTrailing 画 tab 条右侧：新建 tab、终端与更多。
func tabBarTrailing(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Box(c).Width(m.Space3)
	ui.Box(c).Size(m.IconMD, m.IconMD).Radius(m.RadiusSM).Center().
		Background(tk.BgElevated).Children(func() {
		ui.Icon(c, icons.Get("plus")).FontSize(m.IconMD).
			TextColor(tk.TextMuted)
	})
	ui.Spacer(c)
	// 设计稿右侧固定灰 #71717A，没有对应的主题变量。
	ui.Row(c).Gap(m.Space2).Margin(0, m.Space3, 0, 0).
		AlignItems(ui.Center).Children(func() {
		for _, ic := range []string{"square-terminal", "ellipsis"} {
			ui.Icon(c, icons.Get(ic)).FontSize(m.IconMD).
				TextColor(ui.Hex("#71717A"))
		}
	})
}

// tabsBar 是主区域顶部的请求 tab，Segment 风格。
func tabsBar(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	state := props.state
	tabBar(c).
		Children(func() {
			for _, id := range state.tabs {
				coll, tabNode := findNode(props.collections, id)
				if coll == nil {
					continue
				}
				tab := ui.ButtonBase(c.Key("tab-"+id)).Height(m.SizeLG).
					Padding(0, m.Space2, 0, m.Space3).Gap(m.Space2).
					Radius(m.RadiusMD).AlignItems(ui.Center).
					Label(tabLabel(coll, tabNode)).Role(ui.RoleTab)
				selected := state.active == id
				switch {
				case selected:
					tab.Background(tk.BgElevated).TextColor(tk.TextPrimary)
				case tab.Hovered():
					tab.Background(tk.HoverSurface).TextColor(tk.TextPrimary)
				default:
					tab.TextColor(tk.TextSecondary)
				}
				if tab.Clicked() {
					props.actions.openTab(id)
				}
				tab.Children(func() {
					switch {
					case tabNode != nil && tabNode.Request != nil:
						ui.Text(c, tabNode.Request.Method).FontSize(m.FontXS).
							LetterSpacing(0.3).Bold().
							TextColor(methodColor(tk, tabNode.Request.Method))
						name := ui.Text(c, tabNode.Name).FontSize(m.FontSM).
							SingleLine()
						if selected {
							name.FontWeight(600)
						}
					default:
						// 集合与文件夹概览 tab 共用设计稿里的同一个 13x11 图标。
						ui.Icon(c, icons.Get("folder")).Size(13, 11).
							TextColor(tk.TextSecondary)
						ui.Text(c, tabLabel(coll, tabNode)).FontSize(m.FontMD).
							TextColor(tabNameColor(tk, selected)).SingleLine()
					}
					close := ui.ButtonBase(c.Key("close-" + id)).Width(m.SizeXS - 2).
						Height(m.SizeXS - 2).Justify(ui.Center).AlignItems(ui.Center).
						Label(locales.Clear()).Role(ui.RoleButton).TextColor(tk.TextDisabled)
					if close.Hovered() {
						close.Background(tk.Border).Radius(m.RadiusSM).TextColor(tk.TextPrimary)
					}
					if close.Clicked() {
						state.closeTab(id)
					}
					close.Children(func() {
						ui.Icon(c, icons.Get("x")).FontSize(m.FontSM)
					})
				})
			}
			tabBarTrailing(c)
		})
}

// tabLabel 给 tab 提供无障碍名称：集合用集合名，文件夹用节点名，请求用请求名。
func tabLabel(coll *domain.Collection, node *domain.Node) string {
	if node == nil {
		return coll.Name
	}
	return node.Name
}

// tabNameColor 是概览 tab 的名称颜色：选中时用主文字色。
func tabNameColor(tk theme.Tokens, selected bool) ui.Color {
	if selected {
		return tk.TextPrimary
	}
	return tk.TextSecondary
}

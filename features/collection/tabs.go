package collection

import (
	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// tabBar 是主区顶部的标签条（对齐 Pixso inst / row / tabs：34 高、BgApp 底色、底部 1px 分割线）。
func tabBar(c *ui.Context) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	return ui.Row(c).FillWidth().Height(34).Padding(0, 12, 0, 10).Gap(6).
		AlignItems(ui.Center).Background(tk.BgApp).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
}

func findEnvironment(envs []domain.Environment, id domain.ID) *domain.Environment {
	for i := range envs {
		if envs[i].ID == id {
			return &envs[i]
		}
	}
	return nil
}

// tabsBar 绘制主区域顶部栏：胶囊轨道内多类型 Tabs + 新建「+」（支持横向滚动，去除右侧环境切换器与多余外层底色）。
func tabsBar(c *ui.Context, props pageProps) {
	m, tk := thV2.Metrics, thV2.Of(c)
	tkV1 := theme.Of(c)
	state := props.state

	tabBar(c).Children(func() {
		// 1. 横向滚动的 Tab 行：每个 Tab 独立呈现，直接容器无多余背景色，支持超长横向滚动
		ui.ScrollHorizontal(c).Grow(1).FillHeight().AlignItems(ui.Center).Gap(4).Children(func() {
			for _, id := range state.tabs {
				tabID := id
				selected := state.active == tabID

				// 环境变量 Tab
				if envID, isEnv := parseEnvTabID(tabID); isEnv {
					env := findEnvironment(props.environments, envID)
					if env == nil {
						continue
					}
					tab := ui.ButtonBase(c.Key("tab-"+tabID)).Height(26).MinWidth(60).
						Padding(0, 8, 0, 10).Gap(6).
						Radius(4).AlignItems(ui.Center).
						Cursor(ui.CursorPointer).Label(env.Name).Role(ui.RoleTab).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case selected:
						tab.Background(tk.BgSurface).
							Border(1, tk.Border).
							Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
					case tab.Pressed():
						tab.Background(tk.BgSelected)
					case tab.Hovered():
						tab.Background(tk.BgHover)
					default:
						tab.Background(ui.Transparent)
					}
					if tab.Clicked() {
						props.actions.openTab(tabID)
					}
					tab.Children(func() {
						iconColor := tk.TextSecondary
						if selected {
							iconColor = tk.Accent
						}
						ui.Icon(c, icons.Globe).Size(12, 12).TextColor(iconColor)
						ui.Text(c, env.Name).FontSize(m.FontXS).FontWeight(500).
							TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[selected]).SingleLine()
						tabCloseButton(c, tabID, state)
					})
					continue
				}

				coll, tabNode := findNodeInWorkspace(props.collections, state, tabID)
				if coll == nil {
					continue
				}
				tab := ui.ButtonBase(c.Key("tab-"+tabID)).Height(26).MinWidth(60).
					Padding(0, 8, 0, 10).Gap(6).
					Radius(4).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Label(tabLabel(coll, tabNode)).Role(ui.RoleTab).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case selected:
					tab.Background(tk.BgSurface).
						Border(1, tk.Border).
						Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
				case tab.Pressed():
					tab.Background(tk.BgSelected)
				case tab.Hovered():
					tab.Background(tk.BgHover)
				default:
					tab.Background(ui.Transparent)
				}
				if tab.Clicked() {
					props.actions.openTab(tabID)
				}
				tab.Children(func() {
					switch {
					case tabNode != nil && tabNode.Request != nil:
						ui.Text(c, tabNode.Request.Method).FontSize(10).
							LetterSpacing(0.3).Bold().
							TextColor(methodColor(tkV1, tabNode.Request.Method))
						ui.Text(c, tabNode.Name).FontSize(m.FontXS).FontWeight(500).
							TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[selected]).SingleLine()
					case tabNode == nil:
						ui.Icon(c, icons.Folder).Size(12, 12).TextColor(tk.TextSecondary)
						ui.Text(c, coll.Name).FontSize(m.FontXS).FontWeight(500).
							TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[selected]).SingleLine()
					default:
						ui.Icon(c, icons.Folder).Size(12, 12).TextColor(tk.TextSecondary)
						ui.Text(c, tabNode.Name).FontSize(m.FontXS).FontWeight(500).
							TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[selected]).SingleLine()
					}

					// 未保存小圆点指示器（对齐 Pixso span 节点：直径 6px 实心微圆点）
					if state.isUnsaved(tabID) {
						ui.Box(c.Key("tab-unsaved-" + tabID)).
							Size(6, 6).Radius(3).
							Background(tk.Accent).
							Margin(0, 2, 0, 0)
					}

					tabCloseButton(c, tabID, state)
				})
			}

			// 2. 新建「+」按钮（紧随 Tab 之后，同样处于滚动行内）
			addBtn := ui.ButtonBase(c.Key("tab-add")).
				Size(24, 24).Radius(m.RadiusSM).Center().
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("tab add").
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case addBtn.Pressed():
				addBtn.Background(tk.BgSelected)
			case addBtn.Hovered():
				addBtn.Background(tk.BgHover)
			default:
				addBtn.Background(ui.Transparent)
			}
			if addBtn.Clicked() {
				if len(props.collections) > 0 {
					props.actions.createNode(props.collections[0].ID, domain.KindRequest)
				} else if props.actions.createCollectionTab != nil {
					props.actions.createCollectionTab()
				}
			}
			addBtn.Children(func() {
				ui.Icon(c, icons.Plus).FontSize(13).TextColor(tk.TextSecondary)
			})
		})
	})
}

func tabCloseButton(c *ui.Context, tabID string, state *workspace) {
	m, tk := thV2.Metrics, thV2.Of(c)
	closeBtn := ui.ButtonBase(c.Key("close-" + tabID)).
		Size(16, 16).Center().Radius(m.RadiusSM).
		Cursor(ui.CursorPointer).Label(locales.Clear()).Role(ui.RoleButton).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case closeBtn.Pressed():
		closeBtn.Background(tk.BgSelected)
	case closeBtn.Hovered():
		closeBtn.Background(tk.BgHover)
	}
	if closeBtn.Clicked() {
		state.closeTab(tabID)
	}
	closeBtn.Children(func() {
		ui.Icon(c, icons.X).FontSize(10).TextColor(tk.TextMuted)
	})
}

// tabLabel 给 tab 提供无障碍名称：集合用集合名，文件夹用节点名，请求用请求名。
func tabLabel(coll *domain.Collection, node *domain.Node) string {
	if node == nil {
		return coll.Name
	}
	return node.Name
}

// tabNameColor 让当前选中的 tab 文字变深，其余偏灰。
func tabNameColor(tk theme.Tokens, selected bool) ui.Color {
	if selected {
		return tk.TextPrimary
	}
	return tk.TextSecondary
}

package collection

import (
	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// tabBar 是主区顶部的标签条：纯白 BgBase 与主内容区融为一体、去底部冗余通栏线，视觉轻盈通透。
func tabBar(c *ui.Context) ui.Element {
	tk := thV2.Of(c)
	return ui.Row(c).FillWidth().Height(36).Padding(0, 16, 0, 12).Gap(6).
		AlignItems(ui.Center).Background(tk.BgBase)
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
						Radius(m.RadiusMD).AlignItems(ui.Center).
						Cursor(ui.CursorPointer).Label(env.Name).Role(ui.RoleTab).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case selected:
						tab.Background(tk.BgSelected)
					case tab.Pressed():
						tab.Background(tk.BgHover)
					case tab.Hovered():
						tab.Background(tk.BgSubtle)
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

				// 开发者小工具 Tab
				if toolID, isTool := parseToolTabID(tabID); isTool {
					label := toolID
					if props.actions.toolTabLabel != nil {
						label = props.actions.toolTabLabel(toolID)
					}
					tab := ui.ButtonBase(c.Key("tab-"+tabID)).Height(26).MinWidth(60).
						Padding(0, 8, 0, 10).Gap(6).
						Radius(m.RadiusMD).AlignItems(ui.Center).
						Cursor(ui.CursorPointer).Label(label).Role(ui.RoleTab).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case selected:
						tab.Background(tk.BgSelected)
					case tab.Pressed():
						tab.Background(tk.BgHover)
					case tab.Hovered():
						tab.Background(tk.BgSubtle)
					default:
						tab.Background(ui.Transparent)
					}
					if tab.Clicked() {
						props.actions.openTab(tabID)
					}
					tab.Children(func() {
						badgeColor := tk.TextSecondary
						if selected {
							badgeColor = tk.Accent
						}
						ui.Text(c, "TOOL").FontSize(10).
							LetterSpacing(0.3).Bold().
							TextColor(badgeColor)
						ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).
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
					Radius(m.RadiusMD).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Label(tabLabel(coll, tabNode)).Role(ui.RoleTab).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case selected:
					tab.Background(tk.BgSelected)
				case tab.Pressed():
					tab.Background(tk.BgHover)
				case tab.Hovered():
					tab.Background(tk.BgSubtle)
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

			// 2. 新建「+」按钮（仅当有打开的 Tabs 时紧随其后显示；没有 Tabs 时不显示）
			if len(state.tabs) > 0 {
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
			}
		})

		// 2. 最右侧环境切换下拉按钮（不带多余图标按钮，支持一键查看与切换当前生效环境）
		tabBarEnvironmentSwitcher(c, props)
	})
}

// tabBarEnvironmentSwitcher 绘制 Tab 栏最右侧的环境变量下拉切换按钮。
func tabBarEnvironmentSwitcher(c *ui.Context, props pageProps) {
	m, tk := thV2.Metrics, thV2.Of(c)

	activeEnvName := "No Environment"
	hasActiveEnv := false
	for _, env := range props.environments {
		if env.ID == props.selectedEnv {
			activeEnvName = env.Name
			hasActiveEnv = true
			break
		}
	}

	btn := ui.ButtonBase(c.Key("tab-env-switcher")).
		Height(26).Padding(0, 8).Gap(5).
		Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).
		Label(activeEnvName).Tooltip("Switch environment").
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	open := ui.Local(btn, "tab-env-open", func() bool { return false })
	if btn.Clicked() {
		*open = !*open
	}

	switch {
	case *open, btn.Pressed():
		btn.Background(tk.BgSelected)
	case btn.Hovered():
		btn.Background(tk.BgHover)
	default:
		btn.Background(ui.Transparent)
	}

	btn.Children(func() {
		iconColor := tk.TextMuted
		if hasActiveEnv {
			iconColor = tk.Accent
		}
		ui.Icon(c, icons.Globe).FontSize(12).TextColor(iconColor)
		ui.Text(c, activeEnvName).FontSize(11).
			FontWeight(map[bool]int{true: 500, false: 400}[hasActiveEnv]).
			TextColor(tk.TextPrimary).SingleLine()
		ui.Icon(c, icons.ChevronDown).FontSize(10).TextColor(tk.TextMuted)
	})

	v2.Popover(c, btn, open, v2.PopoverOpts{Width: 220}, func(panel ui.Element) {
		ui.Column(c).Padding(4, 0).Gap(1).Children(func() {
			ui.Text(c, "ENVIRONMENT").FontSize(10).FontWeight(600).
				TextColor(tk.TextMuted).Padding(4, 10, 2, 10)

			// 1. "No Environment" 选项
			noEnvBtn := ui.ButtonBase(c.Key("tab-env-opt-none")).
				FillWidth().Height(28).Padding(0, 10).Gap(8).
				Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("No Environment")
			if !hasActiveEnv {
				noEnvBtn.Background(tk.BgSelected)
			} else if noEnvBtn.Hovered() {
				noEnvBtn.Background(tk.BgHover)
			}
			if noEnvBtn.Clicked() {
				if props.actions.selectEnvironment != nil {
					props.actions.selectEnvironment("")
				}
				*open = false
			}
			noEnvBtn.Children(func() {
				ui.Text(c, "No Environment").FontSize(m.FontXS).
					FontWeight(map[bool]int{true: 500, false: 400}[!hasActiveEnv]).
					TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[!hasActiveEnv]).
					SingleLine().Grow(1)
				if !hasActiveEnv {
					ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.Accent)
				}
			})

			if len(props.environments) > 0 {
				v2.PopoverDivider(c)
			}

			// 2. 环境列表
			for _, e := range props.environments {
				env := e
				isSel := env.ID == props.selectedEnv
				row := ui.ButtonBase(c.Key("tab-env-opt-" + string(env.ID))).
					FillWidth().Height(28).Padding(0, 10).Gap(8).
					Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.SpaceBetween).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(env.Name)
				if isSel {
					row.Background(tk.BgSelected)
				} else if row.Hovered() {
					row.Background(tk.BgHover)
				}
				if row.Clicked() {
					if props.actions.selectEnvironment != nil {
						props.actions.selectEnvironment(env.ID)
					}
					*open = false
				}
				row.Children(func() {
					ui.Text(c, env.Name).FontSize(m.FontXS).
						FontWeight(map[bool]int{true: 500, false: 400}[isSel]).
						TextColor(map[bool]ui.Color{true: tk.TextPrimary, false: tk.TextSecondary}[isSel]).
						SingleLine().Grow(1)
					if isSel {
						ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.Accent)
					}
				})
			}
		})
	})
}

func tabCloseButton(c *ui.Context, tabID string, state *workspace) {
	m, tk := thV2.Metrics, thV2.Of(c)
	closeBtn := ui.ButtonBase(c.Key("close-" + tabID)).
		Size(18, 18).Center().Radius(m.RadiusSM).
		Cursor(ui.CursorPointer).Label(locales.Clear()).Role(ui.RoleButton).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	iconColor := tk.TextMuted
	switch {
	case closeBtn.Pressed():
		closeBtn.Background(tk.TextPrimary.Alpha(0.14))
		iconColor = tk.TextPrimary
	case closeBtn.Hovered():
		closeBtn.Background(tk.BgSelected)
		iconColor = tk.TextPrimary
	}
	if closeBtn.Clicked() {
		state.closeTab(tabID)
	}
	closeBtn.Children(func() {
		ui.Icon(c, icons.X).FontSize(11).TextColor(iconColor)
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

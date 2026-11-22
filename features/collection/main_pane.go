package collection

import (
	"runtime"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func mainPane(c *ui.Context, props pageProps) {
	tk := thV2.Of(c)
	state := props.state

	ui.Column(c).Grow(1).MinHeight(0).Background(tk.BgSurface).Children(func() {
		// 顶部常驻 Tab 条（含多类型 Tabs、+ 与右侧环境切换器，对齐 Pixso 8-请求-Body）
		tabsBar(c, props)
		if envID, isEnv := parseEnvTabID(state.active); isEnv && props.actions.renderEnvTab != nil {
			props.actions.renderEnvTab(envID)
			return
		}
		coll, node := findNodeInWorkspace(props.collections, props.state, state.active)
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

// emptyWorkspace 是「没有打开任何 Tab」时的右侧现代化工作区欢迎页。
func emptyWorkspace(c *ui.Context, props pageProps) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	wsName := props.workspaceName
	if wsName == "" {
		wsName = "API Workspace"
	}

	ui.Scroll(c).Grow(1).Background(tk.BgSurface).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(640).Padding(48, 20, 40, 20).
				AlignItems(ui.Center).Gap(20).Children(func() {

				// 1. 顶部工作区徽标与标题区
				ui.Box(c).Size(52, 52).Radius(14).
					Background(tk.Accent.Alpha(0.12)).
					Border(1, tk.Accent.Alpha(0.24)).Center().Children(func() {
					ui.Icon(c, icons.Layers).FontSize(24).TextColor(tk.Accent)
				})

				ui.Column(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Text(c, wsName).FontSize(20).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, "在工作台中组织集合、调试 API 请求与同步环境变量。从左侧打开已有接口，或立即开始：").
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).Height(4)

				// 2. 核心快捷操作卡片组（2 × 2 网格，完全内联创建，无弹窗）
				ui.Column(c).FillWidth().Gap(12).Children(func() {
					// 第一行：新建请求 + 新建集合
					ui.Row(c).FillWidth().Gap(12).Children(func() {
						emptyActionCard(c, emptyCardProps{
							Key:      "empty-act-new-req",
							Icon:     icons.Send,
							Title:    locales.NewRequest(),
							Desc:     "快速创建并调试单个 HTTP 请求",
							OnClick: func() {
								if len(props.collections) > 0 {
									props.actions.createNode(domain.ID(props.collections[0].ID), domain.KindRequest)
								} else if props.actions.createCollectionTab != nil {
									props.actions.createCollectionTab()
								}
							},
						})

						emptyActionCard(c, emptyCardProps{
							Key:      "empty-act-new-coll",
							Icon:     icons.Library,
							Title:    locales.NewCollection(),
							Desc:     "在 Tab 内直接编辑集合名称与统一认证",
							OnClick: func() {
								if props.actions.createCollectionTab != nil {
									props.actions.createCollectionTab()
								}
							},
						})
					})

					// 第二行：导入接口 + 环境管理
					ui.Row(c).FillWidth().Gap(12).Children(func() {
						emptyActionCard(c, emptyCardProps{
							Key:      "empty-act-import",
							Icon:     icons.Code,
							Title:    "导入接口 (cURL)",
							Desc:     "从 cURL 命令一键生成并解析请求",
							OnClick: func() {
								props.state.importCurlOpen = true
								props.state.importCurlText = ""
								props.state.importCurlErr = ""
							},
						})

						emptyActionCard(c, emptyCardProps{
							Key:      "empty-act-env",
							Icon:     icons.Globe,
							Title:    locales.RailEnvironments(),
							Desc:     "配置 baseUrl 与多套环境变量",
							OnClick: func() {
								props.actions.navigate(routes.WorkbenchEnvironmentsOf(string(props.workspaceID)))
							},
						})
					})
				})

				ui.Box(c).Height(8)
				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				// 3. 常用键盘快捷键速查清单
				shortcutCheatsheet(c, tk, m)

				ui.Box(c).Height(4)
				ui.Row(c).AlignItems(ui.Center).Gap(6).Children(func() {
					ui.Icon(c, icons.CircleHelp).FontSize(12).TextColor(tk.TextMuted)
					ui.Text(c, locales.EmptyWorkspaceTip(keycap("", "K"))).
						FontSize(12).TextColor(tk.TextMuted)
				})
			})
		})
	})
}

type emptyCardProps struct {
	Key     string
	Icon    *ui.SVG
	Title   string
	Desc    string
	OnClick func()
}

func emptyActionCard(c *ui.Context, p emptyCardProps) {
	tk := thV2.Of(c)
	card := ui.ButtonBase(c.Key(p.Key)).
		Grow(1).Height(76).Padding(14, 16).Gap(12).
		Radius(8).Background(tk.BgSurface).
		Border(1, tk.Border).
		Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.04)).
		AlignItems(ui.Center).Cursor(ui.CursorPointer).
		Role(ui.RoleButton).Label(p.Title).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case card.Pressed():
		card.Background(tk.Accent.Alpha(0.08)).Border(1, tk.Accent)
	case card.Hovered():
		card.Background(tk.Accent.Alpha(0.04)).Border(1, tk.Accent).
			Shadow(0, 4, 12, 0, ui.RGBA(0, 0, 0, 0.05))
	}

	if card.Clicked() && p.OnClick != nil {
		p.OnClick()
	}

	card.Children(func() {
		// 左侧小图标容器与右侧箭头交互色
		iconBg := tk.Accent.Alpha(0.1)
		chevronColor := tk.TextMuted
		if card.Hovered() {
			iconBg = tk.Accent.Alpha(0.15)
			chevronColor = tk.Accent
		}

		ui.Box(c).Size(40, 40).Radius(8).
			Background(iconBg).Center().Children(func() {
			ui.Icon(c, p.Icon).FontSize(18).TextColor(tk.Accent)
		})

		// 右侧文字标题与说明
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			ui.Text(c, p.Title).FontSize(13).FontWeight(500).
				TextColor(tk.TextPrimary).SingleLine()
			ui.Text(c, p.Desc).FontSize(11).
				TextColor(tk.TextMuted).SingleLine()
		})

		ui.Icon(c, icons.ChevronDown).Rotate(-90).FontSize(10).TextColor(chevronColor)
	})
}

// shortcutCheatsheet 绘制键盘快捷键速查卡片。
func shortcutCheatsheet(c *ui.Context, tk thV2.Tokens, m thV2.Metric) {
	col1 := []struct{ label, key string }{
		{locales.NewRequest(), keycap("", "N")},
		{locales.NewCollection(), keycap("", "B")},
		{"发送请求", keycap("", "Enter")},
	}
	col2 := []struct{ label, key string }{
		{locales.GlobalSearch(), keycap("", "K")},
		{locales.SwitchEnvironment(), keycap("", "E")},
		{locales.ShowAllCommands(), keycap("shift", "P")},
	}

	ui.Column(c).FillWidth().Gap(10).Children(func() {
		ui.Text(c, "常用快捷键").FontSize(12).FontWeight(600).
			TextColor(tk.TextSecondary)

		ui.Row(c).FillWidth().Gap(24).Children(func() {
			// 左列
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				for _, row := range col1 {
					shortcutItem(c, tk, row.label, row.key)
				}
			})
			// 右列
			ui.Column(c).Grow(1).Gap(8).Children(func() {
				for _, row := range col2 {
					shortcutItem(c, tk, row.label, row.key)
				}
			})
		})
	})
}

func shortcutItem(c *ui.Context, tk thV2.Tokens, label, key string) {
	ui.Row(c).FillWidth().Height(24).AlignItems(ui.Center).Children(func() {
		ui.Text(c, label).FontSize(12).TextColor(tk.TextSecondary).Grow(1)
		ui.Box(c).Padding(2, 6).Radius(4).
			Background(tk.BgSubtle).Border(1, tk.Border).Children(func() {
			ui.Text(c, key).FontSize(11).Font("monospace").
				TextColor(tk.TextMuted)
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

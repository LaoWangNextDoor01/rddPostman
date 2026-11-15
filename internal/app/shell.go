package app

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/config"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// railEntry 是图标栏的一项。label 和 desc 存函数，
// 每一帧按当前语言取值，语言切换后不用重建列表。
type railEntry struct {
	id    string // 稳定的业务 ID，用作 key，不随语言变化
	route string // 空表示暂未接页面
	label func() string
	desc  func() string
	icon  *ui.SVG
	key   ui.Key // 0 表示没有快捷键
}

// railEntries 是图标栏的模块项，顺序即显示顺序。
var railEntries = []railEntry{
	{"home", routerHome, locales.RailHome, locales.RailHomeDesc, icons.Get("house"), ui.Key1},
	{"collections", routeCollections, locales.RailCollections, locales.RailCollectionsDesc, icons.Get("layers"), ui.Key2},
	{"history", routeHistory, locales.RailHistory, locales.RailHistoryDesc, icons.Get("clock"), ui.Key3},
	{"mock", routeMock, locales.RailMock, locales.RailMockDesc, icons.Get("server"), ui.Key4},
	{"network", routeNetwork, locales.RailNetwork, locales.RailNetworkDesc, icons.Get("network"), ui.Key5},
	{"environments", routeEnvironments, locales.RailEnvironments, locales.RailEnvironmentsDesc, icons.Get("globe"), ui.Key6},
	{"demo", routeDemo, locales.RailDemo, locales.RailDemoDesc, icons.Get("bug"), 0},
}

// railBottom 是图标栏底部的设置与帮助。
var railBottom = []railEntry{
	{"settings", routeSettings, locales.RailSettings, locales.RailSettingsDesc, icons.Get("settings"), 0},
	{"help", "", locales.RailHelp, locales.RailHelpDesc, icons.Get("circle-help"), 0},
}

// titleBar 画自绘标题栏，高度与系统窗口按钮所在区域一致，内容垂直居中。
// c.TitleBar() 给出窗口按钮占用的空间：Height 是它们所在的带子，
// Left/Right 是从左右边缘算起的宽度，Bottom 是它们结束的位置
// （全屏时全为 0）。
func (a *App) titleBar(c *ui.Context, bar ui.TitleBar, selected *string) {
	m := theme.Metrics
	tk := theme.Of(c)
	// 按钮的垂直中线就是这条带子的中线，所以直接用 Height 居中。
	h := bar.Height
	if h <= 0 {
		h = m.TitleBarHeight // 全屏或没有窗口按钮时自绘的兜底高度
	}
	ui.Row(c).
		Height(h).
		Padding(0, bar.Right, 0, bar.Left).
		Background(tk.BgSurface).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		DragWindow().
		Children(func() {
			ui.Row(c).Gap(m.Space2).Padding(0, m.Space3).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(m.SizeXS, m.SizeXS).Radius(m.RadiusSM).
					Background(tk.Accent).Center().Children(func() {
					ui.Text(c, "R").FontSize(m.FontXS).Bold().TextColor(tk.OnAccent)
				})
				a.workspaceSelector(c, selected)
			})
			ui.Spacer(c)
			ui.Row(c).Width(440).Height(m.SizeMD).Gap(m.Space1).
				PaddingX(m.Space2).AlignItems(ui.Center).
				Radius(m.RadiusMD).Background(tk.BgBase).Border(m.Border, tk.Border).
				Children(func() {
					ui.Icon(c, icons.Get("search")).FontSize(m.IconXS).TextColor(tk.TextMuted)
					ui.Text(c, locales.SearchPlaceholder()).FontSize(m.FontXS).
						TextColor(tk.TextMuted).SingleLine()
				})
			ui.Spacer(c)
			// 用户头像
			ui.Row(c).Gap(m.Space2).Padding(0, m.Space3).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(m.SizeSM, m.SizeSM).Radius(m.RadiusFull).
					Background(tk.AccentSoft).Center().Children(func() {
					ui.Text(c, "R").FontSize(m.FontXS).Bold().TextColor(tk.Accent)
				})
			})
		})
}

// workspaceSelector 是标题栏里的工作区切换器：Dropdown 里可搜索、切换，
// 底部可以新建工作空间。
func (a *App) workspaceSelector(c *ui.Context, selected *string) {
	m := theme.Metrics
	tk := theme.Of(c)
	if a.store == nil {
		return
	}

	button := ui.ButtonBase(c.Key("workspace-selector")).Height(m.SizeMD).
		PaddingX(m.Space2).Gap(m.Space1).AlignItems(ui.Center).
		Radius(m.RadiusMD).Label(locales.SwitchWorkspace()).Role(ui.RoleButton)
	if button.Hovered() {
		button.Background(tk.BgBase)
	}

	cur := a.workspace(*selected)
	button.Children(func() {
		ui.Icon(c, icons.Get("layers")).FontSize(m.IconSM).TextColor(tk.TextSecondary)
		name := locales.WorkspaceTitle()
		if cur != nil {
			name = cur.Name
		}
		ui.Text(c, name).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		if cur != nil && cur.Default {
			defaultBadge(c)
		}
		ui.Icon(c, icons.Get("chevron-down")).FontSize(m.IconXS).TextColor(tk.TextMuted)
	})

	open := ui.Local(button, "open", func() bool { return false })
	query := ui.Local(button, "query", func() string { return "" })
	creating := ui.Local(button, "creating", func() bool { return false })
	newName := ui.Local(button, "new-name", func() string { return "" })
	newErr := ui.Local(button, "new-error", func() string { return "" })

	if button.Clicked() {
		*open = !*open
		if *open {
			*query = ""
		}
	}

	ui.PopoverBase(c, button, open, func(panel ui.Element) {
		panel.Width(280).MaxHeight(340).Padding(m.Space1).Gap(m.Space1).
			Radius(m.RadiusMD).Background(tk.BgSurface).Border(m.Border, tk.Border)
		panel.Children(func() {
			ui.TextInput(c, query).Height(m.SizeMD).FillWidth().
				Padding(0, m.Space2).Radius(m.RadiusSM).
				Placeholder(locales.SearchWorkspaces())

			list := filterWorkspaces(a.workspaces(), *query)
			for _, ws := range list {
				active := string(ws.ID) == *selected
				row := ui.ButtonBase(c.Key("ws-" + string(ws.ID))).FillWidth().
					Height(m.SizeLG).PaddingX(m.Space2).Gap(m.Space1).
					AlignItems(ui.Center).Radius(m.RadiusSM)
				switch {
				case active:
					row.Background(tk.AccentSoft)
				case row.Hovered():
					row.Background(tk.HoverSurface)
				}
				if row.Clicked() {
					*selected = string(ws.ID)
					*open = false
				}
				row.Children(func() {
					text := ui.Text(c, ws.Name).FontSize(m.FontSM).Grow(1).SingleLine()
					if active {
						text.TextColor(tk.Accent).Bold()
					} else {
						text.TextColor(tk.TextPrimary)
					}
					if ws.Default {
						defaultBadge(c)
					}
				})
			}
			if len(list) == 0 {
				ui.Text(c, locales.NoWorkspaceMatches()).FontSize(m.FontXS).
					TextColor(tk.TextMuted).Padding(m.Space2)
			}

			ui.Divider(c)

			newRow := ui.ButtonBase(c.Key("ws-new")).FillWidth().
				Height(m.SizeLG).PaddingX(m.Space2).Gap(m.Space1).
				AlignItems(ui.Center).Radius(m.RadiusSM)
			if newRow.Hovered() {
				newRow.Background(tk.HoverSurface)
			}
			if newRow.Clicked() {
				*open = false
				*creating = true
				*newName = ""
				*newErr = ""
			}
			newRow.Children(func() {
				ui.Icon(c, icons.Get("plus")).FontSize(m.IconXS).TextColor(tk.TextMuted)
				ui.Text(c, locales.NewWorkspace()).FontSize(m.FontSM).
					TextColor(tk.TextSecondary).Grow(1)
			})
		})
	})

	a.newWorkspaceDialog(c, creating, newName, newErr, selected)
}

// newWorkspaceDialog 在 Dialog 里输入名字创建工作空间。
func (a *App) newWorkspaceDialog(c *ui.Context, open *bool, name, errText, selected *string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(360).Padding(m.Space4).Gap(m.Space3).
			Radius(m.RadiusLG).Background(tk.BgSurface).Border(m.Border, tk.Border)
		panel.Children(func() {
			ui.Text(c, locales.NewWorkspace()).FontSize(m.FontMD).Bold().
				TextColor(tk.TextPrimary)
			ui.TextInput(c, name).Height(m.SizeXL).FillWidth().
				Padding(0, m.Space2).Radius(m.RadiusSM).
				Placeholder(locales.WorkspaceName())
			if *errText != "" {
				ui.Text(c, *errText).FontSize(m.FontXS).TextColor(tk.Danger)
			}
			ui.Row(c).Gap(m.Space2).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					*open = false
				}
				if ui.PrimaryButton(c, locales.Create()).Clicked() {
					if strings.TrimSpace(*name) == "" {
						*errText = locales.WorkspaceNameRequired()
						return
					}
					ws, err := a.store.CreateWorkspace(*name)
					if err != nil {
						*errText = err.Error()
						return
					}
					*selected = string(ws.ID)
					*open = false
					*errText = ""
				}
			})
		})
	})
}

// defaultBadge 画「默认」小标记。
func defaultBadge(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Box(c).Padding(1, m.Space1).Radius(m.RadiusSM).
		Background(tk.BgElevated).Children(func() {
		ui.Text(c, locales.DefaultWorkspace()).FontSize(10).TextColor(tk.TextMuted)
	})
}

// filterWorkspaces 按名字过滤工作区，空查询返回全部。
func filterWorkspaces(list []*store.Workspace, query string) []*store.Workspace {
	q := strings.ToLower(strings.TrimSpace(query))
	if q == "" {
		return list
	}
	out := make([]*store.Workspace, 0, len(list))
	for _, w := range list {
		if strings.Contains(strings.ToLower(w.Name), q) {
			out = append(out, w)
		}
	}
	return out
}

// railTipAlign 决定提示框贴图标的上边、下边还是垂直居中，
// 避免靠近窗口顶部或底部时超出可用区域。
type railTipAlign int

const (
	tipCenter railTipAlign = iota
	tipTop
	tipBottom
)

// iconRail 是窗口最左侧的模块栏：railEntries 是五个模块，railBottom 在底部。
func (a *App) iconRail(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Width(m.IconRailWidth).Background(tk.BgBase).
		BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).
		AlignItems(ui.Center).Padding(m.Space1, 0).Gap(m.Space1).
		Children(func() {
			for i, e := range railEntries {
				align := tipCenter
				if i == 0 {
					align = tipTop
				}
				a.railItem(c, e, align)
			}
			ui.Spacer(c)
			for i, e := range railBottom {
				align := tipCenter
				if i == len(railBottom)-1 {
					align = tipBottom
				}
				a.railItem(c, e, align)
			}
		})
}

func (a *App) railItem(c *ui.Context, e railEntry, align railTipAlign) {
	m := theme.Metrics
	tk := theme.Of(c)
	label, desc := e.label(), e.desc()
	active := e.route != "" && a.active(e.route)

	// 外层占满整条图标栏（52），整列都能响应 hover；
	// 视觉上的 34×34 底色和指示条在内部单独画。
	item := ui.ButtonBase(c.Key(e.id)).Width(m.IconRailWidth).Height(m.SizeXL).
		Justify(ui.Center).AlignItems(ui.Center).
		Label(label).Description(desc).Role(ui.RoleButton)

	hovered := item.Hovered()
	pressed := item.Pressed()
	switch {
	case active:
		item.TextColor(tk.Accent)
	case pressed, hovered:
		item.TextColor(tk.TextPrimary)
	default:
		item.TextColor(tk.TextSecondary)
	}

	item.Children(func() {
		// 居中在图标栏里的 34×34 底色。
		var bg ui.Color
		switch {
		case active && pressed:
			// 选中项按下：强调色底再深一点。
			bg = tk.AccentSoft.Mix(tk.Accent, 0.12)
		case active:
			bg = tk.AccentSoft
		case pressed:
			// 未选中项按下：比 hover 再深一档，和 SurfacePressed 的推导一致。
			bg = tk.HoverSurface.Mix(tk.TextPrimary, 0.08)
		case hovered:
			bg = tk.HoverSurface
		}
		if bg.A > 0 {
			ui.Box(c).Size(m.SizeXL, m.SizeXL).Radius(m.RadiusLG).
				Absolute().
				Left((m.IconRailWidth - m.SizeXL) / 2).
				Top(0).
				ZIndex(-1).
				Background(bg)
		}
		if hovered || pressed {
			// 外层是 52px，Left(0) 就是窗口左缘，不再需要负数偏移。
			ui.Box(c).Width(3).Height(m.Space4).Radius(m.RadiusSM).Background(tk.Accent).
				Absolute().
				Left(0).
				Top((m.SizeXL - m.Space4) / 2)
		}
		ui.Icon(c, e.icon).FontSize(m.IconLG)
	})

	// 菜单按钮 tooltip 提示
	ui.TooltipBase(c, item, func(tip ui.Element) {
		at, self := ui.AnchorRight, ui.AnchorLeft
		switch align {
		case tipTop:
			at, self = ui.AnchorTopRight, ui.AnchorTopLeft
		case tipBottom:
			at, self = ui.AnchorBottomRight, ui.AnchorBottomLeft
		}
		tip.AttachTo(item, at, self).Margin(0, 0, 0, m.Space1).
			Width(240).Gap(m.Space1).Padding(m.Space2, m.Space3).
			Radius(m.RadiusMD).Background(tk.BgElevated).Border(m.Border, tk.Border)
		tip.Children(func() {
			ui.Text(c, label).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
			ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextMuted)
			if e.key != 0 {
				ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
					ui.Box(c).Height(18).Padding(0, m.Space1).Radius(m.RadiusSM).
						Background(tk.BgSurface).Border(m.Border, tk.Border).Center().Children(func() {
						ui.Text(c, shortcutLabel(e.key)).FontSize(m.FontXS).TextColor(tk.TextSecondary)
					})
					ui.Text(c, locales.OpenItem(label)).FontSize(m.FontXS).TextColor(tk.TextMuted)
				})
			}
		})
	})
	if e.route == "" {
		return
	}
	if item.Clicked() || e.key != 0 && item.Shortcut(ui.Cmd, e.key) {
		a.router.Push(e.route)
	}
}

// shortcutLabel 按平台写出快捷键，如 macOS 的 ⌘1 与其它平台的 Ctrl 1。
func shortcutLabel(k ui.Key) string {
	n := strconv.Itoa(int(k-ui.Key1) + 1)
	if runtime.GOOS == "darwin" {
		return "⌘" + n
	}
	return "Ctrl " + n
}

// statusBar 是窗口底部状态条。
func (a *App) statusBar(c *ui.Context, workspace string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(m.StatusBarHeight).Padding(0, m.Space3).Gap(m.Space2).
		AlignItems(ui.Center).
		Background(tk.BgSurface).
		BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border).
		Children(func() {
			ui.Text(c, locales.StatusWorkspace(workspace)).FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Text(c, locales.NotSignedIn()).FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Spacer(c)
			ui.Text(c, fmt.Sprintf("v%s", config.Version)).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
}

// active 报告当前路由是否在 route 下。
func (a *App) active(route string) bool {
	p := a.router.Path()
	return p == route || strings.HasPrefix(p, route+"/")
}

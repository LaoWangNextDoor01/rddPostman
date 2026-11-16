package app

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/config"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
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
func (a *App) titleBar(c *ui.Context, bar ui.TitleBar) {
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
				a.workspaceSelector(c)
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

// workspaceSelector 是标题栏里的工作区切换器：Dropdown 里切换工作区，
// 底部可以新建或管理。
func (a *App) workspaceSelector(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	if a.store == nil {
		return
	}
	snap := a.store.Snapshot()
	cur := snap.ActiveWorkspace()
	var currentID domain.ID
	if cur != nil {
		currentID = cur.ID
	}

	button := ui.ButtonBase(c.Key("workspace-selector")).Height(m.SizeMD).
		PaddingX(m.Space2).Gap(m.Space1).AlignItems(ui.Center).
		Radius(m.RadiusMD).Label(locales.SwitchWorkspace()).Role(ui.RoleButton)
	if button.Hovered() {
		button.Background(tk.BgBase)
	}

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
	creating := ui.Local(button, "creating", func() bool { return false })
	newName := ui.Local(button, "new-name", func() string { return "" })
	newErr := ui.Local(button, "new-error", func() string { return "" })

	if button.Clicked() {
		*open = !*open
	}

	ui.PopoverBase(c, button, open, func(panel ui.Element) {
		// 设计稿 42.工作区切换下拉：320 宽，面板外观由 theme.PopoverPanel 统一。
		panel.Width(320).MaxHeight(360).Padding(m.Space2)
		theme.PopoverPanel(c, panel)
		panel.Children(func() {
			ui.Text(c, locales.SwitchWorkspace()).FontSize(m.FontSM).
				TextColor(tk.TextMuted).Padding(m.Space1, m.Space2)

			ui.Column(c).Gap(m.Space1).Children(func() {
				for _, ws := range snap.Workspaces {
					active := ws.ID == currentID
					row := ui.ButtonBase(c.Key("ws-" + string(ws.ID))).FillWidth().
						Height(44).PaddingX(m.Space2).Gap(m.Space2).
						AlignItems(ui.Center).Radius(m.RadiusMD)
					if active || row.Hovered() {
						row.Background(tk.PopoverItemHover)
					}
					if row.Clicked() {
						_ = a.store.SetActiveWorkspace(ws.ID)
						*open = false
					}
					row.Children(func() {
						ui.Box(c).Size(28, 28).Radius(m.RadiusFull).
							Background(tk.Accent).Center().Children(func() {
							ui.Text(c, workspaceInitials(ws.Name)).FontSize(m.FontXS).
								TextColor(tk.OnAccent)
						})
						ui.Column(c).Grow(1).Gap(1).Children(func() {
							ui.Text(c, ws.Name).FontSize(m.FontSM).
								TextColor(tk.TextPrimary).SingleLine()
							ui.Text(c, locales.WorkspaceCollectionCount(len(ws.Collections))).
								FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
						})
						if active {
							ui.Text(c, "✓").FontSize(m.FontMD).TextColor(tk.Accent)
						}
					})
				}
			})

			ui.Divider(c).Margin(m.Space2, 0)

			ui.Column(c).Gap(m.Space1).Children(func() {
				action := func(key, label string, fn func()) {
					row := ui.ButtonBase(c.Key(key)).FillWidth().Height(28).
						PaddingX(m.Space2).AlignItems(ui.Center).Radius(m.RadiusSM)
					if row.Hovered() {
						row.Background(tk.PopoverItemHover)
					}
					if row.Clicked() {
						fn()
					}
					row.Children(func() {
						ui.Text(c, label).FontSize(m.FontSM).
							TextColor(tk.TextSecondary).Grow(1)
					})
				}
				action("ws-create", "+ "+locales.CreateWorkspaceAction(), func() {
					*open = false
					*creating = true
					*newName = ""
					*newErr = ""
				})
				action("ws-manage", locales.ManageWorkspacesAction(), func() {
					*open = false
					a.router.Push(routeSettings)
				})
			})
		})
	})

	a.newWorkspaceDialog(c, creating, newName, newErr)
}

// newWorkspaceDialog 在 Dialog 里输入名字创建工作空间。
func (a *App) newWorkspaceDialog(c *ui.Context, open *bool, name, errText *string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(360).Padding(m.Space4).Gap(m.Space3)
		theme.DialogPanel(c, panel)
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
					_ = a.store.SetActiveWorkspace(ws.ID)
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

// workspaceInitials 取工作区名的首字母/首字，画在下拉的圆形头像里。
func workspaceInitials(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return "?"
	}
	fields := strings.Fields(name)
	if len(fields) >= 2 && isASCII(fields[0]) && isASCII(fields[1]) {
		first, second := []rune(fields[0]), []rune(fields[1])
		return strings.ToUpper(string(first[0]) + string(second[0]))
	}
	runes := []rune(name)
	if isASCII(name) {
		runes = []rune(strings.ToUpper(name))
	}
	if len(runes) > 2 {
		runes = runes[:2]
	}
	return string(runes)
}

func isASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return false
		}
	}
	return true
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
func (a *App) statusBar(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	workspace := locales.WorkspaceTitle()
	if a.store != nil {
		if cur := a.store.Snapshot().ActiveWorkspace(); cur != nil {
			workspace = cur.Name
		}
	}

	row := ui.Row(c).Height(m.StatusBarHeight).Padding(0, m.Space3).Gap(m.Space2).
		AlignItems(ui.Center).
		Background(tk.BgSurface).
		BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)

	logOpen := ui.Local(row, "open", func() bool { return false })
	errCount := logger.ErrorCount()

	row.Children(func() {
			ui.Text(c, locales.StatusWorkspace(workspace)).FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Text(c, locales.NotSignedIn()).FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Spacer(c)

			// 错误日志按钮（在版本号左侧）
			logBtn := ui.ButtonBase(c.Key("status-bar-log-btn")).
				Height(20).
				PaddingX(m.Space2).
				Gap(m.Space1).
				AlignItems(ui.Center).
				Radius(m.RadiusSM).
				Role(ui.RoleButton)
			if logBtn.Hovered() {
				logBtn.Background(tk.BgElevated)
			}
			if logBtn.Clicked() {
				*logOpen = true
			}
			logBtn.Children(func() {
				if errCount > 0 {
					ui.Icon(c, icons.Get("triangle-alert")).FontSize(m.IconXS).TextColor(tk.Danger)
					ui.Text(c, fmt.Sprintf("%d 错误", errCount)).FontSize(m.FontXS).TextColor(tk.Danger)
				} else {
					ui.Icon(c, icons.Get("square-terminal")).FontSize(m.IconXS).TextColor(tk.TextMuted)
					ui.Text(c, "日志").FontSize(m.FontXS).TextColor(tk.TextMuted)
				}
			})

			ui.Text(c, "·").FontSize(m.FontXS).TextColor(tk.TextMuted)
			ui.Text(c, fmt.Sprintf("v%s", config.Version)).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})

	a.errorLogDialog(c, logOpen)
}

// errorLogDialog 弹出系统与错误日志排查弹窗。
func (a *App) errorLogDialog(c *ui.Context, open *bool) {
	m := theme.Metrics
	tk := theme.Of(c)
	errCount := logger.ErrorCount()
	logs := logger.Entries()

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		copied := ui.Local(panel, "copied", func() bool { return false })
		backdrop.Background(tk.Overlay)
		panel.Width(680).Height(500).Padding(m.Space4).Gap(m.Space3)
		theme.DialogPanel(c, panel)
		panel.Children(func() {
			// 1. 顶部标题栏与快捷操作
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
				ui.Icon(c, icons.Get("square-terminal")).FontSize(m.IconMD).TextColor(tk.Accent)
				ui.Text(c, "系统与错误日志").FontSize(m.FontMD).Bold().TextColor(tk.TextPrimary)
				if errCount > 0 {
					ui.Box(c).Padding(1, m.Space2).Radius(m.RadiusFull).
						Background(tk.BgElevated).Children(func() {
						ui.Text(c, fmt.Sprintf("%d 个错误", errCount)).FontSize(m.FontXS).TextColor(tk.Danger)
					})
				}
				ui.Spacer(c)

				// 复制全部按钮
				copyBtn := ui.ButtonBase(c.Key("log-copy-all")).
					Height(26).PaddingX(m.Space2).Gap(m.Space1).
					Radius(m.RadiusSM).AlignItems(ui.Center).Role(ui.RoleButton)
				if copyBtn.Hovered() {
					copyBtn.Background(tk.BgElevated)
				}
				if copyBtn.Clicked() {
					c.Services().WriteClipboard(logger.FormatAll())
					*copied = true
				}
				copyBtn.Children(func() {
					ui.Icon(c, icons.Get("copy")).FontSize(m.IconXS).TextColor(tk.TextSecondary)
					copyLabel := "复制全部"
					if *copied {
						copyLabel = "已复制"
					}
					ui.Text(c, copyLabel).FontSize(m.FontXS).TextColor(tk.TextSecondary)
				})

				// 清空日志按钮
				clearBtn := ui.ButtonBase(c.Key("log-clear-all")).
					Height(26).PaddingX(m.Space2).Gap(m.Space1).
					Radius(m.RadiusSM).AlignItems(ui.Center).Role(ui.RoleButton)
				if clearBtn.Hovered() {
					clearBtn.Background(tk.BgElevated)
				}
				if clearBtn.Clicked() {
					logger.Clear()
					*copied = false
				}
				clearBtn.Children(func() {
					ui.Icon(c, icons.Get("trash")).FontSize(m.IconXS).TextColor(tk.TextMuted)
					ui.Text(c, "清空").FontSize(m.FontXS).TextColor(tk.TextMuted)
				})

				// 关闭按钮
				closeBtn := ui.ButtonBase(c.Key("log-close-btn")).
					Size(26, 26).Radius(m.RadiusSM).Center().Role(ui.RoleButton)
				if closeBtn.Hovered() {
					closeBtn.Background(tk.BgElevated)
				}
				if closeBtn.Clicked() {
					*open = false
					*copied = false
				}
				closeBtn.Children(func() {
					ui.Icon(c, icons.Get("x")).FontSize(m.IconXS).TextColor(tk.TextMuted)
				})
			})

			// 2. 日志展示主区域（终端控制台风格，深色暗底 / 边框，可滚动）
			ui.Box(c).Grow(1).Radius(m.RadiusMD).Background(tk.BgBase).
				Border(m.Border, tk.Border).Padding(m.Space2).Children(func() {
				if len(logs) == 0 {
					ui.Box(c).Fill().Center().Children(func() {
						ui.Column(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
							ui.Icon(c, icons.Get("square-terminal")).FontSize(36).TextColor(tk.TextMuted)
							ui.Text(c, "暂无错误与日志，系统运行正常").FontSize(m.FontSM).TextColor(tk.TextMuted)
						})
					})
					return
				}
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(m.Space1).Children(func() {
						for i, entry := range logs {
							ui.Row(c).Key(fmt.Sprintf("log-row-%d", i)).
								Gap(m.Space2).PaddingY(2).AlignItems(ui.Start).Children(func() {
								// 时间
								ui.Text(c, entry.Time.Format("15:04:05.000")).
									FontSize(m.FontXS).TextColor(tk.TextMuted)
								// 级别 Badge
								switch entry.Level {
								case logger.LevelError:
									ui.Text(c, "[ERROR]").FontSize(m.FontXS).Bold().TextColor(tk.Danger)
								case logger.LevelWarn:
									ui.Text(c, "[WARN]").FontSize(m.FontXS).Bold().TextColor(tk.Warning)
								default:
									ui.Text(c, "[INFO]").FontSize(m.FontXS).TextColor(tk.Accent)
								}
								// 消息内容
								ui.Text(c, entry.Message).FontSize(m.FontXS).
									TextColor(tk.TextPrimary).Grow(1)
							})
						}
					})
				})
			})

			// 3. 底部信息行与关闭按钮
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, fmt.Sprintf("共记录 %d 条事件，自动捕获启动与运行时异常", len(logs))).
					FontSize(m.FontXS).TextColor(tk.TextMuted)
				ui.Spacer(c)
				if ui.Button(c, locales.Cancel()).Clicked() {
					*open = false
					*copied = false
				}
			})
		})
	})
}

// active 报告当前路由是否在 route 下。
func (a *App) active(route string) bool {
	p := a.router.Path()
	return p == route || strings.HasPrefix(p, route+"/")
}

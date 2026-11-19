package app

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/config"
	"rddpostman/internal/logger"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// titleBar 画自绘标题栏，高度与系统窗口按钮所在区域一致，内容垂直居中。
// c.TitleBar() 给出窗口按钮占用的空间：Height 是它们所在的带子，
// Left/Right 是从左右边缘算起的宽度，Bottom 是它们结束的位置
// （全屏时全为 0）。
func (a *App) titleBar(c *ui.Context) {
	m, tk, bar := thV2.Metrics, thV2.Of(c), c.TitleBar()
	// 标题栏高度：实测与 Postman 对齐为 36（内容 35 + 底部 1px 分割线），
	// 设计稿当初的 36 测量也是对的；只有全屏或没有系统按钮时才用兜底值。
	h := bar.Height
	if h <= 0 {
		h = m.TitleBarHeight
	}

	ui.Row(c).
		Height(h).
		Padding(0, bar.Right, 0, bar.Left).
		Background(tk.BgSurface).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		DragWindow().
		Children(func() {
			a.menuButton(c)
			a.workspaceSelector(c)
			ui.Spacer(c)
			a.searchInput(c)
			ui.Spacer(c)
			// 用户头像
			ui.Row(c).Gap(m.Space15).Padding(0, m.Space3).AlignItems(ui.Center).FillHeight().Children(func() {
				a.bellButton(c)
				a.settingButton(c)
				a.userAvatar(c)
			})
		})
}

// 如果是windows则显示一个菜单箭头按钮
func (a *App) menuButton(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	fakeBtn := ui.Box(c).FillHeight().Center().PaddingX(m.Space2).TextColor(tk.TextSecondary)
	if fakeBtn.Hovered() {
		fakeBtn.Background(tk.BgHover).TextColor(tk.TextPrimary)
	}
	fakeBtn.Children(func() {
		ui.Icon(c, icons.ChevronDown).FontSize(m.IconLG)
	})
}

// searchInput 是标题栏中间的全局搜索入口：默认灰底 + TextMuted，
// hover 白底 + 文字转深，并淡入 Ctrl+K 角标（默认全透明，布局不跳动）；
// 点击与快捷键共用一个入口打开全局搜索。
func (a *App) searchInput(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	box := ui.ButtonBase(c.Key(keyTitlebarSearch)).MinWidth(160).Height(m.SizeMD+2).
		Border(m.Border, tk.Border).
		PaddingX(m.Space2).Gap(m.Space1).AlignItems(ui.Center).
		JustifyItems(ui.Center).
		Radius(m.RadiusMD).Background(tk.BgSubtle).
		Cursor(ui.CursorPointer).Label(locales.SearchPlaceholder()).Role(ui.RoleButton).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Base})
	fg := tk.TextMuted
	hoxHovered := false
	if box.Hovered() {
		box.Background(tk.BgBase)
		fg = tk.TextPrimary
		hoxHovered = true
	}
	box.Children(func() {
		ui.Icon(c, icons.Search).FontSize(m.IconXS).TextColor(fg)
		ui.Text(c, locales.SearchPlaceholder()).FontSize(m.FontSM).
			TextColor(fg).SingleLine()
		// 快捷键角标：默认透明，hover 才显示
		cap := ui.Box(c).Children(func() {
			ui.Text(c, shortcutLabel(ui.KeyK)).FontSize(m.FontSM).MarginStart(m.Space05).
				TextColor(tk.TextSecondary).SingleLine()
		})
		if !hoxHovered {
			cap.Opacity(0)
		}
	})
	if box.Clicked() {
		// TODO: 打开全局搜索；同时注册 Ctrl/Cmd+K 快捷键走同一入口
	}
}

// 提醒消息
func (a *App) bellButton(c *ui.Context) {
	if v2.IconButton(c, v2.IconButtonOpts{
		Key: keyTitlebarBell, Label: "通知", Icon: icons.Bell, Tooltip: "通知",
	}).Clicked() {
		// TODO: 打开通知中心 Popover
	}
}

// 设置
func (a *App) settingButton(c *ui.Context) {
	if v2.IconButton(c, v2.IconButtonOpts{
		Key: keyTitlebarSettings, Label: "设置", Icon: icons.Settings, Tooltip: "设置",
	}).Clicked() {
		// TODO: 打开设置 Dialog
	}
}

// 用户头像按钮
func (a *App) userAvatar(c *ui.Context) {
	if v2.IconButton(c, v2.IconButtonOpts{
		Key: keyTitlebarAvatar, Label: "账号", Icon: icons.UserRound,
		Tooltip: "账号", Radius: thV2.Metrics.RadiusFull,
	}).Clicked() {
		// TODO: 打开账号菜单
	}
}

// workspaceSelector 是标题栏里的工作区切换器：Dropdown 里切换工作区，
// 底部可以新建或管理。
func (a *App) workspaceSelector(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	if a.store == nil {
		return
	}
	snap := a.store.Snapshot()
	cur := snap.ActiveWorkspace()

	button := ui.ButtonBase(c.Key(keyWorkspaceSelector)).Height(m.SizeMD + 2).
		PaddingX(m.Space2).Gap(m.Space1).AlignItems(ui.Center).
		Radius(m.RadiusMD).Label(locales.SwitchWorkspace()).Role(ui.RoleButton).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Base})
	if button.Hovered() {
		button.Background(tk.BgHover)
	}

	button.Children(func() {
		ui.Icon(c, icons.Layers).FontSize(m.IconSM).TextColor(tk.TextSecondary)
		name := locales.WorkspaceTitle()
		if cur != nil {
			name = cur.Name
		}
		ui.Text(c, name).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		ui.Icon(c, icons.ChevronDown).FontSize(m.IconXS).TextColor(tk.TextMuted)
	})

	open := ui.Local(button, "open", func() bool {
		return false
	})

	if button.Clicked() {
		*open = !*open
	}

	v2.Popover(c, button, open, v2.PopoverOpts{Width: 300}, func(panel ui.Element) {
		// 内容按设计稿 37:12830「overlay / workspace dropdown」重绘：
		// 搜索行 → 工作区列表 → 1px 分割线 → 48px 底部操作行。

		// 搜索行：内边距 12；搜索框 高32 圆角8 白底。
		wsKw := ui.Local(panel, "search-value", func() string {
			return ""
		})
		ui.Row(c).FillWidth().Padding(m.Space2, m.Space2, 0, m.Space2).Children(func() {
			in := ui.TextInputBase(c.Key("ws-search"), wsKw).FillWidth().
				Height(32).Padding(0, m.Space3).Radius(8).
				Background(tk.BgSurface).Border(m.Border, tk.Border).
				FontSize(m.FontMD).TextColor(tk.TextPrimary).
				Placeholder("Search workspaces...")
			if in.Focused() {
				in.Border(m.Border, tk.Focus)
			}
		})

		// 工作区列表（27-全局框架-Workspace列表Hover）：
		// 行高 40、内边距 [10,16]、间距 12；左侧 32×32 圆角6 灰底 workspace 图标，
		// 名称 14 Medium #212121；hover 整行 #F2F2F2。
		ui.Scroll(c).MaxHeight(160).PaddingY(m.Space2).Children(func() {
			ui.Column(c).Gap(m.Space05).PaddingX(m.Space1).Children(func() {
				for _, v := range a.store.Snapshot().Workspaces {
					row := ui.ButtonBase(c.Key("ws-"+string(v.ID))).FillWidth().
						Height(30).Padding(10, 16).Radius(m.RadiusMD).
						AlignItems(ui.Center).Gap(m.Space3).Label(v.Name)
					if row.Hovered() {
						row.Background(tk.BgSubtle) // #F2F2F2
					}
					if row.Clicked() {
						_ = a.store.SetActiveWorkspace(v.ID)
						*open = false
					}
					if cur.ID == v.ID {
						row.Background(tk.BgSubtle)
					}
					row.Children(func() {
						ui.Box(c).Size(26, 26).Radius(6).Background(tk.BgSubtle).
							AlignItems(ui.Center).Justify(ui.Center).Children(func() {
							ui.Icon(c, icons.Layers).FontSize(m.IconSM).TextColor(tk.TextMuted)
						})
						ui.Text(c, v.Name).FontSize(m.FontMD).FontWeight(500).
							TextColor(tk.TextPrimary).SingleLine().Grow(1)
					})
				}
			})
		})

		// 分割线
		ui.Box(c).FillWidth().Height(m.Border).Background(tk.Border)

		// 底部操作行：高48，左右各一个按钮（图标+文字），中间 1px 竖分隔线。
		ui.Row(c).FillWidth().Height(32).Children(func() {
			ui.ButtonBase(c.Key("ws-create")).Grow(1).Height(32).
				AlignItems(ui.Center).Justify(ui.Center).Gap(m.Space2).
				Label("创建").Children(func() {
				ui.Icon(c, icons.Plus).FontSize(14).TextColor(tk.TextPrimary)
				ui.Text(c, "创建").FontSize(m.FontMD).FontWeight(500).
					TextColor(tk.TextPrimary)
			})
			ui.Box(c).Width(m.Border).Height(24).Background(tk.Border)
			ui.ButtonBase(c.Key("ws-view-all")).Grow(1).Height(32).
				AlignItems(ui.Center).Justify(ui.Center).Gap(m.Space2).
				Label("查看所有").Children(func() {
				ui.Icon(c, icons.LayoutGrid).FontSize(16).TextColor(tk.TextPrimary)
				ui.Text(c, "查看所有").FontSize(m.FontMD).TextColor(tk.TextPrimary)
			})
		})
	})
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
		logBtn := ui.ButtonBase(c.Key(keyStatusBarLogBtn)).
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
				copyBtn := ui.ButtonBase(c.Key(keyLogCopyAll)).
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
				clearBtn := ui.ButtonBase(c.Key(keyLogClearAll)).
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
				closeBtn := ui.ButtonBase(c.Key(keyLogCloseBtn)).
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
							ui.Row(c.Key(fmt.Sprintf("log-row-%d", i))).
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

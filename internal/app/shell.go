package app

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/features/workspace"
	"rddpostman/internal/config"
	"rddpostman/internal/logger"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// titleBar 画自绘标题栏，高度与系统窗口按钮所在区域一致，内容垂直居中。
// c.TitleBar() 给出窗口按钮占用的空间：Height 是它们所在的带子，
// Left/Right 是从左右边缘算起的宽度，Bottom 是它们结束的位置
// （全屏时全为 0）。
func (a *App) titleBar(c *ui.Context, settingsOpen, searchOpen *bool, searchQuery *string, searchFocus, exportOpen *bool) {
	m, tk, bar := thV2.Metrics, thV2.Of(c), c.TitleBar()
	// 标题栏高度：实测与 Postman 对齐为 36（内容 35 + 底部 1px 分割线），
	// 设计稿当初的 36 测量也是对的；只有全屏或没有系统按钮时才用兜底值。
	h := bar.Height
	if h <= 0 {
		h = m.TitleBarHeight
	}

	ui.Row(c).
		Height(h).
		AlignItems(ui.Center).
		Padding(0, bar.Right, 0, bar.Left).
		Background(tk.BgSurface).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		DragWindow().
		Children(func() {
			if a.showWindowsMenu() {
				a.menuButton(c, settingsOpen, exportOpen)
			}
			a.navButtons(c)
			a.workspaceSelector(c)
			ui.Spacer(c)
			a.searchInput(c, searchOpen, searchQuery, searchFocus)
			ui.Spacer(c)
			// 右侧全局操作：设置（全局弹窗）
			ui.Row(c).Gap(m.Space15).Padding(0, m.Space3).AlignItems(ui.Center).FillHeight().Children(func() {
				a.settingButton(c, settingsOpen)
			})
		})
}

// navButtons 绘制路由历史后退/前进按钮（对齐 Pixso 顶部栏 < >）。
func (a *App) navButtons(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	canBack := a.router.CanGoBack()
	canFwd := a.router.CanGoForward()

	ui.Row(c).AlignItems(ui.Center).Gap(2).Padding(0, m.Space1).FillHeight().Children(func() {
		backBtn := ui.ButtonBase(c.Key(keyTitlebarBack)).
			Size(24, 24).Radius(m.RadiusSM).Center().
			Role(ui.RoleButton).Label(locales.Back()).Tooltip(locales.Back()).
			Disabled(!canBack)
		if canBack {
			backBtn.Cursor(ui.CursorPointer)
			switch {
			case backBtn.Pressed():
				backBtn.Background(tk.BgSelected)
			case backBtn.Hovered():
				backBtn.Background(tk.BgHover)
			}
			if backBtn.Clicked() {
				a.router.Back()
			}
		}
		backBtn.Children(func() {
			fg := tk.TextDisabled
			if canBack {
				fg = tk.TextSecondary
			}
			ui.Icon(c, icons.ArrowLeft).FontSize(m.IconXS).TextColor(fg)
		})

		fwdBtn := ui.ButtonBase(c.Key(keyTitlebarForward)).
			Size(24, 24).Radius(m.RadiusSM).Center().
			Role(ui.RoleButton).Label(locales.Forward()).Tooltip(locales.Forward()).
			Disabled(!canFwd)
		if canFwd {
			fwdBtn.Cursor(ui.CursorPointer)
			switch {
			case fwdBtn.Pressed():
				fwdBtn.Background(tk.BgSelected)
			case fwdBtn.Hovered():
				fwdBtn.Background(tk.BgHover)
			}
			if fwdBtn.Clicked() {
				a.router.Forward()
			}
		}
		fwdBtn.Children(func() {
			fg := tk.TextDisabled
			if canFwd {
				fg = tk.TextSecondary
			}
			ui.Icon(c, icons.ArrowRight).FontSize(m.IconXS).TextColor(fg)
		})
	})
}

// menuButton 绘制 Windows 自绘应用菜单按钮：
// 触发按钮为 26x26 圆角 4px 汉堡图标，展开 300px 宽度弹窗，包含带快捷键的 4 个分组与 9 项自绘菜单项。
func (a *App) menuButton(c *ui.Context, settingsOpen, exportOpen *bool) {
	m, tk := thV2.Metrics, thV2.Of(c)
	btn := ui.ButtonBase(c.Key("titlebar-app-menu")).
		Size(26, 26).
		Radius(m.RadiusSM).
		Center().
		Margin(0, m.Space1, 0, m.Space2).
		Role(ui.RoleButton).
		Label(locales.AppMenu()).
		Tooltip(locales.AppMenu()).
		Cursor(ui.CursorPointer).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Base})

	open := ui.Local(btn, "open", func() bool {
		return false
	})

	if btn.Clicked() {
		*open = !*open
	}

	switch {
	case btn.Pressed():
		btn.Background(tk.BgSelected)
	case *open || btn.Hovered():
		btn.Background(tk.BgHover)
	default:
		btn.Background(ui.Transparent)
	}

	btn.Children(func() {
		ui.Icon(c, icons.Menu).FontSize(15).TextColor(tk.TextPrimary)
	})

	v2.Popover(c, btn, open, v2.PopoverOpts{Width: 300}, func(panel ui.Element) {
		ui.Column(c).Padding(6).Children(func() {
			// 分组 1：创建相关（对齐 Pixso 52:20244 - 52:20270）
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:     icons.Send,
				Key:      "menu-new-request",
				Label:    locales.MenuNewRequest(),
				Shortcut: "Ctrl+N",
				Height:   32,
			}).Clicked() {
				a.triggerNewRequest()
				*open = false
			}

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:     icons.FolderPlus,
				Key:      "menu-new-collection",
				Label:    locales.MenuNewCollection(),
				Shortcut: "Ctrl+Shift+N",
				Height:   32,
			}).Clicked() {
				a.triggerNewCollection()
				*open = false
			}

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:   icons.LayoutGrid,
				Key:    "menu-new-workspace",
				Label:  locales.MenuNewWorkspace(),
				Height: 32,
			}).Clicked() {
				a.router.Push(routes.HubWorkspaceNew)
				*open = false
			}

			// 分割线 1（对齐 Pixso 52:20271，1px #EDEDED + 上下 4px 间距）
			ui.Box(c).FillWidth().PaddingY(4).Children(func() {
				v2.PopoverDivider(c)
			})

			// 分组 2：导入导出（对齐 Pixso 52:20273 - 52:20289）
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:     icons.Upload,
				Key:      "menu-import",
				Label:    locales.MenuImport(),
				Shortcut: "Ctrl+O",
				Height:   32,
			}).Clicked() {
				a.triggerImportPostman()
				*open = false
			}

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:   icons.Download,
				Key:    "menu-export",
				Label:  locales.MenuExport(),
				Height: 32,
			}).Clicked() {
				if exportOpen != nil {
					*exportOpen = true
				}
				*open = false
			}

			// 分割线 2（对齐 Pixso 52:20290）
			ui.Box(c).FillWidth().PaddingY(4).Children(func() {
				v2.PopoverDivider(c)
			})

			// 分组 3：设置与帮助（对齐 Pixso 52:20292 - 52:20316）
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:     icons.Settings,
				Key:      "menu-settings",
				Label:    locales.MenuSettings(),
				Shortcut: "Ctrl+,",
				Height:   32,
			}).Clicked() {
				if settingsOpen != nil {
					*settingsOpen = true
				}
				*open = false
			}

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:   icons.CircleHelp,
				Key:    "menu-help",
				Label:  locales.MenuHelp(),
				Height: 32,
			}).Clicked() {
				a.OpenSettings("shortcuts")
				*open = false
			}

			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:   icons.Info,
				Key:    "menu-about",
				Label:  locales.MenuAbout(),
				Height: 32,
			}).Clicked() {
				a.OpenSettings("about")
				*open = false
			}

			// 分割线 3（对齐 Pixso 52:20317）
			ui.Box(c).FillWidth().PaddingY(4).Children(func() {
				v2.PopoverDivider(c)
			})

			// 分组 4：退出应用（对齐 Pixso 52:20319，默认态与普通菜单项一致，仅 hover/press 为危险红底红字 + Alt+F4）
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon:     icons.LogOut,
				Key:      "menu-quit",
				Label:    locales.MenuQuit(),
				Shortcut: "Alt+F4",
				Danger:   true,
				Height:   32,
			}).Clicked() {
				*open = false
				a.quit()
			}
		})
	})
}

// searchInput 是标题栏中间的全局搜索入口：默认灰底 + TextMuted，
// hover 白底 + 文字转深，并淡入 Ctrl+K 角标（默认全透明，布局不跳动）；
// 点击与快捷键共用一个入口打开全局搜索。
func (a *App) searchInput(c *ui.Context, searchOpen *bool, searchQuery *string, searchFocus *bool) {
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
	if box.Clicked() && searchOpen != nil {
		*searchOpen = true
		if searchQuery != nil {
			*searchQuery = ""
		}
		if searchFocus != nil {
			*searchFocus = true
		}
	}
}

// 设置（在 RootLayout 层打开全局设置弹窗，不改变当前路由）
func (a *App) settingButton(c *ui.Context, settingsOpen *bool) {
	if v2.IconButton(c, v2.IconButtonOpts{
		Key: keyTitlebarSettings, Label: locales.SettingsTitle(), Icon: icons.Settings, Tooltip: locales.SettingsTitle(),
	}).Clicked() {
		if settingsOpen != nil {
			*settingsOpen = true
		}
	}
}

// workspaceSelector 是标题栏里的工作区切换器：Dropdown 里切换工作区，
// 底部可以新建或管理（对齐设计稿 37:12830「overlay / workspace dropdown」）。
func (a *App) workspaceSelector(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	if a.store == nil {
		return
	}
	snap := a.store.Snapshot()
	cur := snap.ActiveWorkspace()

	button := ui.ButtonBase(c.Key(keyWorkspaceSelector)).Height(m.SizeMD + 2).
		PaddingX(m.Space2).Gap(m.Space15).AlignItems(ui.Center).
		Radius(m.RadiusMD).Cursor(ui.CursorPointer).
		Label(locales.SwitchWorkspace()).Role(ui.RoleButton).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	open := ui.Local(button, "open", func() bool {
		return false
	})

	if button.Clicked() {
		*open = !*open
	}

	switch {
	case *open:
		button.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.55))
	case button.Pressed():
		button.Background(tk.BgSelected).Border(m.Border, ui.Transparent)
	case button.Hovered():
		button.Background(tk.BgHover).Border(m.Border, ui.Transparent)
	default:
		button.Background(ui.Transparent).Border(m.Border, ui.Transparent)
	}

	button.Children(func() {
		iconPath := ""
		name := locales.WorkspaceTitle()
		if cur != nil {
			name = cur.Name
			iconPath = cur.Icon
		}
		workspace.RenderIconBadge(c, iconPath, 18, 4)
		ui.Text(c, name).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()
		chev := ui.Icon(c, icons.ChevronDown).FontSize(m.IconXS).TextColor(tk.TextMuted)
		if *open {
			chev.Rotate(180).TextColor(tk.Accent)
		}
	})

	v2.Popover(c, button, open, v2.PopoverOpts{Width: 276}, func(panel ui.Element) {
		// 内层容器按 RadiusLG 裁切，确保底部按钮 hover 背景贴合弹层圆角且不裁切外层阴影。
		// 尺寸刻度与标题栏、侧栏及 Windows 应用菜单保持一致（统一 12px 桌面紧凑密度）。
		ui.Column(c).FillWidth().Radius(m.RadiusLG).Clip().Children(func() {
			// 1. 搜索行：四周 8px 留白，输入框高 28px 圆角 6px，12px 字号，聚焦带蓝色外环。
			wsKw := ui.Local(panel, "search-value", func() string {
				return ""
			})
			ui.Row(c).FillWidth().Padding(m.Space2, m.Space2, m.Space15, m.Space2).Children(func() {
				in := ui.TextInputBase(c.Key("ws-search"), wsKw).FillWidth().
					Height(m.SizeLG).Padding(0, m.Space2).Radius(m.RadiusMD).
					Background(tk.BgSurface).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).
					Placeholder(locales.SearchWorkspaces()).PlaceholderColor(tk.TextMuted).
					Label(locales.SearchDropdownWorkspaces())
				switch {
				case in.Focused():
					in.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 2, tk.FocusRing)
				case in.Hovered():
					in.Border(m.Border, tk.BorderStrong)
				default:
					in.Border(m.Border, tk.BorderPopover)
				}
			})

			// 2. 工作区列表：单行高 32px、图标 20×20、12px Medium 字号，与全局菜单密度统一。
			kw := strings.ToLower(strings.TrimSpace(*wsKw))
			ui.Scroll(c).FillWidth().MinHeight(88).MaxHeight(200).Children(func() {
				ui.Column(c).FillWidth().Padding(m.Space05, m.Space15, m.Space15, m.Space15).Gap(m.Space05).Children(func() {
					matched := 0
					for _, v := range a.store.Snapshot().Workspaces {
						if kw != "" && !strings.Contains(strings.ToLower(v.Name), kw) {
							continue
						}
						matched++
						wsItem := v
						isActive := cur != nil && cur.ID == wsItem.ID
						row := ui.ButtonBase(c.Key(keyPrefixWS+string(wsItem.ID))).FillWidth().
							Height(m.SizeXL).Padding(0, m.Space2).Radius(m.RadiusSM).
							AlignItems(ui.Center).Gap(m.Space2).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(wsItem.Name).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						switch {
						case row.Pressed():
							row.Background(tk.BgSelected)
						case row.Hovered():
							row.Background(tk.BgHover)
						case isActive:
							row.Background(tk.BgSubtle)
						}
						if row.Clicked() {
							_ = a.store.SetActiveWorkspace(wsItem.ID)
							a.router.Push(routes.WorkbenchCollectionsOf(string(wsItem.ID)))
							*open = false
						}
						row.Children(func() {
							workspace.RenderIconBadge(c, wsItem.Icon, 20, 4)
							ui.Text(c, wsItem.Name).FontSize(m.FontSM).FontWeight(500).
								TextColor(tk.TextPrimary).SingleLine().Grow(1)
							if isActive {
								ui.Icon(c, icons.Check).FontSize(m.IconXS).TextColor(tk.Accent)
							}
						})
					}
					if matched == 0 {
						ui.Box(c).FillWidth().Height(72).Center().Children(func() {
							ui.Text(c, locales.NoWorkspaceMatches()).FontSize(m.FontXS).TextColor(tk.TextMuted)
						})
					}
				})
			})

			// 3. 顶部分割线
			ui.Box(c).FillWidth().Height(m.Border).Background(tk.Border)

			// 4. 底部操作行：高 36px，左右严格 50/50 等分，13px 图标 + 12px Medium 文字，中间 1px 贯通竖分隔线。
			ui.Row(c).FillWidth().Height(36).AlignItems(ui.Stretch).Children(func() {
				createBtn := ui.ButtonBase(c.Key("ws-create")).Grow(1).Basis(0).FillHeight().
					AlignItems(ui.Center).Justify(ui.Center).Gap(m.Space15).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.Create()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case createBtn.Pressed():
					createBtn.Background(tk.BgSelected)
				case createBtn.Hovered():
					createBtn.Background(tk.BgHover)
				}
				if createBtn.Clicked() {
					a.router.Push(routes.HubWorkspaceNew)
					*open = false
				}
				createBtn.Children(func() {
					icColor := tk.TextSecondary
					if createBtn.Hovered() || createBtn.Pressed() {
						icColor = tk.TextPrimary
					}
					ui.Icon(c, icons.Plus).FontSize(13).TextColor(icColor)
					ui.Text(c, locales.Create()).FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()
				})

				ui.Box(c).Width(m.Border).FillHeight().Background(tk.Border)

				viewAllBtn := ui.ButtonBase(c.Key("ws-view-all")).Grow(1).Basis(0).FillHeight().
					AlignItems(ui.Center).Justify(ui.Center).Gap(m.Space15).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.ViewAll()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case viewAllBtn.Pressed():
					viewAllBtn.Background(tk.BgSelected)
				case viewAllBtn.Hovered():
					viewAllBtn.Background(tk.BgHover)
				}
				if viewAllBtn.Clicked() {
					a.router.Push(routes.HubWorkspaces)
					*open = false
				}
				viewAllBtn.Children(func() {
					icColor := tk.TextSecondary
					if viewAllBtn.Hovered() || viewAllBtn.Pressed() {
						icColor = tk.TextPrimary
					}
					ui.Icon(c, icons.LayoutGrid).FontSize(13).TextColor(icColor)
					ui.Text(c, locales.ViewAll()).FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()
				})
			})
		})
	})
}

// shortcutLabel 按平台写出快捷键，如 macOS 的 ⌘K / ⌘1 与其它平台的 Ctrl K / Ctrl 1。
func shortcutLabel(k ui.Key) string {
	var name string
	switch {
	case k >= ui.KeyA && k <= ui.KeyZ:
		name = string(rune('A' + int(k-ui.KeyA)))
	case k >= ui.Key0 && k <= ui.Key9:
		name = strconv.Itoa(int(k - ui.Key0))
	case k == ui.KeyComma:
		name = ","
	default:
		name = fmt.Sprintf("%v", k)
	}
	if runtime.GOOS == "darwin" {
		return "⌘" + name
	}
	return "Ctrl " + name
}

// statusBar 是窗口底部状态条（高 24px、左 Local mode / Console，右 Globals / Tools / 版本号）。
func (a *App) statusBar(c *ui.Context, globalsOpen *bool) {
	m, tk := thV2.Metrics, thV2.Of(c)

	row := ui.Row(c).Height(24).Padding(0, m.Space3).Gap(12).
		AlignItems(ui.Center).
		Background(tk.BgStatusBar).
		BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)

	logOpen := ui.Local(row, "open", func() bool { return false })
	errCount := logger.ErrorCount()

	row.Children(func() {
		// 左侧：Local mode · Console（点击 Console 打开日志弹窗）
		ui.Row(c).AlignItems(ui.Center).Gap(4).Children(func() {
			ui.Icon(c, icons.TriangleAlert).FontSize(11).TextColor(tk.TextMuted)
			ui.Text(c, locales.StatusLocalMode()).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
		})

		logBtn := ui.ButtonBase(c.Key(keyStatusBarLogBtn)).
			Height(18).PaddingX(6).Gap(4).AlignItems(ui.Center).Radius(4).
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.StatusConsole())
		switch {
		case logBtn.Pressed():
			logBtn.Background(tk.BgSelected)
		case logBtn.Hovered():
			logBtn.Background(tk.BgHover)
		}
		if logBtn.Clicked() {
			*logOpen = true
		}
		logBtn.Children(func() {
			if errCount > 0 {
				ui.Icon(c, icons.TriangleAlert).FontSize(11).TextColor(tk.Danger)
				ui.Text(c, locales.StatusConsoleCount(strconv.Itoa(errCount))).FontSize(11).TextColor(tk.Danger).SingleLine()
			} else {
				ui.Text(c, locales.StatusConsole()).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
			}
		})

		ui.Spacer(c)

		// 右侧：Globals（可点击打开全局变量弹窗） · Tools · 版本号
		globalsLabel := locales.Globals()
		if a.store != nil {
			if n := len(a.store.Snapshot().Globals); n > 0 {
				globalsLabel = locales.StatusGlobalsCount(strconv.Itoa(n))
			}
		}
		globalsBtn := ui.ButtonBase(c.Key(keyStatusBarGlobalsBtn)).
			Height(18).PaddingX(6).AlignItems(ui.Center).Radius(4).
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.Globals())
		switch {
		case globalsBtn.Pressed():
			globalsBtn.Background(tk.BgSelected)
		case globalsBtn.Hovered():
			globalsBtn.Background(tk.BgHover)
		}
		if globalsBtn.Clicked() && globalsOpen != nil {
			*globalsOpen = true
		}
		globalsBtn.Children(func() {
			ui.Text(c, globalsLabel).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
		})

		toolsBtn := ui.ButtonBase(c.Key(keyStatusBarToolsBtn)).
			Height(18).PaddingX(6).AlignItems(ui.Center).Radius(4).
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.StatusTools())
		switch {
		case toolsBtn.Pressed():
			toolsBtn.Background(tk.BgSelected)
		case toolsBtn.Hovered():
			toolsBtn.Background(tk.BgHover)
		}
		if toolsBtn.Clicked() {
			a.router.Push(a.activeWorkbenchRoute(routes.SectionTools))
		}
		toolsBtn.Children(func() {
			ui.Text(c, locales.StatusTools()).FontSize(11).TextColor(tk.TextSecondary).SingleLine()
		})

		ui.Text(c, fmt.Sprintf("v%s", config.Version)).FontSize(11).TextColor(tk.TextMuted).SingleLine()
	})

	a.errorLogDialog(c, logOpen)
}

// errorLogDialog 弹出系统与错误日志排查弹窗（统一使用 v2 Token）。
func (a *App) errorLogDialog(c *ui.Context, open *bool) {
	m, tk := thV2.Metrics, thV2.Of(c)
	errCount := logger.ErrorCount()
	logs := logger.Entries()

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		copied := ui.Local(panel, "copied", func() bool { return false })
		backdrop.Background(tk.Overlay)
		panel.Width(680).Height(500).Padding(m.Space4).Gap(m.Space3).
			Radius(m.RadiusLG).Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Shadow(0, 12, 32, 0, tk.Overlay)
		panel.Children(func() {
			// 1. 顶部标题栏与快捷操作
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
				ui.Icon(c, icons.SquareTerminal).FontSize(m.IconMD).TextColor(tk.Accent)
				ui.Text(c, locales.LogDialogTitle()).FontSize(m.FontMD).Bold().TextColor(tk.TextPrimary)
				if errCount > 0 {
					ui.Box(c).Padding(1, m.Space2).Radius(m.RadiusFull).
						Background(tk.DangerSoft).Children(func() {
						ui.Text(c, locales.LogErrorCount(errCount)).FontSize(m.FontXS).TextColor(tk.Danger)
					})
				}
				ui.Spacer(c)

				// 复制全部按钮
				copyBtn := ui.ButtonBase(c.Key(keyLogCopyAll)).
					Height(26).PaddingX(m.Space2).Gap(m.Space1).
					Radius(m.RadiusSM).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.CopyAll())
				if copyBtn.Hovered() {
					copyBtn.Background(tk.BgHover)
				}
				if copyBtn.Clicked() {
					c.Services().WriteClipboard(logger.FormatAll())
					*copied = true
				}
				copyBtn.Children(func() {
					ui.Icon(c, icons.Copy).FontSize(m.IconXS).TextColor(tk.TextSecondary)
					copyLabel := locales.CopyAll()
					if *copied {
						copyLabel = locales.Copied()
					}
					ui.Text(c, copyLabel).FontSize(m.FontXS).TextColor(tk.TextSecondary)
				})

				// 清空日志按钮
				clearBtn := ui.ButtonBase(c.Key(keyLogClearAll)).
					Height(26).PaddingX(m.Space2).Gap(m.Space1).
					Radius(m.RadiusSM).AlignItems(ui.Center).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.Clear())
				if clearBtn.Hovered() {
					clearBtn.Background(tk.BgHover)
				}
				if clearBtn.Clicked() {
					logger.Clear()
					*copied = false
				}
				clearBtn.Children(func() {
					ui.Icon(c, icons.Trash).FontSize(m.IconXS).TextColor(tk.TextMuted)
					ui.Text(c, locales.Clear()).FontSize(m.FontXS).TextColor(tk.TextMuted)
				})

				// 关闭按钮
				closeBtn := ui.ButtonBase(c.Key(keyLogCloseBtn)).
					Size(26, 26).Radius(m.RadiusSM).Center().
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.LogClose())
				if closeBtn.Hovered() {
					closeBtn.Background(tk.BgHover)
				}
				if closeBtn.Clicked() {
					*open = false
					*copied = false
				}
				closeBtn.Children(func() {
					ui.Icon(c, icons.X).FontSize(m.IconXS).TextColor(tk.TextMuted)
				})
			})

			// 2. 日志展示主区域
			ui.Box(c).Grow(1).Radius(m.RadiusMD).Background(tk.BgApp).
				Border(m.Border, tk.Border).Padding(m.Space2).Children(func() {
				if len(logs) == 0 {
					ui.Box(c).Fill().Center().Children(func() {
						ui.Column(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
							ui.Icon(c, icons.SquareTerminal).FontSize(36).TextColor(tk.TextMuted)
							ui.Text(c, locales.LogEmpty()).FontSize(m.FontSM).TextColor(tk.TextMuted)
						})
					})
					return
				}
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(m.Space1).Children(func() {
						for i, entry := range logs {
							ui.Row(c.Key(fmt.Sprintf("log-row-%d", i))).
								Gap(m.Space2).PaddingY(2).AlignItems(ui.Start).Children(func() {
								ui.Text(c, entry.Time.Format("15:04:05.000")).
									FontSize(m.FontXS).TextColor(tk.TextMuted)
								switch entry.Level {
								case logger.LevelError:
									ui.Text(c, "[ERROR]").FontSize(m.FontXS).Bold().TextColor(tk.Danger)
								case logger.LevelWarn:
									ui.Text(c, "[WARN]").FontSize(m.FontXS).Bold().TextColor(tk.Warning)
								default:
									ui.Text(c, "[INFO]").FontSize(m.FontXS).TextColor(tk.Accent)
								}
								ui.Text(c, entry.Message).FontSize(m.FontXS).
									TextColor(tk.TextPrimary).Grow(1)
							})
						}
					})
				})
			})

			// 3. 底部信息行与关闭按钮
			ui.Row(c).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.LogFooterSummary(len(logs))).
					FontSize(m.FontXS).TextColor(tk.TextMuted)
				ui.Spacer(c)
				if ui.Button(c, locales.Close()).Clicked() {
					*open = false
					*copied = false
				}
			})
		})
	})
}

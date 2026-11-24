package settings

import (
	"fmt"
	"runtime"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/config"
	"rddpostman/locales"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

type settingsNavItem struct {
	id    string
	label string
	icon  *ui.SVG
}

func getSettingsNavItems() []settingsNavItem {
	return []settingsNavItem{
		{id: "general", label: locales.SettingsNavGeneral(), icon: icons.Settings},
		{id: "themes", label: locales.SettingsNavThemes(), icon: icons.Sparkles},
		{id: "shortcuts", label: locales.SettingsNavShortcuts(), icon: icons.SquareTerminal},
		{id: "data", label: locales.SettingsNavData(), icon: icons.Download},
		{id: "about", label: locales.SettingsNavAbout(), icon: icons.Info},
	}
}

// Dialog 在 RootLayout 层绘制全局设置弹窗（对齐 Pixso 24-全局框架-SettingDialog）。
// 无论当前处于 HubLayout 还是 WorkbenchLayout，均可在不切换路由的情况下直接唤起。
func Dialog(c *ui.Context, open *bool, opts ...DialogOptions) {
	if open == nil {
		return
	}
	var opt DialogOptions
	if len(opts) > 0 {
		opt = opts[0]
	}
	if opt.ThemeMode == "" {
		opt.ThemeMode = ThemeSystem
	}
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		activeTab := ui.Local(panel, "settings-active-tab", func() string {
			if opt.InitialTab != "" {
				return opt.InitialTab
			}
			return "general"
		})
		if opt.InitialTab != "" {
			req := opt.InitialTab
			lastReq := ui.Local(panel, "settings-last-req-tab", func() string {
				return ""
			})
			if *lastReq != req {
				*activeTab = req
				*lastReq = req
			}
		}

		backdrop.Background(tk.Overlay)
		panel.Width(700).Height(500).Radius(m.RadiusLG).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Shadow(0, 12, 32, 0, tk.Overlay).
			Clip()

		panel.Children(func() {
			navItems := getSettingsNavItems()
			ui.Row(c).Fill().AlignItems(ui.Stretch).Children(func() {
				// 左侧分类导航栏
				ui.Column(c).Width(176).FillHeight().
					Background(tk.BgSubtle).
					BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).
					Padding(m.Space3, m.Space2).Gap(2).Children(func() {
					for _, item := range navItems {
						it := item
						selected := *activeTab == it.id
						btn := ui.ButtonBase(c.Key("settings-nav-"+it.id)).
							FillWidth().Height(32).Padding(0, 10).Gap(m.Space2).
							Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(it.label).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						switch {
						case selected:
							btn.Background(tk.BgSurface)
						case btn.Hovered():
							btn.Background(tk.BgHover)
						}
						if btn.Clicked() {
							*activeTab = it.id
						}
						btn.Children(func() {
							iconColor := tk.TextSecondary
							textColor := tk.TextSecondary
							weight := 400
							if selected {
								iconColor = tk.Accent
								textColor = tk.TextPrimary
								weight = 600
							}
							ui.Icon(c, it.icon).FontSize(m.IconSM).TextColor(iconColor)
							ui.Text(c, it.label).FontSize(m.FontSM).FontWeight(weight).
								TextColor(textColor).SingleLine()
						})
					}
				})

				// 右侧内容区
				ui.Column(c).Grow(1).FillHeight().Background(tk.BgSurface).Children(func() {
					// 顶部标题与关闭按钮
					title := locales.SettingsNavGeneral()
					for _, it := range navItems {
						if it.id == *activeTab {
							title = it.label
							break
						}
					}
					ui.Row(c).FillWidth().Height(52).Padding(0, m.Space5).
						AlignItems(ui.Center).Justify(ui.SpaceBetween).
						BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
						ui.Text(c, title).FontSize(m.FontXL).FontWeight(600).
							TextColor(tk.TextPrimary).SingleLine()

						closeBtn := ui.ButtonBase(c.Key("settings-dialog-close")).
							Size(28, 28).Radius(m.RadiusSM).Center().
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.SettingsClose())
						if closeBtn.Hovered() {
							closeBtn.Background(tk.BgHover)
						}
						if closeBtn.Clicked() {
							*open = false
						}
						closeBtn.Children(func() {
							ui.Icon(c, icons.X).FontSize(m.IconSM).TextColor(tk.TextSecondary)
						})
					})

					// 可滚动设置项区
					ui.Scroll(c).Grow(1).FillWidth().Children(func() {
						ui.Column(c).FillWidth().Padding(m.Space4, m.Space5).Gap(m.Space2).Children(func() {
							switch *activeTab {
							case "general":
								dialogLanguageRow(c)
								dialogSettingRow(c, locales.DataMode(), locales.DataModeDesc(), locales.OfflineMode())
								dialogSettingRow(c, locales.SettingsHttpVersion(), locales.SettingsHttpVersionDesc(), locales.SettingsAuto())
							case "themes":
								dialogThemeRow(c, opt.ThemeMode, opt.OnThemeMode)
								dialogAccentRow(c)
							case "shortcuts":
								dialogShortcutsList(c)
							case "data":
								dialogSettingRow(c, locales.DataMode(), locales.DataModeDesc(), locales.OfflineMode())
								storageVal := "JSON / Local FS"
								if opt.DataDir != "" {
									storageVal = opt.DataDir
								}
								dialogSettingRow(c, locales.SettingsStorage(), locales.SettingsStorageDesc(), storageVal)
							case "about":
								dialogSettingRow(c, locales.SettingsApp(), locales.SettingsAppDesc(), "rddPostman")
								dialogSettingRow(c, locales.SettingsVersion(), locales.SettingsVersionDesc(), fmt.Sprintf("v%s", config.Version))
							}
						})
					})
				})
			})
		})
	})
}

func dialogThemeRow(c *ui.Context, mode ThemeMode, onChange func(ThemeMode)) {
	m, tk := thV2.Metrics, thV2.Of(c)
	row := ui.Row(c).FillWidth().Padding(10, 0).Gap(m.Space3).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)

	choice := 0
	switch mode {
	case ThemeLight:
		choice = 1
	case ThemeDark:
		choice = 2
	}
	localChoice := ui.Local(row, "dialog-theme-choice", func() int { return choice })
	lastMode := ui.Local(row, "dialog-theme-last-mode", func() ThemeMode { return mode })
	if *lastMode != mode {
		*localChoice = choice
		*lastMode = mode
	}

	row.Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, locales.Appearance()).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
			ui.Text(c, locales.AppearanceDesc()).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		seg := ui.Segmented(c.Key("settings-dialog-theme"), localChoice,
			locales.FollowSystem(), locales.ThemeLight(), locales.ThemeDark())
		if seg.Changed() && onChange != nil {
			next := ThemeSystem
			switch *localChoice {
			case 1:
				next = ThemeLight
			case 2:
				next = ThemeDark
			}
			*lastMode = next
			onChange(next)
		}
	})
}

func dialogShortcutsList(c *ui.Context) {
	mod := "Ctrl"
	alt := "Alt"
	if runtime.GOOS == "darwin" {
		mod = "⌘"
		alt = "⌥"
	}
	join := func(parts ...string) string {
		if runtime.GOOS == "darwin" {
			out := ""
			for _, p := range parts {
				out += p
			}
			return out
		}
		out := ""
		for i, p := range parts {
			if i > 0 {
				out += "+"
			}
			out += p
		}
		return out
	}

	items := []struct {
		name, desc, keys string
	}{
		{locales.GlobalSearch(), locales.ScGlobalSearchDesc(), join(mod, "K")},
		{locales.ShowAllCommands(), locales.ScShowCommandsDesc(), join(mod, "Shift", "P")},
		{locales.SettingsTitle(), locales.ScSettingsDesc(), join(mod, ",")},
		{locales.NewRequest(), locales.ScNewReqDesc(), join(mod, "N")},
		{locales.NewCollection(), locales.ScNewCollDesc(), join(mod, "Shift", "N") + " / " + join(mod, "B")},
		{locales.ScImport(), locales.ScImportDesc(), join(mod, "O")},
		{locales.Globals(), locales.ScGlobalsDesc(), join(mod, "Shift", "G")},
		{locales.ScSaveRequest(), locales.ScSaveRequestDesc(), join(mod, "S")},
		{locales.ScSendRequest(), locales.ScSendRequestDesc(), join(mod, "Enter")},
		{locales.ScFocusURL(), locales.ScFocusURLDesc(), join(mod, "L")},
		{locales.ScCloseTab(), locales.ScCloseTabDesc(), join(mod, "W")},
		{locales.ScCycleTabs(), locales.ScCycleTabsDesc(), "Ctrl+Tab / Ctrl+Shift+Tab"},
		{locales.ScSwitchSidebar(), locales.ScSwitchSidebarDesc(), join(mod, "1") + " / 2 / 3 / 4"},
		{locales.SwitchEnvironment(), locales.ScSwitchEnvDesc(), join(mod, "E")},
		{locales.ScNavHistory(), locales.ScNavHistoryDesc(), alt + "+← / " + alt + "+→"},
		{locales.ScCloseModal(), locales.ScCloseModalDesc(), "Esc"},
	}

	for _, it := range items {
		dialogSettingRow(c, it.name, it.desc, it.keys)
	}
}

func dialogSettingRow(c *ui.Context, name, desc, value string) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Padding(10, 0).Gap(m.Space3).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, name).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
			ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		ui.Box(c).Padding(4, 10).Radius(m.RadiusSM).Background(tk.BgSubtle).Children(func() {
			ui.Text(c, value).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary).SingleLine()
		})
	})
}

func dialogAccentRow(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Padding(10, 0).Gap(m.Space3).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, locales.SettingsAccentColor()).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
			ui.Text(c, locales.SettingsAccentColorDesc()).
				FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
			ui.Box(c).Size(14, 14).Radius(m.RadiusFull).Background(tk.Accent)
			ui.Text(c, "#336FFF").FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary).SingleLine()
		})
	})
}

func dialogLanguageRow(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	row := ui.Row(c).FillWidth().Padding(10, 0).Gap(m.Space3).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
	choice := ui.Local(row, "dialog-language", func() int {
		switch c.Locale() {
		case "zh-Hans":
			return 1
		case "en":
			return 2
		default:
			return 0
		}
	})
	row.Children(func() {
		ui.Column(c).Grow(1).Gap(2).Children(func() {
			ui.Text(c, locales.Language()).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
			ui.Text(c, locales.LanguageDesc()).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		seg := ui.Segmented(c.Key("settings-dialog-language"), choice,
			locales.SystemLanguage(), locales.SimplifiedChinese(), locales.English())
		if seg.Changed() {
			switch *choice {
			case 1:
				mygo.App.SetLocale("zh-Hans")
			case 2:
				mygo.App.SetLocale("en")
			default:
				mygo.App.SetLocale("")
			}
		}
	})
}

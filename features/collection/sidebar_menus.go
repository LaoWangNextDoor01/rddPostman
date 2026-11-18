package collection

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// addMenuButton 是侧栏工具栏里的「新建」按钮与下拉菜单。
func addMenuButton(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	addBtn := v1.MiniButton(c, v1.RdButtonOptions{
		Key:      "add",
		Label:    locales.Add(),
		Icon:     icons.Get("plus"),
		Size:     m.SizeLG,
		IconSize: m.IconMD,
		Radius:   m.RadiusMD,
	})
	if addBtn.Clicked() {
		props.state.addOpen = !props.state.addOpen
	}
	ui.PopoverBase(c, addBtn, &props.state.addOpen, func(panel ui.Element) {
		panel.Margin(m.Space1, 0, 0, 0).Padding(m.Space1).Width(160).
			Gap(m.Space1)
		theme.PopoverPanel(c, panel)
		item := func(key, icon, label string, fn func()) {
			row := ui.ButtonBase(c.Key("add-" + key)).FillWidth().
				Height(26).PaddingX(m.Space2).Gap(m.Space2).
				AlignItems(ui.Center).Radius(m.RadiusSM)
			if row.Hovered() {
				row.Background(tk.PopoverItemHover)
			}
			if row.Clicked() {
				fn()
				props.state.addOpen = false
			}
			row.Children(func() {
				ui.Icon(c, icons.Get(icon)).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
				ui.Text(c, label).FontSize(m.FontSM).
					TextColor(tk.TextSecondary).Grow(1)
			})
		}
		item("collection", "library", locales.NewCollection(), func() {
			props.state.newCollOpen = true
			props.state.newCollName = ""
			props.state.newCollDesc = ""
			props.state.newCollErr = ""
		})
		if len(props.collections) > 0 {
			item("folder", "folder-plus", locales.NewFolder(), func() {
				props.actions.createNode(domain.ID(props.state.selected), domain.KindFolder)
			})
			item("request", "send", locales.NewRequest(), func() {
				props.actions.createNode(domain.ID(props.state.selected), domain.KindRequest)
			})
		}

		ui.Divider(c)

		item("import-curl", "code", "从 cURL 导入", func() {
			props.state.importCurlOpen = true
			props.state.importCurlText = ""
			props.state.importCurlErr = ""
		})
		item("import-postman", "file", "导入 Postman 集合", func() {
			props.state.importPostmanOpen = true
			props.state.importPostmanText = ""
			props.state.importPostmanErr = ""
		})

		ui.Divider(c)

		item("environment", "globe", locales.RailEnvironments(), func() {
			props.actions.navigate(routes.Environments)
		})
		item("docs", "book-open", locales.DocsTitle(), func() {
			props.actions.navigate(routes.Docs)
		})
	})
}

// environmentMenuButton 是侧栏工具栏里的环境切换按钮与弹层。
func environmentMenuButton(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	envBtn := v1.MiniButton(c, v1.RdButtonOptions{
		Key:      "env",
		Label:    locales.SwitchEnvironment(),
		Tip:      locales.SwitchEnvironment(),
		Icon:     icons.Get("globe"),
		Size:     m.SizeLG,
		IconSize: m.IconMD,
		Radius:   m.RadiusMD,
	})
	envOpen := ui.Local(envBtn, "env-open", func() bool { return false })
	if envBtn.Clicked() {
		*envOpen = !*envOpen
	}
	ui.PopoverBase(c, envBtn, envOpen, func(panel ui.Element) {
		panel.Width(280).MaxHeight(360).Padding(m.Space2)
		theme.PopoverPanel(c, panel)
		panel.Children(func() {
			ui.Text(c, locales.SwitchEnvironment()).FontSize(m.FontSM).
				TextColor(tk.TextMuted).Padding(m.Space1, m.Space2)

			if len(props.environments) == 0 {
				ui.Text(c, locales.NoEnvironments()).FontSize(m.FontXS).
					TextColor(tk.TextMuted).Padding(m.Space3, m.Space2)
			} else {
				ui.Column(c).Gap(m.Space1).Children(func() {
					row := func(key, name, meta, icon string, active, muted bool, choose func()) {
						btn := ui.ButtonBase(c.Key(key)).FillWidth().
							Height(44).PaddingX(m.Space2).Gap(m.Space2).
							AlignItems(ui.Center).Radius(m.RadiusMD)
						if active || btn.Hovered() {
							btn.Background(tk.PopoverItemHover)
						}
						if btn.Clicked() {
							choose()
							*envOpen = false
						}
						btn.Children(func() {
							iconBg, iconColor := tk.AccentSoft, tk.Accent
							if muted {
								iconBg, iconColor = tk.BgElevated, tk.TextMuted
							}
							ui.Box(c).Size(24, 24).Radius(m.RadiusMD).
								Background(iconBg).Center().Children(func() {
								ui.Icon(c, icons.Get(icon)).FontSize(m.IconXS).
									TextColor(iconColor)
							})
							ui.Column(c).Grow(1).Gap(1).Children(func() {
								ui.Text(c, name).FontSize(m.FontSM).
									TextColor(tk.TextPrimary).SingleLine()
								if meta != "" {
									ui.Text(c, meta).FontSize(m.FontXS).
										TextColor(tk.TextMuted).SingleLine()
								}
							})
							if active {
								ui.Text(c, "✓").FontSize(m.FontMD).
									TextColor(tk.Accent)
							}
						})
					}
					row("env-none", locales.NoEnvironment(), "", "x",
						props.selectedEnv == "", true, func() {
							props.actions.selectEnvironment("")
						})
					for _, env := range props.environments {
						env := env
						row("env-"+string(env.ID), env.Name,
							locales.EnvironmentVariableCount(len(env.Vars)),
							"globe", env.ID == props.selectedEnv, false, func() {
								props.actions.selectEnvironment(env.ID)
							})
					}
				})
			}

			ui.Divider(c).Margin(m.Space2, 0)

			manage := ui.ButtonBase(c.Key("env-manage")).FillWidth().
				Height(28).PaddingX(m.Space2).AlignItems(ui.Center).
				Radius(m.RadiusSM)
			if manage.Hovered() {
				manage.Background(tk.PopoverItemHover)
			}
			if manage.Clicked() {
				*envOpen = false
				props.actions.navigate(routes.Environments)
			}
			manage.Children(func() {
				ui.Text(c, locales.ManageEnvironmentsAction()).
					FontSize(m.FontSM).TextColor(tk.TextSecondary).Grow(1)
			})
		})
	})
}

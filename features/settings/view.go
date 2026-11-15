// Package settings 实现设置中心。
package settings

import (
	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/locales"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// View 绘制设置页。设置不需要工作区侧栏，占满主区域。
func View(c *ui.Context) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c)
	})
}

func content(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	v1.PageHeader(c, locales.SettingsTitle(), locales.SettingsSubtitle())

	ui.Column(c).Margin(0, m.PagePaddingX).Background(tk.BgSurface).
		Border(m.Border, tk.Border).Radius(m.RadiusMD).Children(func() {
		row(c, locales.Appearance(), locales.AppearanceDesc(), locales.FollowSystem())
		row(c, locales.DataMode(), locales.DataModeDesc(), locales.OfflineMode())
		row(c, locales.Shortcuts(), locales.ShortcutsDesc(), "⌘K")
		languageRow(c)
	})
}

func row(c *ui.Context, name, desc, value string) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Gap(m.Space3).Padding(m.Space3, m.Space4).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, name).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
			ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		ui.Text(c, value).FontSize(m.FontXS).TextColor(tk.TextSecondary)
	})
}

// languageRow 让用户选择跟随系统、简体中文或 English。
func languageRow(c *ui.Context) {
	m := theme.Metrics
	tk := theme.Of(c)
	row := ui.Row(c).Gap(m.Space3).Padding(m.Space3, m.Space4).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
	choice := ui.Local(row, "language", func() int {
		switch mygo.App.ChosenLocale() {
		case "zh-Hans":
			return 1
		case "en":
			return 2
		default:
			return 0
		}
	})
	row.Children(func() {
		ui.Column(c).Gap(1).Grow(1).Children(func() {
			ui.Text(c, locales.Language()).FontSize(m.FontSM).Bold().TextColor(tk.TextPrimary)
			ui.Text(c, locales.LanguageDesc()).FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
		seg := ui.Segmented(c.Key("language-select"), choice,
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

package v2

import (
	themv2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// ConfirmDialogOpts 配置统一的 v2 二次确认对话框（用于删除集合/目录/请求/环境、清空历史等危险操作）。
type ConfirmDialogOpts struct {
	Key          string
	Title        string
	Description  string
	ConfirmLabel string
	CancelLabel  string
	Danger       bool
	OnConfirm    func()
	OnCancel     func()
}

// ConfirmDialog 渲染居中模态二次确认框；点击取消、遮罩或按 Esc 自动关闭，点击确认后执行 OnConfirm 并关闭。
func ConfirmDialog(c *ui.Context, open *bool, opts ConfirmDialogOpts) {
	if open == nil {
		return
	}
	m, tk := themv2.Metrics, themv2.Of(c)

	key := opts.Key
	if key == "" {
		key = "confirm-dialog"
	}
	confirmLbl := opts.ConfirmLabel
	if confirmLbl == "" {
		confirmLbl = locales.ConfirmDelete()
	}
	cancelLbl := opts.CancelLabel
	if cancelLbl == "" {
		cancelLbl = locales.Cancel()
	}

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(400).Padding(m.Space5).Gap(m.Space3).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Radius(m.RadiusLG).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.18))

		panel.Children(func() {
			// 标题行：左侧语义徽标 + 标题
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				badgeBg := tk.AccentSoft
				badgeBorder := tk.Accent.Alpha(0.28)
				ic := icons.Info
				icColor := tk.Accent
				if opts.Danger {
					badgeBg = tk.DangerSoft
					badgeBorder = tk.Danger.Alpha(0.32)
					ic = icons.TriangleAlert
					icColor = tk.Danger
				}
				ui.Box(c).Size(30, 30).Radius(m.RadiusMD).
					Background(badgeBg).Border(m.Border, badgeBorder).
					Center().Children(func() {
					ui.Icon(c, ic).FontSize(15).TextColor(icColor)
				})
				ui.Text(c, opts.Title).FontSize(m.FontMD).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine().Grow(1)
			})

			// 说明正文
			if opts.Description != "" {
				ui.Text(c, opts.Description).FontSize(m.FontSM).FixedLineHeight(20).
					TextColor(tk.TextSecondary).FillWidth()
			}

			// 底部动作区
			ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space1, 0, 0, 0).Children(func() {
				cancelBtn := ui.ButtonBase(c.Key(key+"-cancel")).
					Height(m.ButtonLarge).PaddingX(m.Space4).Center().
					Radius(m.RadiusSM).Cursor(ui.CursorPointer).
					Role(ui.RoleButton).Label(cancelLbl).
					Transition(ui.ElementTransition{Colors: true, Duration: themv2.Motion.Fast})
				switch {
				case cancelBtn.Pressed():
					cancelBtn.Background(tk.BgSelected).Border(m.Border, tk.BorderStrong)
				case cancelBtn.Hovered():
					cancelBtn.Background(tk.BgHover).Border(m.Border, tk.BorderStrong)
				default:
					cancelBtn.Background(tk.BgSurface).Border(m.Border, tk.BorderPopover)
				}
				if cancelBtn.Clicked() {
					*open = false
					if opts.OnCancel != nil {
						opts.OnCancel()
					}
				}
				cancelBtn.Children(func() {
					ui.Text(c, cancelLbl).FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()
				})

				confirmBtn := ui.ButtonBase(c.Key(key+"-confirm")).
					Height(m.ButtonLarge).PaddingX(m.Space4).Center().
					Radius(m.RadiusSM).Cursor(ui.CursorPointer).
					Role(ui.RoleButton).Label(confirmLbl).
					Transition(ui.ElementTransition{Colors: true, Duration: themv2.Motion.Fast})
				if opts.Danger {
					switch {
					case confirmBtn.Pressed():
						confirmBtn.Background(tk.Danger.Alpha(0.78))
					case confirmBtn.Hovered():
						confirmBtn.Background(tk.Danger.Alpha(0.88))
					default:
						confirmBtn.Background(tk.Danger)
					}
				} else {
					switch {
					case confirmBtn.Pressed():
						confirmBtn.Background(tk.AccentPressed)
					case confirmBtn.Hovered():
						confirmBtn.Background(tk.AccentHover)
					default:
						confirmBtn.Background(tk.Accent)
					}
				}
				if confirmBtn.Clicked() {
					*open = false
					if opts.OnConfirm != nil {
						opts.OnConfirm()
					}
				}
				confirmBtn.Children(func() {
					ui.Text(c, confirmLbl).FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.OnAccent).SingleLine()
				})
			})
		})
	})
}

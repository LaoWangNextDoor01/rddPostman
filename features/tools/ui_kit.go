package tools

import (
	"fmt"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// formatByteSize 将字节数按阈值自动升档为 B / KB / MB / GB，避免大文本时字节数过长。
func formatByteSize(n int) string {
	if n < 0 {
		n = 0
	}
	const (
		kb = 1024
		mb = 1024 * kb
		gb = 1024 * mb
	)
	switch {
	case n < kb:
		return fmt.Sprintf("%d B", n)
	case n < mb:
		return fmt.Sprintf("%.2f KB", float64(n)/float64(kb))
	case n < gb:
		return fmt.Sprintf("%.2f MB", float64(n)/float64(mb))
	default:
		return fmt.Sprintf("%.2f GB", float64(n)/float64(gb))
	}
}

type btnVariant int

const (
	btnSecondary btnVariant = iota
	btnPrimary
	btnGhost
	btnActive
	btnSuccess
	btnDanger
)

type pillVariant int

const (
	pillNeutral pillVariant = iota
	pillAccent
	pillPurple
	pillSuccess
	pillDanger
	pillWarning
)

type toolBtnOpts struct {
	Key     string
	Label   string
	Icon    *ui.SVG
	Variant btnVariant
	Height  float32
	PadX    float32
	Chevron bool
	Disabled bool
}

// toolHeader 绘制所有小工具统一的 48px 顶部标题条（对齐 Pixso 设计稿）。
func toolHeader(c *ui.Context, ic *ui.SVG, title, subtitle string, right func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Height(48).Padding(0, m.Space5).Gap(10).
		AlignItems(ui.Center).Background(tk.BgBase).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Box(c).Size(26, 26).Radius(m.RadiusMD).
			Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.28)).
			Center().Children(func() {
			ui.Icon(c, ic).FontSize(14).TextColor(tk.Accent)
		})
		ui.Text(c, title).FontSize(m.FontMD).FontWeight(600).
			TextColor(tk.TextPrimary).SingleLine()
		ui.Text(c, subtitle).FontSize(m.FontXS).
			TextColor(tk.TextMuted).SingleLine().Grow(1)
		if right != nil {
			ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(right)
		}
	})
}

// toolToolbar 绘制 44px 操作控制条（对齐 Pixso 设计稿）。
func toolToolbar(c *ui.Context, left, right func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Height(44).Padding(0, m.Space5).Gap(m.Space2).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Background(tk.BgApp).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(left)
		if right != nil {
			ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(right)
		}
	})
}

// toolDividerV 绘制工具条内的 1×18 垂直细分割线。
func toolDividerV(c *ui.Context) {
	tk := thV2.Of(c)
	ui.Box(c).Size(1, 18).Background(tk.Border)
}

// toolPaneHeader 绘制双栏编辑器的 34px 栏头（INPUT / OUTPUT + 统计信息 + 状态胶囊）。
func toolPaneHeader(c *ui.Context, title, meta string, right func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Height(34).Padding(0, m.Space4).Gap(m.Space2).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Background(tk.BgApp).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, title).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary).SingleLine()
			if meta != "" {
				ui.Text(c, meta).Font("monospace").FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
			}
		})
		if right != nil {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(right)
		}
	})
}

// toolSegmented 绘制 28px 高的分段选择器（轨道 BgSubtle，激活项 BgSurface + 1px Border）。
// 所有项统一保留 1px 透明/实色边框占位与 500 字重，彻底消除切换时的 1~2px 排版抖动（Layout Shift）。
func toolSegmented(c *ui.Context, keyPrefix string, items []string, activeIdx int, onSelect func(idx int)) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c.Key(keyPrefix)).Height(28).Padding(2).Gap(2).
		AlignItems(ui.Center).Radius(m.RadiusMD).Background(tk.BgSubtle).Children(func() {
		for i, lbl := range items {
			idx, label := i, lbl
			on := idx == activeIdx
			seg := ui.ButtonBase(c.Key(keyPrefix+"-"+label)).
				Height(24).PaddingX(9).Radius(m.RadiusSM).Center().
				Cursor(ui.CursorPointer).Role(ui.RoleTab).Label(label).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case on:
				seg.Background(tk.BgSurface).Border(m.Border, tk.Border).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
			case seg.Pressed():
				seg.Background(tk.BgSelected).Border(m.Border, ui.Transparent)
			case seg.Hovered():
				seg.Background(tk.BgHover).Border(m.Border, ui.Transparent)
			default:
				seg.Background(ui.Transparent).Border(m.Border, ui.Transparent)
			}
			if seg.Clicked() && onSelect != nil {
				onSelect(idx)
			}
			seg.Children(func() {
				fg := tk.TextSecondary
				if on {
					fg = tk.TextPrimary
				}
				ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
			})
		}
	})
}

// toolBtn 绘制统一风格的动作按钮（Primary / Secondary / Ghost / Active / Success / Danger）。
func toolBtn(c *ui.Context, o toolBtnOpts) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	h := o.Height
	if h <= 0 {
		h = m.ButtonHeight
	}
	px := o.PadX
	if px <= 0 {
		px = 10
	}
	key := o.Key
	if key == "" {
		key = o.Label
	}

	b := ui.ButtonBase(c.Key(key)).Height(h).PaddingX(px).Gap(6).
		Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(o.Label).
		Disabled(o.Disabled).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	var bg, border, fg ui.Color
	weight := 500
	border = ui.Transparent

	switch o.Variant {
	case btnPrimary:
		bg, fg = tk.Accent, tk.OnAccent
		if b.Pressed() {
			bg = tk.AccentPressed
		} else if b.Hovered() {
			bg = tk.AccentHover
		}
	case btnActive:
		bg, border, fg = tk.AccentSoft, tk.Accent.Alpha(0.35), tk.Accent
	case btnSuccess:
		bg, border, fg = tk.Success.Alpha(0.14), tk.Success.Alpha(0.38), tk.Success
	case btnDanger:
		bg, border, fg = tk.DangerSoft, tk.Danger.Alpha(0.38), tk.Danger
	case btnGhost:
		bg, fg = ui.Transparent, tk.TextSecondary
		if b.Pressed() {
			bg, fg = tk.BgSelected, tk.TextPrimary
		} else if b.Hovered() {
			bg, fg = tk.BgHover, tk.TextPrimary
		}
	default: // btnSecondary
		bg, border, fg = tk.BgSurface, tk.Border, tk.TextPrimary
		if b.Pressed() {
			bg = tk.BgSelected
		} else if b.Hovered() {
			bg, border = tk.BgHover, tk.BorderStrong
		}
	}

	b.Background(bg).Border(m.Border, border)
	if o.Disabled {
		b.Opacity(0.45)
	}

	b.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).FontSize(13).TextColor(fg)
		}
		if o.Label != "" {
			ui.Text(c, o.Label).FontSize(m.FontSM).FontWeight(weight).TextColor(fg).SingleLine()
		}
		if o.Chevron {
			ui.Icon(c, icons.ChevronDown).FontSize(11).TextColor(fg)
		}
	})
	return b
}

// toolPill 绘制 20px 高的状态胶囊（可选左侧圆点）。
func toolPill(c *ui.Context, label string, variant pillVariant, withDot bool) {
	m, tk := thV2.Metrics, thV2.Of(c)
	var bg, fg, border ui.Color
	hasBorder := true
	switch variant {
	case pillAccent:
		bg, fg, border = tk.AccentSoft, tk.Accent, tk.Accent.Alpha(0.32)
	case pillPurple:
		bg, fg, border = tk.MethodPatch.Alpha(0.14), tk.MethodPatch, tk.MethodPatch.Alpha(0.32)
	case pillSuccess:
		bg, fg, border = tk.Success.Alpha(0.14), tk.Success, tk.Success.Alpha(0.35)
	case pillDanger:
		bg, fg, border = tk.DangerSoft, tk.Danger, tk.Danger.Alpha(0.35)
	case pillWarning:
		bg, fg, border = tk.Warning.Alpha(0.14), tk.Warning, tk.Warning.Alpha(0.35)
	default:
		bg, fg, hasBorder = tk.BgSubtle, tk.TextSecondary, false
	}

	p := ui.Row(c).Height(20).PaddingX(7).Gap(5).AlignItems(ui.Center).
		Radius(m.RadiusFull).Background(bg)
	if hasBorder {
		p.Border(m.Border, border)
	}
	p.Children(func() {
		if withDot {
			ui.Box(c).Size(6, 6).Radius(3).Background(fg)
		}
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
	})
}

// toolInlineErrorCard 绘制错误诊断卡片（红底红框 + 标题/说明 + 右侧快捷修复按钮）。
func toolInlineErrorCard(c *ui.Context, key, title, desc, actionLabel string, onAction func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c.Key(key)).FillWidth().MinHeight(52).Padding(10, 12).Gap(10).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Radius(m.RadiusMD).Background(tk.DangerSoft).
		Border(m.Border, tk.Danger.Alpha(0.35)).Children(func() {
		ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.TriangleAlert).FontSize(15).TextColor(tk.Danger)
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Text(c, title).FontSize(m.FontSM).FontWeight(600).TextColor(tk.Danger).SingleLine()
				if desc != "" {
					ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextSecondary)
				}
			})
		})
		if actionLabel != "" && onAction != nil {
			if toolBtn(c, toolBtnOpts{Key: key + "-act", Label: actionLabel, Variant: btnDanger, Height: 24, PadX: 8}).Clicked() {
				onAction()
			}
		}
	})
}

// toolCard 绘制标准 1px 细边框卡片容器（含顶部标题栏与内容区）。
func toolCard(c *ui.Context, key string, borderColor ui.Color, headerBg ui.Color, headerLeft, headerRight, body func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	if borderColor == (ui.Color{}) {
		borderColor = tk.Border
	}
	if headerBg == (ui.Color{}) {
		headerBg = tk.BgApp
	}
	ui.Column(c.Key(key)).FillWidth().Radius(m.RadiusMD).
		Background(tk.BgBase).Border(m.Border, borderColor).Clip().Children(func() {
		ui.Row(c).FillWidth().Height(32).Padding(0, 14).Gap(8).
			AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Background(headerBg).
			BorderWidth(0, 0, m.Border, 0).BorderColor(borderColor).Children(func() {
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(headerLeft)
			if headerRight != nil {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(headerRight)
			}
		})
		body()
	})
}

// toolCheckboxChip 绘制复选框胶囊（用于随机串字符集选择）。
func toolCheckboxChip(c *ui.Context, key, label string, checked bool, onToggle func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(26).PaddingX(8).Gap(6).
		Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleCheckBox).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	if checked {
		b.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.35))
	} else {
		b.Background(tk.BgSurface).Border(m.Border, tk.Border)
		if b.Hovered() {
			b.Background(tk.BgHover)
		}
	}
	if b.Clicked() && onToggle != nil {
		onToggle()
	}
	b.Children(func() {
		sq := ui.Box(c).Size(14, 14).Radius(3).Center()
		if checked {
			sq.Background(tk.Accent)
		} else {
			sq.Background(tk.BgSurface).Border(m.Border, tk.BorderStrong)
		}
		sq.Children(func() {
			if checked {
				ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.OnAccent)
			}
		})
		fg := tk.TextSecondary
		if checked {
			fg = tk.Accent
		}
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
	})
}

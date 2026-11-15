package v1

import (
	"rddpostman/components/theme"

	"github.com/egoist/mygo/ui"
)

type RdButtonOptions struct {
	Key   any
	Icon  *ui.SVG // 可选
	Text  string  // 可见文字，可选
	Label string  // 无障碍名称；为空时用 Text
	Tip   string  // tooltip

	Selected bool
	Disabled bool
	Role     ui.Role // 默认 RoleButton，可传 RoleTab 等

	Size     float32 // 高度，0 = 28
	Width    float32 // 宽度，0 = 自适应
	PaddingX float32 // 文字左右留白，0 = 8
	Gap      float32 // 图标与文字间距，0 = 4
	IconSize float32 // 图标大小，0 = 14
	Radius   float32 // 图标 radius 默认
}

func NavButton(c *ui.Context, icon *ui.SVG, label string) {

}

// mini icon button
func RdButton(c *ui.Context, o RdButtonOptions) ui.Element {
	m := theme.Metrics
	tk := theme.Of(c)

	size := o.Size
	if size == 0 {
		size = m.SizeLG
	}
	padX := o.PaddingX
	if padX == 0 {
		padX = m.Space2
	}
	gap := o.Gap
	if gap == 0 {
		gap = m.Space1
	}
	iconSize := o.IconSize
	if iconSize == 0 {
		iconSize = m.IconMD
	}
	label := o.Label
	if label == "" {
		label = o.Text
	}

	cc := c
	if o.Key != nil {
		cc = c.Key(o.Key)
	}

	btn := ui.ButtonBase(cc).Height(size).
		Justify(ui.Center).AlignItems(ui.Center).
		Gap(gap).Label(label).Description(o.Tip).
		Role(ui.RoleButton).Disabled(o.Disabled).
		Radius(o.Radius)

	switch {
	case o.Width > 0:
		btn.Width(o.Width)
	case o.Icon != nil && o.Text == "":
		btn.Width(size) // 只有图标：正方形
	default:
		btn.PaddingX(padX) // 有文字：左右留白，宽度自适应
	}

	switch {
	case o.Selected:
		btn.Background(tk.AccentSoft).TextColor(tk.Accent)
	case btn.Pressed():
		btn.Background(tk.HoverSurface.Mix(tk.TextPrimary, 0.08)).TextColor(tk.TextPrimary)
	case btn.Hovered():
		btn.Background(tk.HoverSurface).TextColor(tk.TextPrimary)
	default:
		btn.TextColor(tk.TextSecondary)
	}

	btn.Children(func() {
		if o.Icon != nil {
			ui.Icon(c, o.Icon).FontSize(iconSize)
		}
		if o.Text != "" {
			ui.Text(c, o.Text).FontSize(m.FontSM).SingleLine()
		}
	})
	if o.Tip != "" {
		btn.Tooltip(o.Tip)
	}
	return btn
}

// MiniButton 使用紧凑尺寸，其余选项全部透传。
func MiniButton(c *ui.Context, o RdButtonOptions) ui.Element {
	m := theme.Metrics
	if o.Size == 0 {
		o.Size = m.SizeMD // 24
	}
	if o.IconSize == 0 {
		o.IconSize = m.IconSM // 14
	}
	if o.PaddingX == 0 {
		o.PaddingX = m.Space1 // 4
	}
	if o.Radius == 0 {
		o.Radius = m.RadiusLG // 4
	}
	return RdButton(c, o)
}

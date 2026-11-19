// Package v2 是 UI v2 的原子组件层。颜色与尺寸一律来自
// components/theme/v2，组件内不允许出现 ui.Hex 或裸数字色值。
//
// 组件风格约定（与手写页面保持一致）：
//   - 可点击控件用 ui.ButtonBase（自带 Role/Label/键盘激活），不要用裸 Box；
//   - Label 同时是无障碍名和测试的点击名，必须传；
//   - hover/pressed 的职责写在组件里，调用方只关心 Clicked。
package v2

import (
	themv2 "rddpostman/components/theme/v2"

	"github.com/egoist/mygo/ui"
)

// IconButtonOpts 是 IconButton 的参数。
type IconButtonOpts struct {
	// Key 是控件的稳定标识（状态与测试复用）；留空则用 Label。
	Key string
	// Label 是无障碍名，也是测试的点击名，必传。
	Label string
	// Icon 是要显示的图标（components/v1/icons 的类型化变量）。
	Icon *ui.SVG
	// Tooltip 留空则不显示；建议与 Label 一致或更详细。
	Tooltip string
	// Radius 默认 RadiusSM（方形图标钮）；圆形头像按钮传 RadiusFull。
	Radius float32
	// Cursor 指针悬停时的形状：零值默认 CursorPointer（手型）；
	// 传其他 ui.Cursor 值（如 CursorNotAllowed）原样生效。
	Cursor ui.Cursor
}

// IconButton 画一个 24×24 的纯图标按钮（标题栏、工具条通用）：
// 默认次要色，hover 浅底、按下再深一档；返回元素供调用方检查 Clicked：
//
//	if v2.IconButton(c, v2.IconButtonOpts{Label: "通知", Icon: icons.Bell}).Clicked() {
//	    // 打开通知中心
//	}
func IconButton(c *ui.Context, o IconButtonOpts) ui.Element {
	m, tk := themv2.Metrics, themv2.Of(c)

	label := o.Label
	key := o.Key
	if key == "" {
		key = label
	}
	radius := o.Radius
	if radius == 0 {
		radius = m.RadiusSM
	}
	cursor := o.Cursor
	if cursor == 0 {
		cursor = ui.CursorPointer
	}

	b := ui.ButtonBase(c.Key(key)).Size(m.SizeMD, m.SizeMD).
		Justify(ui.Center).AlignItems(ui.Center).
		Radius(radius).Label(label).Role(ui.RoleButton).
		Cursor(cursor).
		Transition(ui.ElementTransition{Colors: true, Duration: themv2.Motion.Base}).
		TextColor(tk.TextSecondary)
	switch {
	case b.Pressed():
		b.Background(tk.BgSelected).TextColor(tk.TextPrimary)
	case b.Hovered():
		b.Background(tk.BgHover).TextColor(tk.TextPrimary)
	}

	b.Children(func() {
		ui.Icon(c, o.Icon).FontSize(m.IconMD)
	})

	if o.Tooltip != "" {
		ui.TooltipBase(c, b, func(tip ui.Element) {
			tip.AttachTo(b, ui.AnchorBottom, ui.AnchorTop).
				Margin(m.Space1, 0, 0, 0).
				Padding(m.Space1, m.Space2).
				Radius(m.RadiusMD).
				Background(tk.BgSurface).Border(m.Border, tk.Border)
			tip.Children(func() {
				ui.Text(c, o.Tooltip).FontSize(m.FontXS).
					TextColor(tk.TextSecondary).SingleLine()
			})
		})
	}
	return b
}

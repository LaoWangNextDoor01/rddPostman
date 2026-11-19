package v2

import (
	themv2 "rddpostman/components/theme/v2"

	"github.com/egoist/mygo/ui"
)

// PopoverOpts 是 Popover 的可选参数。
type PopoverOpts struct {
	// Width 是浮层宽度；0 = 自适应内容宽（设计稿 workspace 下拉为 330）。
	Width float32
	// Height 是浮层高度；0 = 自适应内容高。两者都为 0 时面板完全跟随内容；
	// 固定高度后，内容超长可在 build 里用 ui.Scroll(panel).Fill() 包一层滚动。
	Height float32
	// MinHeight 是浮层最小高度；0 = 不限。
	MinHeight float32
}

// Popover 按设计稿封装的浮层容器（02 §6.5 + workspace 下拉实测规格）：
// 白底 BgSurface、1px BorderPopover、圆角 RadiusLG、双层阴影
// （0 1 2 黑 8% 与 0 12 28 黑 14%），默认锚定在 anchor 下方 8px，
// 下方空间不足自动翻转到 anchor 上方；点击面板与 anchor 之外或按 Esc
// 时 *open 自动置 false。
//
// open 是跨帧状态，用 ui.Local 挂在触发元素上（不要用普通局部变量，
// 每帧重建会丢失）：
//
//	btn := ui.ButtonBase(c.Key(key)).Label("工作区")...
//	open := ui.Local(btn, "open", func() bool { return false })
//	if btn.Clicked() {
//	    *open = !*open
//	}
//	v2.Popover(c, btn, open, v2.PopoverOpts{Width: 330}, func(panel ui.Element) {
//	    // 在 panel 里构建内容；行间分割线用 v2.PopoverDivider(c)
//	})
func Popover(c *ui.Context, anchor ui.Element, open *bool, o PopoverOpts, build func(panel ui.Element)) ui.Element {
	m, tk := themv2.Metrics, themv2.Of(c)
	return ui.PopoverBase(c, anchor, open, func(panel ui.Element) {
		panel.Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Radius(m.RadiusLG).
			Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08)).
			Shadow(0, 12, 28, 0, ui.RGBA(0, 0, 0, 0.14)).
			Margin(m.Space2, 0, 0, 0)
		if o.Width > 0 {
			panel.Width(o.Width)
		}
		if o.Height > 0 {
			panel.Height(o.Height)
		}
		if o.MinHeight > 0 {
			panel.MinHeight(o.MinHeight)
		}
		build(panel)
	})
}

// PopoverDivider 是浮层内的 1px 横向分割线（设计稿 #EDEDED，
// 与 BgHover 同值）。
func PopoverDivider(c *ui.Context) ui.Element {
	m, tk := themv2.Metrics, themv2.Of(c)
	return ui.Box(c).FillWidth().Height(m.Border).Background(tk.BgHover)
}

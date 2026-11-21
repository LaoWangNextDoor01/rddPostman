package theme

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Apply 把设计 token 套到 base 上，让内置控件和自定义组件共用同一套颜色。
// 在根 view 中每帧调用一次，系统深浅色切换后会自动生效：
//
//	c.SetTheme(theme.Apply(c.Theme(), c.Preferences()))
func Apply(base *ui.Theme, p ui.Preferences) *ui.Theme {
	tk := For(base)

	// 先跟随系统的字号缩放与高对比；随后用设计稿颜色覆盖，
	// 避免系统强调色盖掉设计稿的橙色调。
	th := *base.WithPreferences(p)
	th = *th.WithAccent(tk.Accent)
	th.AccentText = tk.OnAccent
	th.Focus = tk.FocusRing

	// th.Background = tk.BgBase
	th.Surface = tk.BgSurface
	th.SurfaceHover = tk.HoverSurface
	// 设计稿没有 pressed 变量，在 hover 基础上向文字色混一点，
	// 浅色变深、深色变浅，两种外观都成立。
	th.SurfacePressed = tk.HoverSurface.Mix(tk.TextPrimary, 0.08)
	th.Border = tk.Border

	th.Text = tk.TextPrimary
	th.TextMuted = tk.TextSecondary

	th.Danger = tk.Danger
	th.Warning = tk.Warning
	th.Success = tk.Success

	// 全局字体：现代复合字体栈（首推方案：西文 Inter 优先，中文平滑匹配 苹方 / 微软雅黑 / Segoe UI）
	th.Font = "Inter, -apple-system, BlinkMacSystemFont, PingFang SC, Hiragino Sans GB, Microsoft YaHei, Segoe UI, system-ui, sans-serif"

	// 图标栏提示在指针停留 400ms 后出现，与设计稿一致。
	th.TooltipDelay = 400 * time.Millisecond

	return &th
}

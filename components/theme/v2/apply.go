package v2

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Apply 将 v2 设计 Token 应用到 MyGo 基础主题上，确保内置控件与自定义 v2 组件
// 在浅色 / 深色外观下颜色完全一致。
func Apply(base *ui.Theme, p ui.Preferences) *ui.Theme {
	if base == nil {
		base = ui.LightTheme()
	}
	tk := For(base)

	th := *base.WithPreferences(p)
	th = *th.WithAccent(tk.Accent)
	th.AccentText = tk.OnAccent
	th.Focus = tk.Focus

	th.Surface = tk.BgSurface
	th.SurfaceHover = tk.BgHover
	th.SurfacePressed = tk.BgSelected
	th.Border = tk.Border

	th.Text = tk.TextPrimary
	th.TextMuted = tk.TextSecondary

	th.Danger = tk.Danger
	th.Warning = tk.Warning
	th.Success = tk.Success

	th.Font = "Inter, -apple-system, BlinkMacSystemFont, PingFang SC, Hiragino Sans GB, Microsoft YaHei, Segoe UI, system-ui, sans-serif"
	th.TooltipDelay = 400 * time.Millisecond

	return &th
}

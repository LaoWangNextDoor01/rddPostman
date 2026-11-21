// Package theme 定义应用的设计 token，按 Light / Dark 两套取值，
// 与 Pixso 设计文件 iWFIhAgDE6k0bpOQjUzQug 的 "Postman UI Tokens" 变量集一一对应。
package theme

import "github.com/egoist/mygo/ui"

// Tokens 是一个外观下的全部颜色 token。
type Tokens struct {
	BgBase           ui.Color // bg-base：应用底色
	BgSurface        ui.Color // bg-surface：面板、卡片
	BgElevated       ui.Color // bg-elevated：抬起的表面
	HoverSurface     ui.Color // hover-surface：悬停表面
	PopoverBg        ui.Color // popover-bg：弹层表面
	PopoverItemHover ui.Color // popover-item-hover：弹层条目悬停

	Border ui.Color // border：描边与分割线

	TextPrimary   ui.Color // text-primary：主要文字
	TextSecondary ui.Color // text-secondary：次要文字
	TextMuted     ui.Color // text-muted：弱化文字、占位符
	TextDisabled  ui.Color // disabled-text：禁用文字

	Accent     ui.Color // accent：强调色
	AccentSoft ui.Color // accent-soft：强调色浅底
	OnAccent   ui.Color // on-accent：强调色之上的文字
	FocusRing  ui.Color // focus-ring：键盘焦点环

	Danger  ui.Color // danger
	Warning ui.Color // warning
	Success ui.Color // success
	Info    ui.Color // info

	Overlay ui.Color // overlay：模态遮罩
}

// Light 是浅色外观的 token，取值来自设计稿变量集的 Light 模式。
var light = Tokens{
	BgBase:           ui.Hex("#F7F7F8"),
	BgSurface:        ui.Hex("#FFFFFF"),
	BgElevated:       ui.Hex("#F2F2F4"),
	HoverSurface:     ui.Hex("#F2F2F4"),
	PopoverBg:        ui.Hex("#FFFFFF"),
	PopoverItemHover: ui.Hex("#F2F2F4"),
	Border:           ui.Hex("#EDEDED"),
	TextPrimary:      ui.Hex("#1F1F23"),
	TextSecondary:    ui.Hex("#52525B"),
	TextMuted:        ui.Hex("#9A9AA4"),
	TextDisabled:     ui.Hex("#B4B4BC"),
	Accent:           ui.Hex("#FF6C37"),
	AccentSoft:       ui.Hex("#FFF1EB"),
	OnAccent:         ui.Hex("#FFFFFF"),
	FocusRing:        ui.Hex("#FF6C37"),
	Danger:           ui.Hex("#D9483B"),
	Warning:          ui.Hex("#C77700"),
	Success:          ui.Hex("#1F9D6B"),
	Info:             ui.Hex("#2F6FEB"),
	Overlay:          ui.Hex("#0F172A66"),
}

// Dark 是深色外观的 token，取值来自设计稿变量集的 Dark 模式。
var dark = Tokens{
	BgBase:           ui.Hex("#1A1A1C"),
	BgSurface:        ui.Hex("#232326"),
	BgElevated:       ui.Hex("#2A2A2E"),
	HoverSurface:     ui.Hex("#2A2A2E"),
	PopoverBg:        ui.Hex("#2A2A2E"),
	PopoverItemHover: ui.Hex("#333338"),
	Border:           ui.Hex("#343438"),
	TextPrimary:      ui.Hex("#ECECEF"),
	TextSecondary:    ui.Hex("#A1A1AA"),
	TextMuted:        ui.Hex("#6E6E78"),
	TextDisabled:     ui.Hex("#55555F"),
	Accent:           ui.Hex("#FF6C37"),
	AccentSoft:       ui.Hex("#FF6C371F"),
	OnAccent:         ui.Hex("#FFFFFF"),
	FocusRing:        ui.Hex("#FF6C37"),
	Danger:           ui.Hex("#FF5F57"),
	Warning:          ui.Hex("#F2B34B"),
	Success:          ui.Hex("#34D399"),
	Info:             ui.Hex("#6C9EE8"),
	Overlay:          ui.Hex("#00000073"),
}

// Set 是一整套外观 token。应用可以整体替换它来自定义主题，
// 组件不用改：它们始终通过 Of(c) 取当前集合。
type Set struct {
	Light Tokens
	Dark  Tokens
}

// Default 是设计稿导出的两套 token。
var Default = Set{Light: light, Dark: dark}

// current 是 Of 和 Apply 使用的集合；只在主线程构建界面时读写。
var current = Default

// Use 让 Of 和 Apply 改用 s。只能在主线程调用，
// 运行时切换后还需让窗口重绘才会生效。
func Use(s Set) { current = s }

// For 按外观返回当前集合中对应的一套。
func For(t *ui.Theme) Tokens { return current.For(t) }

// For 按外观返回 s 中对应的一套。
func (s Set) For(t *ui.Theme) Tokens {
	if t.Dark {
		return s.Dark
	}
	return s.Light
}

// Of 返回当前帧外观的 token；系统切换深浅色后自动跟随。
func Of(c *ui.Context) Tokens { return For(c.Theme()) }

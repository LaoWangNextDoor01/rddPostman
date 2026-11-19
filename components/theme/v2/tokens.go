// Package v2 是 UI v2 的基础 theme token 层，数值全部来自
// docs/ui-v2/02-design-system.md 与 05-token-migration.md。
//
// 它与旧的 components/theme（v1）双轨并存：旧页面继续用 v1，
// 新页面只 import 本包，不要引用 v1 的 token 字段。
// 组件 v2 落地后，本包是它们唯一的颜色与尺寸来源。
//
// 用法（在手写 UI 时）：
//
//	import themev2 "rddpostman/components/theme/v2"
//
//	func view(c *ui.Context) {
//	    tk := themev2.Of(c)
//	    ui.Column(c).Fill().Background(tk.BgBase).Children(func() {
//	        ui.Text(c, "Hello").FontSize(themev2.Metrics.FontSM).TextColor(tk.TextPrimary)
//	    })
//	}
//
// 约定：
//   - 颜色只允许来自 Tokens，页面里不允许 ui.Hex / 具体数值；
//   - 尺寸只允许来自 Metrics，保持与设计稿同一刻度；
//   - 未在组件稿中标注的颜色（标了「建议值」的）在 Phase 0 核对前
//     不要基于它们做视觉对比验收。
package v2

import "github.com/egoist/mygo/ui"

// Tokens 是一个外观下的全部语义颜色 token。
// 字段按 docs/ui-v2/02-design-system.md §2 命名，注释里标了来源：
// 「组件稿标注」= Pixso 组件页有明确标注；「建议值」= 从帧取色推导，
// Phase 0 逐项核对前只可预览、不可作为验收基准。
type Tokens struct {
	// 背景
	BgBase      ui.Color // 主内容区、编辑器、表格底（Light #FFFFFF / Dark #1E1E1E）
	BgApp       ui.Color // 工作台侧栏、标签条（Light #F9F9F9 / Dark #191919）
	BgTitleBar  ui.Color // 标题栏，与主内容同底色、靠分割线区分（Light #FFFFFF / Dark #1E1E1E）
	BgStatusBar ui.Color // 状态栏（Light #FFFFFF / Dark #1E1E1E）
	BgSubtle    ui.Color // Segment 轨道、搜索框默认底（#F2F2F2 / #2C2C2C）
	BgHover     ui.Color // 通用 hover（#EDEDED / #333333，Dark 建议值）
	BgSelected  ui.Color // 选中/按下（#E6E6E6 / #3A3A3A）
	BgSurface   ui.Color // 卡片、弹层、输入框（#FFFFFF / #1E1E1E）

	// 边框
	Border        ui.Color // 分割线、输入框默认边框（#E6E6E6 / #383838）
	BorderStrong  ui.Color // 输入框 hover 边框、复选框边框（#A6A6A6 / #4E4E4E，Dark 建议值）
	BorderPopover ui.Color // 浮层描边（#D8D8D8 / #4E4E4E，Dark 建议值）

	// 文字
	TextPrimary   ui.Color // 主文字（#212121 / #E4E4E4）
	TextSecondary ui.Color // 次要文字、表头（#6B6B6B / #A8A8A8）
	TextMuted     ui.Color // 占位符、说明（#909090 / #858585）
	TextDisabled  ui.Color // 禁用文字（#B5B5B5 / #636363）

	// 强调色（组件稿标注）
	Accent        ui.Color // 主按钮、选中、POST 方法（#FF6C37）
	AccentHover   ui.Color // 主按钮 hover（#ED5B2A）
	AccentPressed ui.Color // 主按钮按下（#D03E0A，配合内阴影）
	AccentSoft    ui.Color // 强调浅底、选中行（#FFF1EB / #3A2A24，均为建议值）
	OnAccent      ui.Color // 主按钮之上的文字（#FFFFFF）

	// 焦点（组件稿标注：聚焦 1px #097BED + 3px #E7F0FF 外环）
	Focus     ui.Color // 聚焦边框（#097BED / #4C9AFF，Dark 建议值）
	FocusRing ui.Color // 聚焦外环（#E7F0FF / #1E3A5F，Dark 建议值）

	// 语义色
	Danger     ui.Color // 错误输入、删除（#EB2013 / #FF5F57）
	DangerSoft ui.Color // 危险按钮 hover（#FFEBE7 / #3A2220，Dark 建议值）
	Warning    ui.Color // 警告、POST 方法（#C77700 / #F2B34B，Dark 建议值）
	Success    ui.Color // 成功、GET 方法（#1F9D6B / #34D399，Dark 建议值）
	Info       ui.Color // 链接、PUT 方法（#2F6FEB / #6C9EE8，Dark 建议值）

	// 遮罩（沿用 v1 值）
	Overlay ui.Color // 模态遮罩（#0F172A66 / #00000073）

	// 请求方法标签色（02 §6.4；均为建议值，Phase 0 核对）
	MethodGet    ui.Color // GET（#1F9D6B / #34D399）
	MethodPost   ui.Color // POST（#C77700 / #F2B34B）
	MethodPut    ui.Color // PUT（#2F6FEB / #6C9EE8）
	MethodDelete ui.Color // DELETE（#D9483B / #FF5F57）
	MethodPatch  ui.Color // PATCH（#7C4DFF / #A78BFA）
}

// Light 是浅色外观 token（02 §2.1）。
var light = Tokens{
	BgBase:      ui.Hex("#FFFFFF"),
	BgApp:       ui.Hex("#F9F9F9"),
	BgTitleBar:  ui.Hex("#FFFFFF"),
	BgStatusBar: ui.Hex("#FFFFFF"),
	BgSubtle:    ui.Hex("#F2F2F2"),
	BgHover:     ui.Hex("#EDEDED"),
	BgSelected:  ui.Hex("#E6E6E6"),
	BgSurface:   ui.Hex("#FFFFFF"),

	Border:        ui.Hex("#E6E6E6"),
	BorderStrong:  ui.Hex("#A6A6A6"),
	BorderPopover: ui.Hex("#D8D8D8"),

	TextPrimary:   ui.Hex("#212121"),
	TextSecondary: ui.Hex("#6B6B6B"),
	TextMuted:     ui.Hex("#909090"),
	TextDisabled:  ui.Hex("#B5B5B5"),

	Accent:        ui.Hex("#FF6C37"),
	AccentHover:   ui.Hex("#ED5B2A"),
	AccentPressed: ui.Hex("#D03E0A"),
	AccentSoft:    ui.Hex("#FFF1EB"),
	OnAccent:      ui.Hex("#FFFFFF"),

	Focus:     ui.Hex("#097BED"),
	FocusRing: ui.Hex("#E7F0FF"),

	Danger:     ui.Hex("#EB2013"),
	DangerSoft: ui.Hex("#FFEBE7"),
	Warning:    ui.Hex("#C77700"),
	Success:    ui.Hex("#1F9D6B"),
	Info:       ui.Hex("#2F6FEB"),

	Overlay: ui.Hex("#0F172A66"),

	MethodGet:    ui.Hex("#1F9D6B"),
	MethodPost:   ui.Hex("#C77700"),
	MethodPut:    ui.Hex("#2F6FEB"),
	MethodDelete: ui.Hex("#D9483B"),
	MethodPatch:  ui.Hex("#7C4DFF"),
}

// Dark 是深色外观 token（02 §2.2）。
var dark = Tokens{
	BgBase:      ui.Hex("#1E1E1E"),
	BgApp:       ui.Hex("#191919"),
	BgTitleBar:  ui.Hex("#1E1E1E"),
	BgStatusBar: ui.Hex("#1E1E1E"),
	BgSubtle:    ui.Hex("#2C2C2C"),
	BgHover:     ui.Hex("#333333"),
	BgSelected:  ui.Hex("#3A3A3A"),
	BgSurface:   ui.Hex("#1E1E1E"),

	Border:        ui.Hex("#383838"),
	BorderStrong:  ui.Hex("#4E4E4E"),
	BorderPopover: ui.Hex("#4E4E4E"),

	TextPrimary:   ui.Hex("#E4E4E4"),
	TextSecondary: ui.Hex("#A8A8A8"),
	TextMuted:     ui.Hex("#858585"),
	TextDisabled:  ui.Hex("#636363"),

	Accent:        ui.Hex("#FF6C37"),
	AccentHover:   ui.Hex("#ED5B2A"),
	AccentPressed: ui.Hex("#D03E0A"),
	AccentSoft:    ui.Hex("#3A2A24"),
	OnAccent:      ui.Hex("#FFFFFF"),

	Focus:     ui.Hex("#4C9AFF"),
	FocusRing: ui.Hex("#1E3A5F"),

	Danger:     ui.Hex("#FF5F57"),
	DangerSoft: ui.Hex("#3A2220"),
	Warning:    ui.Hex("#F2B34B"),
	Success:    ui.Hex("#34D399"),
	Info:       ui.Hex("#6C9EE8"),

	Overlay: ui.Hex("#00000073"),

	MethodGet:    ui.Hex("#34D399"),
	MethodPost:   ui.Hex("#F2B34B"),
	MethodPut:    ui.Hex("#6C9EE8"),
	MethodDelete: ui.Hex("#FF5F57"),
	MethodPatch:  ui.Hex("#A78BFA"),
}

// Set 是一整套外观 token；整体替换它即可定制主题，组件不用改。
type Set struct {
	Light Tokens
	Dark  Tokens
}

// Default 是设计稿导出的两套 token。
var Default = Set{Light: light, Dark: dark}

// current 是 Of / For 使用的集合；只在主线程构建界面时读写。
var current = Default

// Use 让 Of / For 改用 s。只能在主线程调用；切换后需让窗口重绘。
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

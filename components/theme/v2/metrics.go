package v2

import "time"

// MotionTimings 是动效时长 token（02 §4.3）。
// Postman 风格：快速、克制，缓动用 MyGo 默认的 EaseOut。
type MotionTimings struct {
	// Fast 用于 hover/press 的背景与文字色过渡（120ms）。
	Fast time.Duration
	// Base 用于尺寸与位置过渡（200ms，即 MyGo 的默认时长）。
	Base time.Duration
}

// Motion 是动效时长 token；配合
// ui.ElementTransition{Colors: true, Duration: Motion.Fast} 使用。
// 尊重系统的「减少动态效果」时，把 Duration 置为负数即可关闭动画。
var Motion = MotionTimings{
	Fast: 120 * time.Millisecond,
	Base: 200 * time.Millisecond,
}

// Metric 是 UI v2 的尺寸刻度：间距、圆角、字号、行高、控件与区域尺寸。
// 数值全部来自 docs/ui-v2/02-design-system.md §3–§5，单位 DIP。
//
// 间距沿用 Tailwind 的步进命名：SpaceN = N × 4px，
// 半步写作 05 / 15（对应 Tailwind 的 0.5 / 1.5）。与 v1 同名同值，
// 迁移时不需要换算。
//
// 与 v1 的其他关键差异：
//   - 圆角只有 4 / 6 / 8 / Full，删除 10；
//   - 字号新增 11，删除 18；
//   - 删除 IconRailWidth（新设计没有图标栏）；
//   - TitleBarHeight 只是全屏/无系统按钮时的兜底值，
//     正常情况一律用 c.TitleBar()（见 02 §5.1）。
type Metric struct {
	// 间距（Tailwind 步进：SpaceN = N × 4px；Space05 = 2，Space15 = 6）
	Space0, Space05, Space1, Space15, Space2, Space3, Space4, Space5, Space6, Space8 float32

	// 圆角
	RadiusSM, RadiusMD, RadiusLG, RadiusFull float32

	// 字号
	FontMicro, FontXS, FontSM, FontMD, FontLG, FontXL, FontXXL float32

	// 行高（与字号一一对应）
	LineMicro, LineXS, LineSM, LineMD, LineLG, LineXL, LineXXL float32

	// 边框
	Border float32

	// 图标尺寸
	IconXS, IconSM, IconMD, IconLG float32

	// 通用方形尺寸：头像、图标底、logo、方形按钮等（4 的倍数阶梯）
	SizeXS, SizeSM, SizeMD, SizeLG, SizeXL float32

	// 控件高度（组件稿标注：按钮 24/28/32、输入 28、搜索 22、下拉 32）
	ButtonSmall       float32 // 工具条文字按钮
	ButtonHeight      float32 // 默认按钮
	ButtonLarge       float32 // 弹窗/空态主操作
	InputHeight       float32 // 单行输入
	SearchHeight      float32 // 搜索框
	SelectHeight      float32 // 下拉触发器
	RowHeight         float32 // 表格行 / 树行
	TableHeaderHeight float32 // 表头

	// 区域尺寸
	TitleBarHeight  float32 // 兜底高度；正常情况用 c.TitleBar()（02 §5.1）
	StatusBarHeight float32
	SidebarWidth    float32 // 管理端与工作台统一侧栏宽度
	TabBarHeight    float32 // 工作台顶部标签条

	// 页面内容区留白（建议值）
	ContentPaddingX, ContentPaddingY float32
}

// Metrics 是设计稿的尺寸 token（02 §4–§5）。
var Metrics = Metric{
	Space0: 0, Space05: 2, Space1: 4, Space15: 6, Space2: 8,
	Space3: 12, Space4: 16, Space5: 20, Space6: 24, Space8: 32,

	RadiusSM: 4, RadiusMD: 6, RadiusLG: 8, RadiusFull: 999,

	FontMicro: 10, FontXS: 11, FontSM: 12, FontMD: 14, FontLG: 16, FontXL: 20, FontXXL: 24,

	LineMicro: 14, LineXS: 16, LineSM: 18, LineMD: 22,
	LineLG: 24, LineXL: 28, LineXXL: 32,

	Border: 1,

	IconXS: 12, IconSM: 14, IconMD: 16, IconLG: 18,

	SizeXS: 16, SizeSM: 20, SizeMD: 24, SizeLG: 28, SizeXL: 32,

	ButtonSmall:       24,
	ButtonHeight:      28,
	ButtonLarge:       32,
	InputHeight:       28,
	SearchHeight:      22,
	SelectHeight:      32,
	RowHeight:         26,
	TableHeaderHeight: 30,

	TitleBarHeight:  36,
	StatusBarHeight: 24,
	SidebarWidth:    232,
	TabBarHeight:    35,

	ContentPaddingX: 20,
	ContentPaddingY: 20,
}

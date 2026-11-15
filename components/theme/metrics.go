package theme

// Metric 是设计稿的尺寸刻度：间距、圆角、字号、控件与区域尺寸。
// 尺寸不区分浅色和深色，只有颜色需要两套 token。
type Metric struct {
	// 间距，4 的倍数
	Space1, Space2, Space3, Space4, Space5, Space6, Space8 float32

	// 圆角
	RadiusSM, RadiusMD, RadiusLG, RadiusXL, RadiusFull float32

	// 字号
	FontXS, FontSM, FontMD, FontLG, FontXL, FontXXL float32

	// 行高
	LineXS, LineSM, LineMD float32

	// 边框
	Border float32

	// 图标尺寸
	IconXS, IconSM, IconMD, IconLG float32

	// 通用控件尺寸：控件高度、头像、方形按钮等
	SizeXS, SizeSM, SizeMD, SizeLG, SizeXL float32

	// 区域尺寸
	TitleBarHeight  float32 // 仅作设计参考，实际用 c.TitleBar()
	IconRailWidth   float32
	SidebarWidth    float32
	StatusBarHeight float32

	// 页面内容区
	PagePaddingX, PagePaddingY float32

	// 卡片与标签
	CardHeight          float32
	TagWidth, TagHeight float32
}

// Metrics 是设计稿的尺寸 token，全部单位为 DIP。
var Metrics = Metric{
	Space1: 4, Space2: 8, Space3: 12, Space4: 16, Space5: 20, Space6: 24, Space8: 32,

	RadiusSM: 4, RadiusMD: 6, RadiusLG: 8, RadiusXL: 10, RadiusFull: 999,

	FontXS: 11, FontSM: 12, FontMD: 13, FontLG: 14, FontXL: 16, FontXXL: 20,

	LineXS: 15, LineSM: 16, LineMD: 18,

	Border: 1,

	IconXS: 12, IconSM: 14, IconMD: 16, IconLG: 18,

	SizeXS: 20, SizeSM: 22, SizeMD: 24, SizeLG: 28, SizeXL: 34,

	TitleBarHeight: 40, IconRailWidth: 52, SidebarWidth: 268, StatusBarHeight: 26,

	PagePaddingX: 24, PagePaddingY: 20,

	CardHeight: 96,
	TagWidth:   50, TagHeight: 23,
}

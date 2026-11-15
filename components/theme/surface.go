package theme

import "github.com/egoist/mygo/ui"

// PopoverPanel 给下拉、弹层套上统一外观：popover-bg、10 圆角和外层阴影。
// 调用方只负责尺寸、内边距和内容，保证所有弹层看起来是同一套东西。
func PopoverPanel(c *ui.Context, panel ui.Element) {
	tk := Of(c)
	panel.Background(tk.PopoverBg).
		Radius(Metrics.RadiusXL).
		Shadow(0, 14, 32, 0, ui.Hex("#0F16281E"))
}

// DialogPanel 给对话框面板套上统一外观。比弹层多一条边框，
// 保证在遮罩和内容之上边界清晰。
func DialogPanel(c *ui.Context, panel ui.Element) {
	tk := Of(c)
	panel.Background(tk.PopoverBg).
		Radius(Metrics.RadiusXL).
		Border(Metrics.Border, tk.Border).
		Shadow(0, 18, 48, 0, ui.Hex("#0F16282E"))
}

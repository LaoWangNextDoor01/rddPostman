package v1

import (
	"rddpostman/components/theme"

	"github.com/egoist/mygo/ui"
)

// Tag 固定宽 50；高度 = 行高 15 + 上下各 4 = 23。
// 默认样式都放在根上，调用方可以链式覆盖。
func Tag(c *ui.Context, label string) ui.Element {
	m := theme.Metrics
	t := c.Theme()
	tk := theme.Of(c)
	tag := ui.Row(c).
		AlignSelf(ui.Start).
		Shrink(0).
		Width(m.TagWidth).
		PaddingY(m.Space1).
		Justify(ui.Center).
		Radius(m.RadiusSM).
		Background(tk.BgBase).
		FontSize(m.FontSM).
		FixedLineHeight(m.LineXS).
		TextColor(t.TextMuted) // 根上的默认色，向下继承

	tag.Children(func() {
		ui.Text(c, label).SingleLine() // 不写颜色/字号
	})
	return tag
}

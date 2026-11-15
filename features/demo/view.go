// Package demo 是一个空白页面，用于手动测试组件。
package demo

import "github.com/egoist/mygo/ui"

// View 绘制空白页。把要测试的组件写在这里。
func View(c *ui.Context) {
	ui.Box(c).Grow(1).Children(func() {
		ui.Text(c, "Demo")
	})
}

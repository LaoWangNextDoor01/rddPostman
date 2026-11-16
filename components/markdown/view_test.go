package markdown

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestRenderView(t *testing.T) {
	doc := `# 集合概览

这是集合的第一段描述，包含 **加粗** 和 *斜体* 以及 ` + "`inline_code`" + ` 和 [链接](https://example.com)。

- 任务 1
- [x] 完成任务
- [ ] 未完成任务

> 一段引述文字

` + "```go\nfunc main() {\n    println(\"hello world\")\n}\n```" + `

---

| 属性 | 说明 |
| :--- | ---: |
| name | 用户名 |
`
	view := func(c *ui.Context) {
		ui.Column(c).Width(720).Children(func() {
			View(c, doc)
		})
	}

	tt := ui.NewTester(view, 720, 800)
	tt.Frame()
	img := tt.Image()
	if img == nil {
		t.Fatal("渲染结果为空")
	}
}

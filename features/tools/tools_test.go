package tools

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestToolsSidebarFilterAndSelect(t *testing.T) {
	var opened string
	view := func(c *ui.Context) {
		SidebarView(c, opened, func(toolID string) {
			opened = toolID
		})
	}

	tt := ui.NewTester(view, 232, 640)
	if !tt.HasText("数据处理") || !tt.HasText("JSON 格式化 / 转义") || !tt.HasText("哈希与 HMAC 签名") || !tt.HasText("编解码工作台") {
		t.Fatalf("expected unified tools tree in sidebar; got %q", tt.Texts())
	}

	if err := tt.Click("Filter dev tools"); err != nil {
		t.Fatalf("Click filter input: %v", err)
	}
	tt.Type("hmac")
	tt.Frame()
	if !tt.HasText("哈希与 HMAC 签名") || tt.HasText("UUID / 随机串") {
		t.Fatalf("expected filter 'hmac' to narrow tools list; got %q", tt.Texts())
	}

	if err := tt.Click("哈希与 HMAC 签名"); err != nil {
		t.Fatalf("Click Hash tool: %v", err)
	}
	tt.Frame()
	if opened != ToolHash {
		t.Fatalf("opened = %q, want %q", opened, ToolHash)
	}
}

func TestToolsEditorsRenderAndInteract(t *testing.T) {
	// 1. JSON：空态 -> 载入示例 -> 格式化成功 + Key 排序 + 压缩
	ttJSON := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolJSON)
	}, 1047, 705)
	if !ttJSON.HasText("粘贴或输入 JSON 文本") {
		t.Fatalf("expected JSON empty state; got %q", ttJSON.Texts())
	}
	if err := ttJSON.Click("载入示例数据"); err != nil {
		t.Fatalf("Click JSON sample: %v", err)
	}
	ttJSON.Frame()
	if !ttJSON.HasText("Valid JSON") {
		t.Fatalf("expected Valid JSON badge after loading sample; got %q", ttJSON.Texts())
	}
	if err := ttJSON.Click("压缩"); err != nil {
		t.Fatalf("Click Minify tab: %v", err)
	}
	ttJSON.Frame()
	if !ttJSON.HasText("已压缩") {
		t.Fatalf("expected minified badge; got %q", ttJSON.Texts())
	}

	// 2. 编解码工作台：Base64 -> URL -> 自动解析 Query Params 预览
	ttCodec := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolCodec)
	}, 1047, 705)
	if err := ttCodec.Click("URL"); err != nil {
		t.Fatalf("Click URL tab: %v", err)
	}
	ttCodec.Frame()
	if err := ttCodec.Click("载入示例"); err != nil {
		t.Fatalf("Click URL sample: %v", err)
	}
	ttCodec.Frame()
	if !ttCodec.HasText("QUERY PARAMS 自动解析预览 (3 项)") {
		t.Fatalf("expected URL query preview table; got %q", ttCodec.Texts())
	}

	// 3. 哈希与 HMAC：实时计算 MD5 + 摘要比对匹配 + 切 HMAC 模式
	ttHash := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolHash)
	}, 1047, 705)
	if err := ttHash.Click("Hash input text"); err != nil {
		t.Fatalf("Click hash input: %v", err)
	}
	ttHash.Type("hello")
	ttHash.Frame()
	if !ttHash.HasText("5d41402abc4b2a76b9719d911017c592") {
		t.Fatalf("expected MD5 hash of 'hello'; got %q", ttHash.Texts())
	}
	if err := ttHash.Click("摘要校验比对"); err != nil {
		t.Fatalf("Click compare toggle: %v", err)
	}
	ttHash.Frame()
	if err := ttHash.Click("Expected hash compare"); err != nil {
		t.Fatalf("Click compare input: %v", err)
	}
	ttHash.Type("5d41402abc4b2a76b9719d911017c592")
	ttHash.Frame()
	if !ttHash.HasText("完全匹配 · MD5 (32位)") {
		t.Fatalf("expected matched MD5 pill; got %q", ttHash.Texts())
	}

	// 4. JWT：载入示例 -> 解析 Header/Payload -> 展开验签 -> 验签通过
	ttJWT := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolJWT)
	}, 1047, 705)
	if err := ttJWT.Click("载入示例 JWT"); err != nil {
		t.Fatalf("Click JWT sample: %v", err)
	}
	ttJWT.Frame()
	if !ttJWT.HasText("TIME CLAIMS · 时间声明换算") {
		t.Fatalf("expected decoded JWT Time Claims card; got %q", ttJWT.Texts())
	}
	if err := ttJWT.Click("输入密钥验证签名"); err != nil {
		t.Fatalf("Click verify signature: %v", err)
	}
	ttJWT.Frame()
	if !ttJWT.HasText("Signature Verified ✓") {
		t.Fatalf("expected verified signature pill; got %q", ttJWT.Texts())
	}

	// 5. 时间戳 + UUID / 随机串：均正常渲染与切换
	ttTS := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolTimestamp)
	}, 1047, 705)
	if !ttTS.HasText("Unix 时间戳与时区转换") || !ttTS.HasText("ISO 8601") {
		t.Fatalf("expected timestamp view; got %q", ttTS.Texts())
	}
	if err := ttTS.Click("本年末"); err != nil {
		t.Fatalf("Click preset: %v", err)
	}
	ttTS.Frame()

	ttUUID := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolUUID)
	}, 1047, 705)
	if !ttUUID.HasText("UUID 与高熵随机串生成") {
		t.Fatalf("expected UUID view; got %q", ttUUID.Texts())
	}
	if err := ttUUID.Click("随机字符串"); err != nil {
		t.Fatalf("Click random string tab: %v", err)
	}
	ttUUID.Frame()
	if !ttUUID.HasText("极强 CSPRNG") {
		t.Fatalf("expected CSPRNG entropy pill; got %q", ttUUID.Texts())
	}

	// 6. 旧 ID 别名兼容（base64 / url / unicode）
	for _, alias := range []string{ToolBase64, ToolURL, ToolUnicode} {
		tt := ui.NewTester(func(c *ui.Context) {
			EditorView(c, alias)
		}, 1047, 705)
		if !tt.HasText("编解码工作台") {
			t.Fatalf("alias %q failed to render codec workbench", alias)
		}
	}
}

func TestToolsSidebarTooltipRightSide(t *testing.T) {
	view := func(c *ui.Context) {
		ui.Row(c).Fill().Children(func() {
			ui.Box(c).Width(232).FillHeight().Children(func() {
				SidebarView(c, ToolJSON, nil)
			})
			ui.Box(c).Grow(1).FillHeight()
		})
	}
	tt := ui.NewTester(view, 900, 640)
	rowRect, ok := tt.Find("JSON 格式化 / 转义")
	if !ok {
		t.Fatalf("could not find sidebar item")
	}
	tt.Move(rowRect.X+rowRect.W/2, rowRect.Y+rowRect.H/2)
	tt.Advance(700 * time.Millisecond)

	desc := Find(ToolJSON).Description
	tipRect, ok := tt.Find(desc)
	if !ok {
		t.Fatalf("expected tooltip %q on hover; texts=%q", desc, tt.Texts())
	}
	if tipRect.X < rowRect.X+rowRect.W {
		t.Fatalf("expected tooltip to appear on the right of sidebar row: tip=%+v row=%+v", tipRect, rowRect)
	}
}

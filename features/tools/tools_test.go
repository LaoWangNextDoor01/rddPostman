package tools

import (
	"testing"
	"time"

	"rddpostman/locales"

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
	if !tt.HasText(locales.ToolsCatData()) || !tt.HasText(locales.ToolJSONName()) || !tt.HasText(locales.ToolHashName()) || !tt.HasText(locales.ToolCodecName()) {
		t.Fatalf("expected unified tools tree in sidebar; got %q", tt.Texts())
	}

	if err := tt.Click(locales.ToolsFilterLabel()); err != nil {
		t.Fatalf("Click filter input: %v", err)
	}
	tt.Type("hmac")
	tt.Frame()
	if !tt.HasText(locales.ToolHashName()) || tt.HasText(locales.ToolUUIDName()) {
		t.Fatalf("expected filter 'hmac' to narrow tools list; got %q", tt.Texts())
	}

	if err := tt.Click(locales.ToolHashName()); err != nil {
		t.Fatalf("Click Hash tool: %v", err)
	}
	tt.Frame()
	if opened != ToolHash {
		t.Fatalf("opened = %q, want %q", opened, ToolHash)
	}
}

func TestToolsEditorsRenderAndInteract(t *testing.T) {
	// 1. JSON：默认直接展示左侧编辑器（无空态遮罩按钮组）+ 保留一键粘贴按钮 -> 直接输入 / 载入示例 -> 格式化成功 + 压缩
	ttJSON := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolJSON)
	}, 1047, 705)
	if _, ok := ttJSON.Find("JSON input"); !ok {
		t.Fatalf("expected JSON input editor to be directly visible without extra click; got %q", ttJSON.Texts())
	}
	if _, ok := ttJSON.Find(locales.PasteFromClipboard()); !ok {
		t.Fatalf("expected Paste from Clipboard button in JSON INPUT pane header; got %q", ttJSON.Texts())
	}
	if ttJSON.HasText(locales.ManualInput()) {
		t.Fatalf("empty overlay ManualInput button should be removed; got %q", ttJSON.Texts())
	}
	if err := ttJSON.Click(locales.LoadSample()); err != nil {
		t.Fatalf("Click JSON sample: %v", err)
	}
	ttJSON.Frame()
	if !ttJSON.HasText(locales.JsonValid()) {
		t.Fatalf("expected Valid JSON badge after loading sample; got %q", ttJSON.Texts())
	}
	if err := ttJSON.Click(locales.JsonModeMinify()); err != nil {
		t.Fatalf("Click Minify tab: %v", err)
	}
	ttJSON.Frame()
	if !ttJSON.HasText(locales.JsonMinified()) {
		t.Fatalf("expected minified badge; got %q", ttJSON.Texts())
	}

	// 2. 编解码工作台：默认直接展示左侧编辑器 + 保留一键粘贴按钮 -> URL -> 自动解析 Query Params 预览
	ttCodec := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolCodec)
	}, 1047, 705)
	if _, ok := ttCodec.Find("Codec input"); !ok {
		t.Fatalf("expected Codec input editor to be directly visible without extra click; got %q", ttCodec.Texts())
	}
	if _, ok := ttCodec.Find(locales.PasteFromClipboard()); !ok {
		t.Fatalf("expected Paste from Clipboard button in Codec INPUT pane header; got %q", ttCodec.Texts())
	}
	if ttCodec.HasText(locales.ManualInput()) {
		t.Fatalf("codec empty overlay ManualInput button should be removed; got %q", ttCodec.Texts())
	}
	if err := ttCodec.Click("URL"); err != nil {
		t.Fatalf("Click URL tab: %v", err)
	}
	ttCodec.Frame()
	if err := ttCodec.Click(locales.LoadSample()); err != nil {
		t.Fatalf("Click URL sample: %v", err)
	}
	ttCodec.Frame()
	if !ttCodec.HasText(locales.CodecQueryPreviewHeader("3")) {
		t.Fatalf("expected URL query preview table; got %q", ttCodec.Texts())
	}

	// 3. 哈希与 HMAC：实时计算 MD5 + 摘要比对匹配 + 切 HMAC 模式
	ttHash := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolHash)
	}, 1047, 705)
	if err := ttHash.Click(locales.HashInputLabel()); err != nil {
		t.Fatalf("Click hash input: %v", err)
	}
	ttHash.Type("hello")
	ttHash.Frame()
	if !ttHash.HasText("5d41402abc4b2a76b9719d911017c592") {
		t.Fatalf("expected MD5 hash of 'hello'; got %q", ttHash.Texts())
	}
	if err := ttHash.Click(locales.HashCompareToggle()); err != nil {
		t.Fatalf("Click compare toggle: %v", err)
	}
	ttHash.Frame()
	if err := ttHash.Click(locales.HashCompareInputLabel()); err != nil {
		t.Fatalf("Click compare input: %v", err)
	}
	ttHash.Type("5d41402abc4b2a76b9719d911017c592")
	ttHash.Frame()
	if !ttHash.HasText(locales.HashMatchExact(locales.HashMD532())) {
		t.Fatalf("expected matched MD5 pill; got %q", ttHash.Texts())
	}

	// 4. JWT：载入示例 -> 解析 Header/Payload -> 展开验签 -> 验签通过
	ttJWT := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolJWT)
	}, 1047, 705)
	if err := ttJWT.Click(locales.JwtLoadSample()); err != nil {
		t.Fatalf("Click JWT sample: %v", err)
	}
	ttJWT.Frame()
	if !ttJWT.HasText(locales.JwtClaimsCardTitle()) {
		t.Fatalf("expected decoded JWT Time Claims card; got %q", ttJWT.Texts())
	}
	if err := ttJWT.Click(locales.JwtOpenVerify()); err != nil {
		t.Fatalf("Click verify signature: %v", err)
	}
	ttJWT.Frame()
	if !ttJWT.HasText(locales.JwtSigVerified()) {
		t.Fatalf("expected verified signature pill; got %q", ttJWT.Texts())
	}
	// 验证 JWT 输入框为多行区域（高度 >= 72）且支持含换行的多行 Token 自动解析
	jwtInRect, ok := ttJWT.Find("JWT token input")
	if !ok {
		t.Fatalf("JWT token input not found")
	}
	if jwtInRect.H < 72 {
		t.Fatalf("expected multiline JWT textarea height >= 72, got %.1f", jwtInRect.H)
	}
	if err := ttJWT.Click(locales.Clear()); err != nil {
		t.Fatalf("Click Clear JWT: %v", err)
	}
	ttJWT.Frame()
	if err := ttJWT.Click("JWT token input"); err != nil {
		t.Fatalf("Click JWT textarea: %v", err)
	}
	sample := makeSampleJWT()
	ttJWT.Type("Authorization: Bearer " + sample[:40] + "\n" + sample[40:])
	ttJWT.Frame()
	if !ttJWT.HasText(locales.JwtClaimsCardTitle()) {
		t.Fatalf("multiline wrapped JWT should decode cleanly; got %q", ttJWT.Texts())
	}

	// 5. 时间戳 + UUID / 随机串：均正常渲染与切换
	ttTS := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolTimestamp)
	}, 1047, 705)
	if !ttTS.HasText(locales.ToolTimestampTitle()) || !ttTS.HasText("ISO 8601") {
		t.Fatalf("expected timestamp view; got %q", ttTS.Texts())
	}
	if err := ttTS.Click(locales.DtPresetYearEnd()); err != nil {
		t.Fatalf("Click preset: %v", err)
	}
	ttTS.Frame()

	ttUUID := ui.NewTester(func(c *ui.Context) {
		EditorView(c, ToolUUID)
	}, 1047, 705)
	if !ttUUID.HasText(locales.ToolUUIDTitle()) {
		t.Fatalf("expected UUID view; got %q", ttUUID.Texts())
	}
	if err := ttUUID.Click(locales.UuidModeRandom()); err != nil {
		t.Fatalf("Click random string tab: %v", err)
	}
	ttUUID.Frame()
	if !ttUUID.HasText(locales.UuidStrengthStrong()) {
		t.Fatalf("expected CSPRNG entropy pill; got %q", ttUUID.Texts())
	}

	// 6. 旧 ID 别名兼容（base64 / url / unicode）
	for _, alias := range []string{ToolBase64, ToolURL, ToolUnicode} {
		tt := ui.NewTester(func(c *ui.Context) {
			EditorView(c, alias)
		}, 1047, 705)
		if !tt.HasText(locales.ToolCodecName()) {
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
	rowRect, ok := tt.Find(locales.ToolJSONName())
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

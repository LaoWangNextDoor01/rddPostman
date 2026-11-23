package tools

import (
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// EditorView 在 WorkbenchLayout 右侧多 Tab 区域渲染指定小工具的操作界面。
func EditorView(c *ui.Context, toolID string) {
	def := Find(toolID)
	tk := thV2.Of(c)
	m := thV2.Metrics
	if def == nil {
		ui.Column(c).Fill().Center().Background(tk.BgBase).Gap(m.Space2).Children(func() {
			ui.Icon(c, icons.Code).FontSize(24).TextColor(tk.TextMuted)
			ui.Text(c, "Tool not found").FontSize(m.FontMD).TextColor(tk.TextSecondary)
		})
		return
	}

	ui.Column(c.Key("tool-editor-"+toolID)).Fill().Background(tk.BgBase).Children(func() {
		switch def.ID {
		case ToolJSON:
			renderJSONTool(c)
		case ToolCodec:
			initialMode := codecModeBase64
			if toolID == ToolURL {
				initialMode = codecModeURL
			} else if toolID == ToolUnicode {
				initialMode = codecModeUnicode
			}
			renderCodecTool(c, toolID, initialMode)
		case ToolHash:
			renderHashTool(c)
		case ToolJWT:
			renderJWTTool(c)
		case ToolTimestamp:
			renderTimestampTool(c)
		case ToolUUID:
			renderUUIDTool(c)
		}
	})
}

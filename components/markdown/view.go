package markdown

import (
	"fmt"
	"strings"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"

	"github.com/egoist/mygo/ui"
)

// View 渲染 markdown 内容。如果为空，不产生任何元素。
func View(c *ui.Context, src string) ui.Element {
	return ViewWithWidth(c, src, 0)
}

// ViewWithWidth 按指定宽度渲染 markdown 内容。width <= 0 表示撑满父容器。
func ViewWithWidth(c *ui.Context, src string, width float32) ui.Element {
	tk := theme.Of(c)
	m := theme.Metrics

	blocks := ParseBlocks(src)
	col := ui.Column(c).Gap(m.Space2)
	if width > 0 {
		col.Width(width)
	} else {
		col.FillWidth()
	}

	col.Children(func() {
		for idx, b := range blocks {
			b := b
			key := fmt.Sprintf("md-blk-%d", idx)
			c.Key(key)

			switch b.Type {
			case BlockHeading:
				renderHeading(c, b, tk, m)
			case BlockParagraph:
				renderParagraph(c, b.Text, tk, m)
			case BlockCode:
				renderCodeBlock(c, b.Lang, b.Code, tk, m, key)
			case BlockQuote:
				renderQuote(c, b.Quote, tk, m)
			case BlockList:
				renderList(c, b, tk, m)
			case BlockTable:
				renderTable(c, b, tk, m)
			case BlockThematicBreak:
				ui.Box(c).FillWidth().Height(m.Border).Background(tk.Border).Margin(12, 0)
			}
		}
	})

	return col
}

func renderHeading(c *ui.Context, b Block, tk theme.Tokens, m theme.Metric) {
	var fontSize float32
	var topMargin, bottomMargin float32

	switch b.Level {
	case 1:
		fontSize = m.FontXXL
		topMargin, bottomMargin = 16, 8
	case 2:
		fontSize = m.FontXL
		topMargin, bottomMargin = 14, 6
	case 3:
		fontSize = m.FontMD
		topMargin, bottomMargin = 12, 4
	case 4:
		fontSize = m.FontSM
		topMargin, bottomMargin = 10, 4
	default:
		fontSize = m.FontSM
		topMargin, bottomMargin = 8, 2
	}

	row := ui.Row(c).FillWidth().Margin(topMargin, 0, bottomMargin, 0).AlignItems(ui.Center)
	row.Children(func() {
		spans := toSpans(b.Text, tk, m, fontSize, 600)
		ui.RichText(c, spans...).FontSize(fontSize).FontWeight(600).
			TextColor(tk.TextPrimary)
	})
}

func renderParagraph(c *ui.Context, text string, tk theme.Tokens, m theme.Metric) {
	segs := ParseInline(text)
	hasLink := false
	for _, s := range segs {
		if s.Link != "" {
			hasLink = true
			break
		}
	}

	if !hasLink {
		spans := toSpans(text, tk, m, m.FontMD, 400)
		ui.RichText(c, spans...).FillWidth().FontSize(m.FontMD).
			FixedLineHeight(24).TextColor(tk.TextPrimary).Margin(2, 0, 6, 0)
		return
	}

	// 存在链接时，用 RichText.Children 渲染
	ui.RichText(c).FillWidth().FontSize(m.FontMD).FixedLineHeight(24).
		TextColor(tk.TextPrimary).Margin(2, 0, 6, 0).Children(func() {
		for _, seg := range segs {
			if seg.Link != "" {
				ui.Link(c, seg.Text, seg.Link).FontSize(m.FontMD).
					TextColor(tk.Info).Underline()
			} else {
				t := ui.Text(c, seg.Text).FontSize(m.FontMD).TextColor(tk.TextPrimary)
				if seg.Bold {
					t.Bold()
				}
				if seg.Italic {
					t.Italic()
				}
				if seg.Code {
					t.Font("monospace").TextColor(tk.Accent)
				}
			}
		}
	})
}

func renderCodeBlock(c *ui.Context, lang, code string, tk theme.Tokens, m theme.Metric, key string) {
	ui.Column(c).FillWidth().Margin(6, 0, 10, 0).Radius(m.RadiusMD).
		Background(tk.BgElevated).Border(m.Border, tk.Border).Children(func() {
		// 顶栏：语言标签与复制按钮
		ui.Row(c).FillWidth().Height(28).PaddingX(m.Space2).AlignItems(ui.Center).
			Background(tk.HoverSurface).Radius(m.RadiusMD).Children(func() {
			displayLang := "CODE"
			if lang != "" {
				displayLang = strings.ToUpper(lang)
			}
			ui.Text(c, displayLang).FontSize(10).FontWeight(600).
				TextColor(tk.TextMuted)

			ui.Spacer(c)

			copyBtn := ui.ButtonBase(c.Key(key + "-copy")).Height(20).PaddingX(6).
				Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
			copied := ui.Local(copyBtn, "copied", func() bool { return false })
			if copyBtn.Hovered() {
				copyBtn.Background(tk.Border)
			}
			if copyBtn.Clicked() {
				c.Services().WriteClipboard(code)
				*copied = true
			}
			copyBtn.Children(func() {
				if *copied {
					ui.Text(c, "已复制").FontSize(10).TextColor(tk.Success)
				} else {
					ui.Icon(c, icons.Get("copy")).FontSize(11).TextColor(tk.TextMuted)
					ui.Text(c, "复制").FontSize(10).TextColor(tk.TextMuted)
				}
			})
		})

		// 代码内容
		ui.Scroll(c).FillWidth().MaxHeight(360).Padding(10, 12).Children(func() {
			ui.Text(c, code).Font("monospace").FontSize(m.FontSM).
				FixedLineHeight(20).TextColor(tk.TextPrimary)
		})
	})
}

func renderQuote(c *ui.Context, quoteLines []string, tk theme.Tokens, m theme.Metric) {
	ui.Row(c).FillWidth().Margin(4, 0, 8, 0).Children(func() {
		// 左侧重点色坚条
		ui.Box(c).Width(3).Radius(1.5).Background(tk.Accent).Margin(2, 8, 2, 0)
		ui.Column(c).Grow(1).Gap(3).Children(func() {
			for _, line := range quoteLines {
				spans := toSpans(line, tk, m, m.FontSM, 400)
				for i := range spans {
					spans[i].Italic = true
					if spans[i].Color == (ui.Color{}) {
						spans[i].Color = tk.TextSecondary
					}
				}
				ui.RichText(c, spans...).FontSize(m.FontSM).FixedLineHeight(20).
					TextColor(tk.TextSecondary)
			}
		})
	})
}

func renderList(c *ui.Context, b Block, tk theme.Tokens, m theme.Metric) {
	ui.Column(c).FillWidth().Margin(4, 0, 8, 0).Gap(4).Children(func() {
		for i, item := range b.Items {
			ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Start).Children(func() {
				if item.IsTask {
					// 任务清单勾选框
					box := ui.Box(c).Size(14, 14).Radius(m.RadiusSM).
						Border(m.Border, tk.Border).Center().Margin(5, 0, 0, 0)
					if item.Checked {
						box.Background(tk.AccentSoft)
						ui.Text(c, "✓").FontSize(10).FontWeight(700).TextColor(tk.Accent)
					}
				} else if b.Ordered {
					ui.Text(c, fmt.Sprintf("%d.", i+1)).FontSize(m.FontMD).
						TextColor(tk.TextMuted).Width(18)
				} else {
					ui.Box(c).Size(4, 4).Radius(m.RadiusFull).
						Background(tk.TextMuted).Margin(10, 2, 0, 2)
				}

				spans := toSpans(item.Text, tk, m, m.FontMD, 400)
				ui.RichText(c, spans...).Grow(1).FontSize(m.FontMD).
					FixedLineHeight(24).TextColor(tk.TextPrimary)
			})
		}
	})
}

func renderTable(c *ui.Context, b Block, tk theme.Tokens, m theme.Metric) {
	ui.Column(c).FillWidth().Margin(6, 0, 10, 0).Radius(m.RadiusMD).
		Border(m.Border, tk.Border).Children(func() {
		// 表头
		if len(b.Headers) > 0 {
			ui.Row(c).FillWidth().Height(32).PaddingX(m.Space2).
				AlignItems(ui.Center).Background(tk.BgElevated).Children(func() {
				for i, h := range b.Headers {
					align := getAlign(b.Aligns, i)
					text := ui.Text(c, h).FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.TextPrimary).Grow(1)
					if align == "center" {
						text.TextAlign(ui.Center)
					} else if align == "right" {
						text.TextAlign(ui.End)
					}
				}
			})
		}

		// 表格行
		for rIdx, row := range b.Rows {
			ui.Box(c).FillWidth().Height(m.Border).Background(tk.Border)
			rowElem := ui.Row(c).FillWidth().MinHeight(30).Padding(4, m.Space2).
				AlignItems(ui.Center)
			if rIdx%2 == 1 {
				rowElem.Background(tk.HoverSurface.Alpha(0.3))
			}
			rowElem.Children(func() {
				for i, cell := range row {
					align := getAlign(b.Aligns, i)
					text := ui.Text(c, cell).FontSize(m.FontSM).
						TextColor(tk.TextSecondary).Grow(1)
					if align == "center" {
						text.TextAlign(ui.Center)
					} else if align == "right" {
						text.TextAlign(ui.End)
					}
				}
			})
		}
	})
}

func getAlign(aligns []string, i int) string {
	if i < len(aligns) {
		return aligns[i]
	}
	return "left"
}

func toSpans(text string, tk theme.Tokens, m theme.Metric, fontSize float32, baseWeight int) []ui.Span {
	segs := ParseInline(text)
	spans := make([]ui.Span, 0, len(segs))

	for _, seg := range segs {
		span := ui.Span{
			Text:   seg.Text,
			Size:   fontSize,
			Weight: baseWeight,
		}
		if seg.Bold {
			span.Weight = 700
		}
		if seg.Italic {
			span.Italic = true
		}
		if seg.Strike {
			span.Strikethrough = true
		}
		if seg.Code {
			span.Font = "monospace"
			span.Background = tk.BgElevated
			span.Color = tk.Accent
			span.PaddingX = 4
			span.PaddingY = 1
			span.Radius = m.RadiusSM
		}
		spans = append(spans, span)
	}

	return spans
}

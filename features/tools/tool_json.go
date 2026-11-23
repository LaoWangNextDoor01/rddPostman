package tools

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/devtools"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const sampleJSONText = `{"orderId":"ORD-20261123-8842","status":"paid","amount":19800,"customer":{"name":"Alice Chen","vip":true,"tags":["pro","api"]}}`

const (
	jsonModeFormat   = 0
	jsonModeMinify   = 1
	jsonModeEscape   = 2
	jsonModeUnescape = 3
)

type jsonIndentOption struct {
	key   string
	label string
	short string
	pad   string
	hint  string
}

var jsonIndentOptions = []jsonIndentOption{
	{key: "2", label: "2 空格 (推荐)", short: "缩进: 2 空格", pad: "  ", hint: "2 spaces"},
	{key: "4", label: "4 空格", short: "缩进: 4 空格", pad: "    ", hint: "4 spaces"},
	{key: "tab", label: "制表符 Tab", short: "缩进: Tab", pad: "\t", hint: `\t`},
}

type jsonToolState struct {
	inputEd   *editor.State
	outputEd  *editor.State
	mode      int
	indentIdx int
	sortKeys  bool
	lastSig   string
	errInfo   *devtools.JSONErrorInfo
	copied    bool
	editing   bool // 点击空白输入区直接进入编辑器
}

func renderJSONTool(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-json-root")).Grow(1).FillWidth()
	st := *ui.Local(root, "json-state-v2", func() *jsonToolState {
		inBuf := editor.NewBuffer("")
		inBuf.SetLanguage(editor.LanguageNamed("JSON"))
		outBuf := editor.NewBuffer("")
		outBuf.SetLanguage(editor.LanguageNamed("JSON"))
		return &jsonToolState{
			inputEd: &editor.State{
				Buffer:      inBuf,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOn,
			},
			outputEd: &editor.State{
				Buffer:      outBuf,
				ReadOnly:    true,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOn,
			},
		}
	})

	// 实时联动计算
	recompute := func(force bool) {
		raw := st.inputEd.Buffer.String()
		ind := jsonIndentOptions[st.indentIdx]
		sig := fmt.Sprintf("%d|%d|%v|%s", st.mode, st.indentIdx, st.sortKeys, raw)
		if !force && sig == st.lastSig {
			return
		}
		st.lastSig = sig
		st.copied = false

		if strings.TrimSpace(raw) == "" {
			st.errInfo = nil
			st.outputEd.Buffer.Reset("")
			return
		}

		var (
			out string
			err error
		)
		switch st.mode {
		case jsonModeFormat:
			out, err = devtools.FormatJSONWithPad(raw, ind.pad, st.sortKeys)
		case jsonModeMinify:
			out, err = devtools.MinifyJSONSorted(raw, st.sortKeys)
		case jsonModeEscape:
			out = devtools.EscapeJSON(raw)
		case jsonModeUnescape:
			out, err = devtools.UnescapeJSON(raw)
		}
		if err != nil {
			st.errInfo = devtools.ExtractJSONError(err)
			return
		}
		st.errInfo = nil
		st.outputEd.Buffer.Reset(out)
	}
	recompute(false)

	rawInput := st.inputEd.Buffer.String()
	rawOutput := st.outputEd.Buffer.String()
	isEmpty := strings.TrimSpace(rawInput) == ""
	inBytes := len([]byte(rawInput))
	outBytes := len([]byte(rawOutput))
	inLines := countLines(rawInput)
	outLines := countLines(rawOutput)
	indOpt := jsonIndentOptions[st.indentIdx]

	root.Children(func() {
		// 1. 顶部标题栏 (48px)
		toolHeader(c, icons.Code, "JSON 格式化与转义", "校验语法 · 美化缩进 · 压缩排序 · 字符串转义", func() {
			if toolBtn(c, toolBtnOpts{Key: "json-sample", Label: "载入示例", Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset(sampleJSONText)
				st.editing = true
				recompute(true)
			}
			if toolBtn(c, toolBtnOpts{Key: "json-clear", Label: "清空", Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset("")
				st.outputEd.Buffer.Reset("")
				st.errInfo = nil
				st.copied = false
				st.editing = false
				st.lastSig = ""
			}
		})

		// 2. 操作工具条 (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "json-mode", []string{"格式化", "压缩", "转义", "去转义"}, st.mode, func(idx int) {
				st.mode = idx
				recompute(true)
			})
			toolDividerV(c)

			// 缩进下拉按钮 + 浮层
			indentBtn := toolBtn(c, toolBtnOpts{
				Key:     "json-indent-btn",
				Label:   indOpt.short,
				Icon:    icons.List,
				Variant: btnSecondary,
				Chevron: true,
			})
			indentOpen := ui.Local(indentBtn, "json-indent-open", func() bool { return false })
			if indentBtn.Clicked() {
				*indentOpen = !*indentOpen
			}
			if *indentOpen {
				indentBtn.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.35))
			}
			v2.Popover(c, indentBtn, indentOpen, v2.PopoverOpts{Width: 180}, func(panel ui.Element) {
				ui.Column(c).Padding(4).Gap(2).Children(func() {
					for i, opt := range jsonIndentOptions {
						idx, o := i, opt
						on := idx == st.indentIdx
						row := ui.ButtonBase(c.Key("json-ind-opt-"+o.key)).
							FillWidth().Height(30).Padding(0, 8).Gap(8).
							Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.SpaceBetween).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(o.label)
						if on {
							row.Background(tk.AccentSoft)
						} else if row.Hovered() {
							row.Background(tk.BgHover)
						}
						if row.Clicked() {
							st.indentIdx = idx
							*indentOpen = false
							recompute(true)
						}
						row.Children(func() {
							fg := tk.TextPrimary
							w := 400
							if on {
								fg = tk.Accent
								w = 500
							}
							ui.Text(c, o.label).FontSize(m.FontSM).FontWeight(w).TextColor(fg).SingleLine()
							if on {
								ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.Accent)
							} else {
								ui.Text(c, o.hint).Font("monospace").FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
							}
						})
					}
				})
			})

			sortVar := btnSecondary
			if st.sortKeys {
				sortVar = btnActive
			}
			if toolBtn(c, toolBtnOpts{Key: "json-sort", Label: "Key 排序 A→Z", Icon: icons.ListOrdered, Variant: sortVar}).Clicked() {
				st.sortKeys = !st.sortKeys
				recompute(true)
			}
		}, func() {
			if toolBtn(c, toolBtnOpts{Key: "json-swap", Label: "结果回填", Icon: icons.ArrowLeft, Variant: btnSecondary, Disabled: rawOutput == ""}).Clicked() {
				if rawOutput != "" {
					st.inputEd.Buffer.Reset(rawOutput)
					st.editing = true
					recompute(true)
				}
			}
			copyVar := btnPrimary
			copyLbl := "复制结果"
			copyIc := icons.Copy
			if st.copied {
				copyVar = btnSuccess
				copyLbl = "已复制"
				copyIc = icons.Check
			}
			if toolBtn(c, toolBtnOpts{Key: "json-copy", Label: copyLbl, Icon: copyIc, Variant: copyVar, Disabled: rawOutput == ""}).Clicked() {
				if rawOutput != "" {
					c.Services().WriteClipboard(rawOutput)
					st.copied = true
				}
			}
		})

		// 3. 左右 50/50 分栏主体工作区
		ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
			// 左栏：INPUT
			ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
				inMeta := "0 B"
				if !isEmpty {
					inMeta = fmt.Sprintf("%d B · %d lines", inBytes, inLines)
				}
				toolPaneHeader(c, "INPUT", inMeta, func() {
					if isEmpty {
						toolPill(c, "等待输入", pillNeutral, false)
					} else if st.errInfo != nil {
						toolPill(c, "1 处语法错误", pillDanger, true)
					} else {
						toolPill(c, "UTF-8", pillNeutral, false)
					}
				})

				if isEmpty && !st.editing {
					// 精美空态引导页
					ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(10).Background(tk.BgBase).Children(func() {
						ui.Box(c).Size(44, 44).Radius(10).Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.25)).Center().Children(func() {
							ui.Icon(c, icons.CodeBoldDuotone).FontSize(22).TextColor(tk.Accent)
						})
						ui.Text(c, "粘贴或输入 JSON 文本").FontSize(13).FontWeight(500).TextColor(tk.TextPrimary)
						ui.Text(c, "支持大体积 JSON 实时校验、自动格式化与字符串转义").FontSize(m.FontSM).TextColor(tk.TextMuted)
						ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
							if toolBtn(c, toolBtnOpts{Key: "json-empty-paste", Label: "从剪贴板粘贴", Icon: icons.Copy, Variant: btnPrimary}).Clicked() {
								clip := c.Services().ReadClipboard()
								st.editing = true
								if strings.TrimSpace(clip) != "" {
									st.inputEd.Buffer.Reset(clip)
									recompute(true)
								}
							}
							if toolBtn(c, toolBtnOpts{Key: "json-empty-sample", Label: "载入示例数据", Icon: icons.Sparkles, Variant: btnSecondary}).Clicked() {
								st.inputEd.Buffer.Reset(sampleJSONText)
								st.editing = true
								recompute(true)
							}
							if toolBtn(c, toolBtnOpts{Key: "json-empty-edit", Label: "手动输入", Variant: btnGhost}).Clicked() {
								st.editing = true
							}
						})
					})
				} else {
					ui.Column(c).Grow(1).FillWidth().Children(func() {
						ui.Box(c.Key("json-in-ed")).Grow(1).FillWidth().Background(tk.BgBase).Clip().Children(func() {
							editor.View(c, st.inputEd).Fill()
						})
						if st.errInfo != nil {
							ui.Box(c).FillWidth().Padding(8, 12, 12, 12).Children(func() {
								actionLbl := ""
								if st.errInfo.Line > 0 {
									actionLbl = fmt.Sprintf("第 %d 行", st.errInfo.Line)
								}
								toolInlineErrorCard(c, "json-err-card", st.errInfo.Summary, st.errInfo.Detail, actionLbl, func() {
									// 点击按钮载入合法示例或保持提示
								})
							})
						}
					})
				}
			})

			// 中间 1px 竖分割线
			ui.Box(c).Width(1).FillHeight().Background(tk.Border)

			// 右栏：OUTPUT
			ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
				outMeta := "0 B"
				if st.errInfo != nil {
					outMeta = "校验失败"
				} else if rawOutput != "" {
					if st.mode == jsonModeMinify && inBytes > 0 {
						pct := int(float64(inBytes-outBytes) / float64(inBytes) * 100)
						outMeta = fmt.Sprintf("%d B · -%d%%", outBytes, pct)
					} else if st.mode == jsonModeEscape {
						outMeta = fmt.Sprintf("%d chars · Escaped", len([]rune(rawOutput)))
					} else {
						outMeta = fmt.Sprintf("%d B · %d lines", outBytes, outLines)
					}
				}

				toolPaneHeader(c, "OUTPUT", outMeta, func() {
					if st.errInfo != nil {
						toolPill(c, "已暂停更新", pillNeutral, false)
					} else if rawOutput != "" {
						switch st.mode {
						case jsonModeMinify:
							lbl := "已压缩"
							if st.sortKeys {
								lbl = "已压缩 · Key 已排序"
							}
							toolPill(c, lbl, pillAccent, true)
						case jsonModeEscape:
							toolPill(c, "已转义字符串", pillAccent, true)
						case jsonModeUnescape:
							toolPill(c, "已去转义", pillSuccess, true)
						default:
							toolPill(c, "Valid JSON", pillSuccess, true)
						}
					}
				})

				switch {
				case st.errInfo != nil:
					ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(10).Background(tk.BgApp).Children(func() {
						ui.Box(c).Size(40, 40).Radius(10).Background(tk.DangerSoft).Border(m.Border, tk.Danger.Alpha(0.35)).Center().Children(func() {
							ui.Icon(c, icons.TriangleAlert).FontSize(18).TextColor(tk.Danger)
						})
						ui.Text(c, "无法生成格式化输出").FontSize(13).FontWeight(500).TextColor(tk.TextPrimary)
						ui.Text(c, "修复左侧语法错误后，输出将自动恢复实时同步").FontSize(m.FontSM).TextColor(tk.TextMuted)
					})
				case rawOutput == "":
					ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(8).Background(tk.BgApp).Children(func() {
						ui.Text(c, "格式化结果将在此实时呈现").FontSize(m.FontSM).TextColor(tk.TextMuted)
						ui.Text(c, "支持 2/4 空格与 Tab 缩进 · Key 深度排序 · 字符串转义").FontSize(m.FontXS).TextColor(tk.TextDisabled)
					})
				default:
					ui.Box(c.Key("json-out-ed")).Grow(1).FillWidth().Background(tk.BgBase).Clip().Children(func() {
						editor.View(c, st.outputEd).Fill()
					})
				}
			})
		})

		// 4. 底部状态条 (26px)
		stText := "就绪 · 等待输入"
		stCol := tk.TextMuted
		if st.errInfo != nil {
			stText = "校验失败 · " + st.errInfo.Summary
			stCol = tk.Danger
		} else if st.copied {
			stText = fmt.Sprintf("已复制 %d 字节到剪贴板", outBytes)
			stCol = tk.Success
		} else if rawOutput != "" {
			stText = "JSON 语法有效 · RFC 8259"
			stCol = tk.Success
		}
		toolFooter(c, stText, stCol, indOpt.short, "编码: UTF-8")
	})
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

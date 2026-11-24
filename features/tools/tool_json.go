package tools

import (
	"fmt"
	"strconv"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/devtools"
	"rddpostman/locales"

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

func getJSONIndentOptions() []jsonIndentOption {
	return []jsonIndentOption{
		{key: "2", label: locales.JsonIndent2Label(), short: locales.JsonIndent2Short(), pad: "  ", hint: "2 spaces"},
		{key: "4", label: locales.JsonIndent4Label(), short: locales.JsonIndent4Short(), pad: "    ", hint: "4 spaces"},
		{key: "tab", label: locales.JsonIndentTabLabel(), short: locales.JsonIndentTabShort(), pad: "\t", hint: `\t`},
	}
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
				Label:       "JSON input",
			},
			outputEd: &editor.State{
				Buffer:      outBuf,
				ReadOnly:    true,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOn,
			},
		}
	})
	st.inputEd.Placeholder = locales.JsonEmptyTitle() + " · " + locales.JsonEmptyDesc()

	indentOpts := getJSONIndentOptions()

	// 实时联动计算
	recompute := func(force bool) {
		raw := st.inputEd.Buffer.String()
		ind := indentOpts[st.indentIdx]
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
	indOpt := indentOpts[st.indentIdx]

	root.Children(func() {
		// 1. 顶部标题栏 (48px)
		toolHeader(c, icons.Code, locales.ToolJSONTitle(), locales.ToolJSONDesc(), func() {
			if toolBtn(c, toolBtnOpts{Key: "json-sample", Label: locales.LoadSample(), Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset(sampleJSONText)
				recompute(true)
			}
			if toolBtn(c, toolBtnOpts{Key: "json-clear", Label: locales.Clear(), Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset("")
				st.outputEd.Buffer.Reset("")
				st.errInfo = nil
				st.copied = false
				st.lastSig = ""
			}
		})

		// 2. 操作工具条 (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "json-mode", []string{locales.JsonModeFormat(), locales.JsonModeMinify(), locales.JsonModeEscape(), locales.JsonModeUnescape()}, st.mode, func(idx int) {
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
					for i, opt := range indentOpts {
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
			if toolBtn(c, toolBtnOpts{Key: "json-sort", Label: locales.JsonSortKeys(), Icon: icons.ListOrdered, Variant: sortVar}).Clicked() {
				st.sortKeys = !st.sortKeys
				recompute(true)
			}
		}, func() {
			if toolBtn(c, toolBtnOpts{Key: "json-swap", Label: locales.JsonUseOutput(), Icon: icons.ArrowLeft, Variant: btnSecondary, Disabled: rawOutput == ""}).Clicked() {
				if rawOutput != "" {
					st.inputEd.Buffer.Reset(rawOutput)
					recompute(true)
				}
			}
			copyVar := btnPrimary
			copyLbl := locales.JsonCopyResult()
			copyIc := icons.Copy
			if st.copied {
				copyVar = btnSuccess
				copyLbl = locales.Copied()
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
				inMeta := "0 B · UTF-8"
				if !isEmpty {
					inMeta = fmt.Sprintf("%s · %d lines · UTF-8", formatByteSize(inBytes), inLines)
				}
				toolPaneHeader(c, "INPUT", inMeta, func() {
					if toolBtn(c, toolBtnOpts{Key: "json-in-paste", Label: locales.PasteFromClipboard(), Icon: icons.Copy, Variant: btnGhost, Height: 22, PadX: 7}).Clicked() {
						if clip := c.Services().ReadClipboard(); strings.TrimSpace(clip) != "" {
							st.inputEd.Buffer.Reset(clip)
							recompute(true)
						}
					}
					if !isEmpty && st.errInfo != nil {
						toolPill(c, locales.JsonSyntaxErrCount(), pillDanger, true)
					}
				})

				ui.Column(c).Grow(1).FillWidth().Children(func() {
					ui.Box(c.Key("json-in-ed")).Grow(1).FillWidth().Background(tk.BgBase).Clip().Children(func() {
						editor.View(c, st.inputEd).Fill()
					})
					if st.errInfo != nil {
						ui.Box(c).FillWidth().Padding(8, 12, 12, 12).Children(func() {
							actionLbl := ""
							if st.errInfo.Line > 0 {
								actionLbl = locales.JsonLineBadge(strconv.Itoa(st.errInfo.Line))
							}
							toolInlineErrorCard(c, "json-err-card", st.errInfo.Summary, st.errInfo.Detail, actionLbl, func() {
								// 点击按钮载入合法示例或保持提示
							})
						})
					}
				})
			})

			// 中间 1px 竖分割线
			ui.Box(c).Width(1).FillHeight().Background(tk.Border)

			// 右栏：OUTPUT
			ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
				outMeta := "0 B"
				if st.errInfo != nil {
					outMeta = locales.JsonValidationFailed()
				} else if rawOutput != "" {
					if st.mode == jsonModeMinify && inBytes > 0 {
						pct := int(float64(inBytes-outBytes) / float64(inBytes) * 100)
						outMeta = fmt.Sprintf("%s · -%d%%", formatByteSize(outBytes), pct)
					} else if st.mode == jsonModeEscape {
						outMeta = fmt.Sprintf("%d chars · Escaped", len([]rune(rawOutput)))
					} else {
						outMeta = fmt.Sprintf("%s · %d lines", formatByteSize(outBytes), outLines)
					}
				}

				toolPaneHeader(c, "OUTPUT", outMeta, func() {
					if st.errInfo != nil {
						toolPill(c, locales.JsonPaused(), pillNeutral, false)
					} else if rawOutput != "" {
						switch st.mode {
						case jsonModeMinify:
							lbl := locales.JsonMinified()
							if st.sortKeys {
								lbl = locales.JsonMinifiedSorted()
							}
							toolPill(c, lbl, pillAccent, true)
						case jsonModeEscape:
							toolPill(c, locales.JsonEscaped(), pillAccent, true)
						case jsonModeUnescape:
							toolPill(c, locales.JsonUnescaped(), pillSuccess, true)
						default:
							toolPill(c, locales.JsonValid(), pillSuccess, true)
						}
					}
				})

				switch {
				case st.errInfo != nil:
					ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(10).Background(tk.BgApp).Children(func() {
						ui.Box(c).Size(40, 40).Radius(10).Background(tk.DangerSoft).Border(m.Border, tk.Danger.Alpha(0.35)).Center().Children(func() {
							ui.Icon(c, icons.TriangleAlert).FontSize(18).TextColor(tk.Danger)
						})
						ui.Text(c, locales.JsonOutputErrTitle()).FontSize(13).FontWeight(500).TextColor(tk.TextPrimary)
						ui.Text(c, locales.JsonOutputErrDesc()).FontSize(m.FontSM).TextColor(tk.TextMuted)
					})
				case rawOutput == "":
					ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(8).Background(tk.BgApp).Children(func() {
						ui.Text(c, locales.JsonOutputEmptyTitle()).FontSize(m.FontSM).TextColor(tk.TextMuted)
						ui.Text(c, locales.JsonOutputEmptyDesc()).FontSize(m.FontXS).TextColor(tk.TextDisabled)
					})
				default:
					ui.Box(c.Key("json-out-ed")).Grow(1).FillWidth().Background(tk.BgBase).Clip().Children(func() {
						editor.View(c, st.outputEd).Fill()
					})
				}
			})
		})
	})
}

func countLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

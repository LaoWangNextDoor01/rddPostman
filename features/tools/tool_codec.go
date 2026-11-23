package tools

import (
	"fmt"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/devtools"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const (
	codecModeBase64  = 0
	codecModeURL     = 1
	codecModeUnicode = 2
)

type codecToolState struct {
	mode         int  // 0=Base64, 1=URL, 2=Unicode
	decodeDir    bool // false=Encode, true=Decode
	urlSafe      bool
	omitPadding  bool
	urlFullURI   bool
	nonASCIIOnly bool
	inputEd      *editor.State
	outputEd     *editor.State
	lastSig      string
	errMsg       string
	copied       bool
	copiedParam  string
	editing      bool
}

func sampleForCodec(mode int, decode bool) string {
	switch mode {
	case codecModeURL:
		if decode {
			return "https%3A%2F%2Fapi.example.com%2Fv1%2Fsearch%3Fq%3D%E6%8E%A5%E5%8F%A3%E8%B0%83%E8%AF%95%20%E5%B7%A5%E5%85%B7%26env%3D%E7%94%9F%E4%BA%A7%E7%8E%AF%E5%A2%83%26tag%3Da%2Bb"
		}
		return "https://api.example.com/v1/search?q=接口调试 工具&env=生产环境&tag=a+b"
	case codecModeUnicode:
		if decode {
			return `\u7814\u53d1\u5de5\u4f5c\u53f0 API \u8c03\u8bd5\u52a9\u624b v2.4`
		}
		return "研发工作台 · API 调试助手 v2.4"
	default:
		if decode {
			return "eyJ1aWQiOiAxMDA4NiwgInJvbGUiOiAiYWRtaW4iLCAid29ya3NwYWNlIjogIueglOWPkeW3peS9nOWPsCJ9"
		}
		return `{"uid": 10086, "role": "admin", "workspace": "研发工作台"}`
	}
}

func renderCodecTool(c *ui.Context, keySuffix string, initialMode int) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-codec-root-" + keySuffix)).Grow(1).FillWidth()
	st := *ui.Local(root, "codec-state-v2", func() *codecToolState {
		return &codecToolState{
			mode:         initialMode,
			nonASCIIOnly: true,
			inputEd: &editor.State{
				Buffer:      editor.NewBuffer(""),
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOff,
			},
			outputEd: &editor.State{
				Buffer:      editor.NewBuffer(""),
				ReadOnly:    true,
				SoftWrap:    true,
				LineNumbers: editor.LineNumbersOff,
			},
		}
	})

	recompute := func(force bool) {
		raw := st.inputEd.Buffer.String()
		sig := fmt.Sprintf("%d|%v|%v|%v|%v|%v|%s",
			st.mode, st.decodeDir, st.urlSafe, st.omitPadding, st.urlFullURI, st.nonASCIIOnly, raw)
		if !force && sig == st.lastSig {
			return
		}
		st.lastSig = sig
		st.copied = false

		if raw == "" {
			st.errMsg = ""
			st.outputEd.Buffer.Reset("")
			return
		}

		var (
			out string
			err error
		)
		switch st.mode {
		case codecModeBase64:
			if st.decodeDir {
				out, err = devtools.Base64Decode(raw, st.urlSafe)
			} else {
				out = devtools.Base64EncodeOpts(raw, st.urlSafe, st.omitPadding)
			}
		case codecModeURL:
			if st.decodeDir {
				out, err = devtools.URLDecode(raw)
			} else {
				out = devtools.URLEncodeMode(raw, st.urlFullURI)
			}
		case codecModeUnicode:
			if st.decodeDir {
				out, err = devtools.UnicodeUnescape(raw)
			} else {
				out = devtools.UnicodeEscapeOpts(raw, st.nonASCIIOnly)
			}
		}
		if err != nil {
			st.errMsg = err.Error()
			return
		}
		st.errMsg = ""
		st.outputEd.Buffer.Reset(out)
	}
	recompute(false)

	rawIn := st.inputEd.Buffer.String()
	rawOut := st.outputEd.Buffer.String()
	isEmpty := rawIn == ""
	inBytes := len([]byte(rawIn))
	outRunes := len([]rune(rawOut))
	dirIdx := 0
	if st.decodeDir {
		dirIdx = 1
	}

	var queryPreview []devtools.QueryParamPreview
	if st.mode == codecModeURL && rawIn != "" {
		srcForQuery := rawIn
		if st.decodeDir && rawOut != "" {
			srcForQuery = rawOut
		}
		queryPreview = devtools.ParseQueryPreview(srcForQuery)
	}

	root.Children(func() {
		// 1. Header (48px)
		toolHeader(c, icons.Link, "编解码工作台", "Base64 (Standard / URL-Safe) · URL Query/Path · Unicode 转义", func() {
			if toolBtn(c, toolBtnOpts{Key: "codec-sample", Label: "载入示例", Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset(sampleForCodec(st.mode, st.decodeDir))
				st.editing = true
				recompute(true)
			}
			if toolBtn(c, toolBtnOpts{Key: "codec-clear", Label: "清空", Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.inputEd.Buffer.Reset("")
				st.outputEd.Buffer.Reset("")
				st.errMsg = ""
				st.copied = false
				st.editing = false
				st.lastSig = ""
			}
		})

		// 2. Toolbar (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "codec-mode", []string{"Base64", "URL", "Unicode"}, st.mode, func(idx int) {
				st.mode = idx
				recompute(true)
			})
			toolDividerV(c)
			toolSegmented(c, "codec-dir", []string{"编码 Encode", "解码 Decode"}, dirIdx, func(idx int) {
				st.decodeDir = idx == 1
				recompute(true)
			})

			switch st.mode {
			case codecModeBase64:
				usVar := btnSecondary
				if st.urlSafe {
					usVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "codec-urlsafe", Label: "URL-Safe (- / _)", Variant: usVar}).Clicked() {
					st.urlSafe = !st.urlSafe
					recompute(true)
				}
				opVar := btnSecondary
				if st.omitPadding {
					opVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "codec-nopad", Label: "省略 =", Variant: opVar}).Clicked() {
					st.omitPadding = !st.omitPadding
					recompute(true)
				}
			case codecModeURL:
				uIdx := 0
				if st.urlFullURI {
					uIdx = 1
				}
				toolSegmented(c, "codec-url-scope", []string{"Query 参数", "完整 URI"}, uIdx, func(idx int) {
					st.urlFullURI = idx == 1
					recompute(true)
				})
			case codecModeUnicode:
				naVar := btnSecondary
				if st.nonASCIIOnly {
					naVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "codec-nonascii", Label: "仅转义非 ASCII", Variant: naVar}).Clicked() {
					st.nonASCIIOnly = !st.nonASCIIOnly
					recompute(true)
				}
			}
		}, func() {
			if toolBtn(c, toolBtnOpts{Key: "codec-swap", Label: "互换方向", Icon: icons.ArrowLeft, Variant: btnSecondary, Disabled: rawOut == ""}).Clicked() {
				if rawOut != "" {
					st.decodeDir = !st.decodeDir
					st.inputEd.Buffer.Reset(rawOut)
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
			if toolBtn(c, toolBtnOpts{Key: "codec-copy", Label: copyLbl, Icon: copyIc, Variant: copyVar, Disabled: rawOut == ""}).Clicked() {
				if rawOut != "" {
					c.Services().WriteClipboard(rawOut)
					st.copied = true
				}
			}
		})

		// 3. 主体区（双栏 + URL 模式底部 Query 参数预览表）
		ui.Column(c).Grow(1).FillWidth().Children(func() {
			inTitle, outTitle := "PLAIN TEXT", "BASE64 OUTPUT"
			switch st.mode {
			case codecModeBase64:
				if st.decodeDir {
					inTitle, outTitle = "BASE64 INPUT", "DECODED TEXT"
				} else if st.urlSafe {
					outTitle = "URL-SAFE BASE64"
				}
			case codecModeURL:
				if st.decodeDir {
					inTitle, outTitle = "PERCENT ENCODED", "DECODED URL / QUERY"
				} else {
					inTitle, outTitle = "RAW URL / QUERY", "PERCENT ENCODED"
				}
			case codecModeUnicode:
				if st.decodeDir {
					inTitle, outTitle = `UNICODE ESCAPE (\uXXXX)`, "DECODED UTF-8"
				} else {
					inTitle, outTitle = "TEXT INPUT", `UNICODE ESCAPE (\uXXXX)`
				}
			}

			ui.Row(c).Grow(1).FillWidth().AlignItems(ui.Stretch).Children(func() {
				// 左栏
				ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
					inMeta := "0 B"
					if !isEmpty {
						inMeta = fmt.Sprintf("%d B · %d chars", inBytes, len([]rune(rawIn)))
					}
					toolPaneHeader(c, inTitle, inMeta, func() {
						if isEmpty {
							toolPill(c, "等待输入", pillNeutral, false)
						} else if st.errMsg != "" {
							toolPill(c, "包含非法字符", pillDanger, true)
						} else {
							toolPill(c, "UTF-8", pillNeutral, false)
						}
					})

					if isEmpty && !st.editing {
						ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(10).Background(tk.BgBase).Children(func() {
							ui.Box(c).Size(44, 44).Radius(10).Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.25)).Center().Children(func() {
								ui.Icon(c, icons.LinkBoldDuotone).FontSize(22).TextColor(tk.Accent)
							})
							ui.Text(c, "输入或粘贴需要编解码的文本").FontSize(13).FontWeight(500).TextColor(tk.TextPrimary)
							ui.Text(c, "输入时自动实时转换，支持 UTF-8 中文、URL-Safe 与 Unicode 序列").FontSize(m.FontSM).TextColor(tk.TextMuted)
							ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
								if toolBtn(c, toolBtnOpts{Key: "codec-empty-paste", Label: "从剪贴板粘贴", Icon: icons.Copy, Variant: btnPrimary}).Clicked() {
									clip := c.Services().ReadClipboard()
									st.editing = true
									if clip != "" {
										st.inputEd.Buffer.Reset(clip)
										recompute(true)
									}
								}
								if toolBtn(c, toolBtnOpts{Key: "codec-empty-sample", Label: "载入示例", Icon: icons.Sparkles, Variant: btnSecondary}).Clicked() {
									st.inputEd.Buffer.Reset(sampleForCodec(st.mode, st.decodeDir))
									st.editing = true
									recompute(true)
								}
								if toolBtn(c, toolBtnOpts{Key: "codec-empty-edit", Label: "手动输入", Variant: btnGhost}).Clicked() {
									st.editing = true
								}
							})
						})
					} else {
						ui.Column(c).Grow(1).FillWidth().Children(func() {
							ui.Box(c.Key("codec-in-ed")).Grow(1).FillWidth().Padding(8, 12).Background(tk.BgBase).Clip().Children(func() {
								editor.View(c, st.inputEd).Fill()
							})
							if st.errMsg != "" {
								ui.Box(c).FillWidth().Padding(8, 12, 12, 12).Children(func() {
									act := ""
									var actFn func()
									if st.mode == codecModeBase64 && st.decodeDir {
										act = "自动清理"
										actFn = func() {
											cleaned := devtools.CleanBase64(st.inputEd.Buffer.String())
											st.inputEd.Buffer.Reset(cleaned)
											recompute(true)
										}
									}
									toolInlineErrorCard(c, "codec-err-card", st.errMsg,
										"标准 Base64 仅允许 A-Z、a-z、0-9、+、/ 及末尾填充符 =", act, actFn)
								})
							}
						})
					}
				})

				ui.Box(c).Width(1).FillHeight().Background(tk.Border)

				// 右栏
				ui.Column(c).Grow(1).Basis(0).FillHeight().Children(func() {
					outMeta := "0 chars"
					if st.errMsg != "" {
						outMeta = "解码失败"
					} else if rawOut != "" {
						if st.mode == codecModeBase64 && !st.decodeDir && inBytes > 0 {
							delta := int(float64(len(rawOut)-inBytes) / float64(inBytes) * 100)
							outMeta = fmt.Sprintf("%d chars · +%d%%", outRunes, delta)
						} else {
							outMeta = fmt.Sprintf("%d chars", outRunes)
						}
					}
					toolPaneHeader(c, outTitle, outMeta, func() {
						if st.errMsg != "" {
							toolPill(c, "已停止", pillNeutral, false)
						} else if rawOut != "" {
							switch {
							case st.mode == codecModeBase64 && st.urlSafe && !st.decodeDir:
								toolPill(c, "RFC 4648 §5", pillAccent, true)
							case st.mode == codecModeURL && !st.decodeDir:
								toolPill(c, "RFC 3986", pillSuccess, true)
							case st.copied:
								toolPill(c, "已复制", pillSuccess, true)
							default:
								lbl := "已编码"
								if st.decodeDir {
									lbl = "已解码"
								}
								toolPill(c, lbl, pillSuccess, true)
							}
						}
					})

					switch {
					case st.errMsg != "":
						ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(10).Background(tk.BgApp).Children(func() {
							ui.Box(c).Size(40, 40).Radius(10).Background(tk.DangerSoft).Border(m.Border, tk.Danger.Alpha(0.35)).Center().Children(func() {
								ui.Icon(c, icons.TriangleAlert).FontSize(18).TextColor(tk.Danger)
							})
							ui.Text(c, "解码失败").FontSize(13).FontWeight(500).TextColor(tk.TextPrimary)
							ui.Text(c, "请修正左侧非法字符，或点击“自动清理”跳过非法符号").FontSize(m.FontSM).TextColor(tk.TextMuted)
						})
					case rawOut == "":
						ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space6).Gap(8).Background(tk.BgApp).Children(func() {
							ui.Text(c, "转换结果将在此处实时生成").FontSize(m.FontSM).TextColor(tk.TextMuted)
							ui.Text(c, "支持 Base64 / URL / Unicode 三模式无缝切换").FontSize(m.FontXS).TextColor(tk.TextDisabled)
						})
					default:
						ui.Column(c).Grow(1).FillWidth().Children(func() {
							ui.Box(c.Key("codec-out-ed")).Grow(1).FillWidth().Padding(8, 12).Background(tk.BgBase).Clip().Children(func() {
								editor.View(c, st.outputEd).Fill()
							})
							// Base64 URL-Safe 替换摘要卡片
							if st.mode == codecModeBase64 && !st.decodeDir && (st.urlSafe || st.omitPadding) {
								diff := devtools.Base64DiffStats(rawIn, st.urlSafe, st.omitPadding)
								ui.Box(c).FillWidth().Padding(0, 12, 12, 12).Children(func() {
									ui.Column(c).FillWidth().Padding(10, 12).Gap(8).Radius(m.RadiusMD).
										Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.3)).Children(func() {
										ui.Text(c, "URL-Safe 安全替换摘要").FontSize(m.FontSM).FontWeight(600).TextColor(tk.Accent)
										ui.Row(c).Gap(8).Children(func() {
											toolPill(c, fmt.Sprintf("'+' → '-' (%d 处)", diff.PlusReplaced), pillNeutral, false)
											toolPill(c, fmt.Sprintf("'/' → '_' (%d 处)", diff.SlashReplaced), pillAccent, false)
											if st.omitPadding {
												toolPill(c, fmt.Sprintf("已省略 %d 个 '=' 填充", diff.PadOmitted), pillSuccess, false)
											}
										})
									})
								})
							}
						})
					}
				})
			})

			// URL Query Params 自动解析预览表（仅 URL 模式且解析出参数时展示）
			if len(queryPreview) > 0 {
				ui.Box(c).FillWidth().Height(1).Background(tk.Border)
				ui.Column(c).FillWidth().MaxHeight(196).Background(tk.BgBase).Children(func() {
					ui.Row(c).FillWidth().Height(32).Padding(0, 16).AlignItems(ui.Center).Justify(ui.SpaceBetween).
						Background(tk.BgApp).BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
						ui.Text(c, fmt.Sprintf("QUERY PARAMS 自动解析预览 (%d 项)", len(queryPreview))).
							FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						ui.Text(c, "点击任意行复制参数值").FontSize(m.FontXS).TextColor(tk.TextMuted)
					})
					ui.Scroll(c).FillWidth().MaxHeight(164).Children(func() {
						ui.Column(c).FillWidth().Children(func() {
							for i, qp := range queryPreview {
								idx, item := i, qp
								rowKey := fmt.Sprintf("qp-%d-%s", idx, item.Key)
								isCopied := st.copiedParam == rowKey
								row := ui.ButtonBase(c.Key(rowKey)).FillWidth().Height(36).Padding(0, 16).Gap(16).
									AlignItems(ui.Center).Justify(ui.Start).Cursor(ui.CursorPointer).
									Role(ui.RoleButton).Label("Copy param " + item.Key)
								if idx > 0 {
									row.BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)
								}
								if isCopied {
									row.Background(tk.Success.Alpha(0.1))
								} else if row.Hovered() {
									row.Background(tk.BgHover)
								}
								if row.Clicked() {
									c.Services().WriteClipboard(item.RawValue)
									st.copiedParam = rowKey
								}
								row.Children(func() {
									ui.Text(c, item.Key).Font("monospace").FontSize(m.FontSM).FontWeight(500).
										TextColor(tk.Accent).Width(120).SingleLine()
									ui.Text(c, item.RawValue).FontSize(m.FontSM).TextColor(tk.TextPrimary).Width(220).SingleLine()
									ui.Text(c, item.EncodedValue).Font("monospace").FontSize(m.FontXS).
										TextColor(tk.TextSecondary).SingleLine().Grow(1)
									if isCopied {
										ui.Text(c, "已复制 ✓").FontSize(m.FontXS).TextColor(tk.Success).SingleLine()
									}
								})
							}
						})
					})
				})
			}
		})

		// 4. Footer (26px)
		fTxt := "就绪 · 实时双向编解码"
		fCol := tk.TextMuted
		if st.errMsg != "" {
			fTxt = "解码失败 · " + st.errMsg
			fCol = tk.Danger
		} else if st.copied {
			fTxt = fmt.Sprintf("已复制 %d 个字符到剪贴板", outRunes)
			fCol = tk.Success
		} else if rawOut != "" {
			fTxt = "转换成功 · UTF-8 字节精准对齐"
			fCol = tk.Success
		}
		toolFooter(c, fTxt, fCol, "字符编码: UTF-8")
	})
}

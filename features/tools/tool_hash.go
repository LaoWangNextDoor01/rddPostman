package tools

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/devtools"

	"github.com/egoist/mygo/ui"
)

const sampleHashInput = "orderId=ORD20261123&amount=19800&currency=CNY&ts=1763884855"
const sampleHMACSecret = "sk_live_9f2a8c4e1b7d03aa6c1f"

type hashToolState struct {
	mode        int // 0=Hash, 1=HMAC
	upperHex    bool
	compareOpen bool
	compareVal  string
	input       string
	secret      string
	showSecret  bool
	hmacAlgoIdx int // 0=md5, 1=sha1, 2=sha256, 3=sha512
	hmacBase64  bool
	copiedKey   string
}

var hmacAlgos = []struct {
	id    string
	label string
}{
	{"md5", "MD5"},
	{"sha1", "SHA-1"},
	{"sha256", "SHA-256"},
	{"sha512", "SHA-512"},
}

func renderHashTool(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-hash-root")).Grow(1).FillWidth()
	st := *ui.Local(root, "hash-state-v2", func() *hashToolState {
		return &hashToolState{hmacAlgoIdx: 2} // default SHA-256
	})

	isHmac := st.mode == 1
	summary := devtools.ComputeHashes(st.input)
	algo := hmacAlgos[st.hmacAlgoIdx]
	hmacFmt := "hex"
	if st.hmacBase64 {
		hmacFmt = "base64"
	}
	hmacOut, _ := devtools.ComputeHMAC(algo.id, st.secret, st.input, hmacFmt)
	if !st.hmacBase64 && st.upperHex {
		hmacOut = strings.ToUpper(hmacOut)
	}

	matchedAlg := ""
	hasMatch := false
	if !isHmac && st.compareOpen && st.input != "" && strings.TrimSpace(st.compareVal) != "" {
		matchedAlg, hasMatch = devtools.MatchHash(summary, st.compareVal)
	}

	fmtHex := func(s string) string {
		if st.upperHex {
			return strings.ToUpper(s)
		}
		return strings.ToLower(s)
	}

	root.Children(func() {
		// 1. Header (48px)
		toolHeader(c, icons.Lock, "哈希与 HMAC 签名", "MD5 · SHA-1 / 256 / 512 实时摘要 · HMAC 签名 · 校验比对", func() {
			if toolBtn(c, toolBtnOpts{Key: "hash-sample", Label: "载入示例", Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.input = sampleHashInput
				if isHmac && st.secret == "" {
					st.secret = sampleHMACSecret
				}
				st.copiedKey = ""
			}
			if toolBtn(c, toolBtnOpts{Key: "hash-clear", Label: "清空", Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.input = ""
				st.secret = ""
				st.compareVal = ""
				st.copiedKey = ""
			}
		})

		// 2. Toolbar (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "hash-mode", []string{"摘要计算 Hash", "HMAC 签名"}, st.mode, func(idx int) {
				st.mode = idx
				st.copiedKey = ""
			})
			toolDividerV(c)
			if !isHmac {
				caseIdx := 0
				if st.upperHex {
					caseIdx = 1
				}
				toolSegmented(c, "hash-case", []string{"小写 hex", "大写 HEX"}, caseIdx, func(idx int) {
					st.upperHex = idx == 1
				})
				cmpVar := btnSecondary
				if st.compareOpen {
					cmpVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "hash-cmp-toggle", Label: "摘要校验比对", Icon: icons.Check, Variant: cmpVar}).Clicked() {
					st.compareOpen = !st.compareOpen
				}
			} else {
				toolSegmented(c, "hmac-algo", []string{"MD5", "SHA-1", "SHA-256", "SHA-512"}, st.hmacAlgoIdx, func(idx int) {
					st.hmacAlgoIdx = idx
					st.copiedKey = ""
				})
				fmtIdx := 0
				if st.hmacBase64 {
					fmtIdx = 1
				}
				toolSegmented(c, "hmac-fmt", []string{"HEX", "Base64"}, fmtIdx, func(idx int) {
					st.hmacBase64 = idx == 1
					st.copiedKey = ""
				})
			}
		}, func() {
			if isHmac {
				isCopied := st.copiedKey == "hmac-main"
				v := btnPrimary
				lbl := "复制签名"
				ic := icons.Copy
				if isCopied {
					v = btnSuccess
					lbl = "已复制签名"
					ic = icons.Check
				}
				if toolBtn(c, toolBtnOpts{Key: "hmac-copy-top", Label: lbl, Icon: ic, Variant: v, Disabled: st.input == "" && st.secret == ""}).Clicked() {
					c.Services().WriteClipboard(hmacOut)
					st.copiedKey = "hmac-main"
				}
			} else {
				allCopied := st.copiedKey == "hash-all"
				v := btnSecondary
				lbl := "复制全部结果"
				ic := icons.Copy
				if allCopied {
					v = btnSuccess
					lbl = "已复制全部"
					ic = icons.Check
				}
				if toolBtn(c, toolBtnOpts{Key: "hash-copy-all", Label: lbl, Icon: ic, Variant: v, Disabled: st.input == ""}).Clicked() {
					lines := []string{
						"MD5 (32): " + fmtHex(summary.MD5Lower),
						"MD5 (16): " + fmtHex(summary.MD516),
						"SHA-1:    " + fmtHex(summary.SHA1),
						"SHA-256:  " + fmtHex(summary.SHA256),
						"SHA-512:  " + fmtHex(summary.SHA512),
					}
					c.Services().WriteClipboard(strings.Join(lines, "\n"))
					st.copiedKey = "hash-all"
				}
			}
		})

		// 3. 主体滚动区
		ui.Scroll(c.Key("hash-scroll")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(18, 24).Gap(14).Children(func() {
				// 输入明文卡片
				inTitle := "INPUT 明文内容"
				if isHmac {
					inTitle = "MESSAGE 明文内容"
				}
				inMeta := "0 B"
				if st.input != "" {
					inMeta = fmt.Sprintf("%d B · UTF-8", len([]byte(st.input)))
				}
				toolCard(c, "hash-in-card", tk.Border, tk.BgApp, func() {
					ui.Text(c, inTitle).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
					ui.Text(c, inMeta).Font("monospace").FontSize(m.FontXS).TextColor(tk.TextMuted)
				}, func() {
					if st.input == "" {
						toolPill(c, "等待输入", pillNeutral, false)
					} else {
						toolPill(c, "实时联动计算", pillSuccess, true)
					}
				}, func() {
					ui.Box(c).FillWidth().Padding(8, 12).Children(func() {
						in := ui.TextAreaBase(c.Key("hash-input"), &st.input).
							FillWidth().MinHeight(72).MaxHeight(220).Padding(4, 4).
							Font("monospace").FontSize(13).FixedLineHeight(20).TextColor(tk.TextPrimary).
							Placeholder("在此输入或粘贴待计算摘要的字符串（支持多行与自动换行），输入时将实时计算全部算法结果...").
							PlaceholderColor(tk.TextMuted).
							Label("Hash input text")
						if in.Changed() {
							st.copiedKey = ""
						}
					})
				})

				if !isHmac {
					// 摘要比对校验栏
					if st.compareOpen {
						cmpBg := tk.BgApp
						cmpBorder := tk.Border
						if strings.TrimSpace(st.compareVal) != "" && st.input != "" {
							if hasMatch {
								cmpBg = tk.Success.Alpha(0.12)
								cmpBorder = tk.Success.Alpha(0.38)
							} else {
								cmpBg = tk.DangerSoft
								cmpBorder = tk.Danger.Alpha(0.38)
							}
						}
						ui.Row(c.Key("hash-cmp-bar")).FillWidth().MinHeight(46).Padding(8, 14).Gap(10).
							AlignItems(ui.Center).Radius(m.RadiusMD).Background(cmpBg).Border(m.Border, cmpBorder).Children(func() {
							icCol := tk.TextSecondary
							if hasMatch {
								icCol = tk.Success
							} else if strings.TrimSpace(st.compareVal) != "" && st.input != "" {
								icCol = tk.Danger
							}
							ui.Icon(c, icons.Check).FontSize(15).TextColor(icCol)
							ui.Text(c, "期望摘要比对").FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
							cmpIn := ui.TextInputBase(c.Key("hash-cmp-input"), &st.compareVal).
								Grow(1).Height(28).Padding(0, 10).Radius(m.RadiusSM).
								Background(tk.BgBase).Border(m.Border, cmpBorder).
								Font("monospace").FontSize(m.FontXS).TextColor(tk.TextPrimary).
								Placeholder("粘贴预期 MD5 / SHA 哈希值自动比对...").PlaceholderColor(tk.TextMuted).
								Label("Expected hash compare")
							if cmpIn.Focused() {
								cmpIn.Border(m.Border, tk.Focus)
							}
							if strings.TrimSpace(st.compareVal) != "" && st.input != "" {
								if hasMatch {
									toolPill(c, "完全匹配 · "+matchedAlg, pillSuccess, true)
								} else {
									toolPill(c, "不匹配", pillDanger, true)
								}
							}
						})
					}

					// 摘要结果表格卡片
					toolCard(c, "hash-results-card", tk.Border, tk.BgApp, func() {
						ui.Text(c, "摘要结果 (DIGEST RESULTS)").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
					}, func() {
						ui.Text(c, "点击行尾按钮单独复制").FontSize(m.FontXS).TextColor(tk.TextMuted)
					}, func() {
						rows := []struct {
							alg  string
							bits string
							val  string
						}{
							{"MD5 (32位)", "128-bit", fmtHex(summary.MD5Lower)},
							{"MD5 (16位)", "64-bit", fmtHex(summary.MD516)},
							{"SHA-1", "160-bit", fmtHex(summary.SHA1)},
							{"SHA-256", "256-bit", fmtHex(summary.SHA256)},
							{"SHA-512", "512-bit", fmtHex(summary.SHA512)},
						}
						for i, r := range rows {
							idx, item := i, r
							rowKey := fmt.Sprintf("hash-row-%d", idx)
							isMatch := hasMatch && matchedAlg == item.alg
							isCopied := st.copiedKey == rowKey
							row := ui.Row(c.Key(rowKey)).FillWidth().MinHeight(48).Padding(6, 14).Gap(12).AlignItems(ui.Center)
							if idx > 0 {
								row.BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)
							}
							if isMatch {
								row.Background(tk.Success.Alpha(0.12))
							} else if row.Hovered() {
								row.Background(tk.BgHover)
							}
							row.Children(func() {
								ui.Column(c).Width(108).Gap(2).Children(func() {
									algCol := tk.TextPrimary
									if isMatch {
										algCol = tk.Success
									}
									ui.Text(c, item.alg).FontSize(m.FontSM).FontWeight(600).TextColor(algCol).SingleLine()
									ui.Text(c, item.bits).Font("monospace").FontSize(10).TextColor(tk.TextMuted).SingleLine()
								})
								if st.input == "" {
									ui.Text(c, "— 等待输入明文后自动生成 —").FontSize(m.FontSM).TextColor(tk.TextDisabled).SingleLine().Grow(1)
								} else {
									vc := tk.TextPrimary
									if isMatch {
										vc = tk.Success
									}
									ui.Text(c, item.val).Font("monospace").FontSize(m.FontSM).FixedLineHeight(20).
										TextColor(vc).MinWidth(0).Grow(1)
								}
								if isMatch {
									toolPill(c, "匹配", pillSuccess, true)
								}
								if st.input != "" {
									bVar := btnGhost
									bLbl := "复制"
									bIc := icons.Copy
									if isCopied {
										bVar = btnSuccess
										bLbl = "已复制"
										bIc = icons.Check
									}
									if toolBtn(c, toolBtnOpts{Key: rowKey + "-copy", Label: bLbl, Icon: bIc, Variant: bVar, Height: 24, PadX: 8}).Clicked() {
										c.Services().WriteClipboard(item.val)
										st.copiedKey = rowKey
									}
								}
							})
						}
					})
				} else {
					// HMAC Secret Key 配置条
					ui.Row(c.Key("hmac-key-card")).FillWidth().Height(50).Padding(0, 14).Gap(10).
						AlignItems(ui.Center).Radius(m.RadiusMD).Background(tk.BgBase).
						Border(m.Border, tk.Accent.Alpha(0.35)).Children(func() {
						ui.Icon(c, icons.Lock).FontSize(14).TextColor(tk.Accent)
						ui.Text(c, "Secret Key").FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
						toolDividerV(c)
						secIn := ui.TextInputBase(c.Key("hash-hmac-secret"), &st.secret).
							Grow(1).Height(30).Padding(0, 4).
							Font("monospace").FontSize(13).TextColor(tk.TextPrimary).
							Placeholder("输入 HMAC Secret Key...").PlaceholderColor(tk.TextMuted).
							Label("HMAC secret")
						if !st.showSecret {
							secIn.Password()
						}
						if secIn.Changed() {
							st.copiedKey = ""
						}
						toolPill(c, fmt.Sprintf("UTF-8 · %d B", len([]byte(st.secret))), pillNeutral, false)
						eyeLbl := "显示密钥"
						if st.showSecret {
							eyeLbl = "隐藏密钥"
						}
						if toolBtn(c, toolBtnOpts{Key: "hmac-toggle-vis", Label: eyeLbl, Icon: icons.Eye, Variant: btnGhost, Height: 26, PadX: 8}).Clicked() {
							st.showSecret = !st.showSecret
						}
					})

					// HMAC 签名输出卡片
					fmtMeta := fmt.Sprintf("HEX · %d chars", len(hmacOut))
					if st.hmacBase64 {
						fmtMeta = fmt.Sprintf("Base64 · %d chars", len(hmacOut))
					}
					toolCard(c, "hmac-out-card", tk.Border, tk.BgApp, func() {
						ui.Text(c, "HMAC-"+algo.label+" 签名输出").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						if st.input != "" || st.secret != "" {
							toolPill(c, fmtMeta, pillAccent, false)
						}
					}, func() {
						if st.input != "" || st.secret != "" {
							toolPill(c, "签名已生成", pillSuccess, true)
						} else {
							toolPill(c, "等待输入", pillNeutral, false)
						}
					}, func() {
						ui.Column(c).FillWidth().Padding(16).Gap(14).Children(func() {
							sigDisp := hmacOut
							if st.input == "" && st.secret == "" {
								sigDisp = "— 输入 Message 与 Secret Key 后自动生成 HMAC 签名 —"
							}
							ui.Row(c).FillWidth().MinHeight(54).Padding(12, 14).Gap(12).
								AlignItems(ui.Center).Justify(ui.SpaceBetween).Radius(m.RadiusMD).
								Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.32)).Children(func() {
								ui.Text(c, sigDisp).Font("monospace").FontSize(13).FontWeight(500).
									FixedLineHeight(21).TextColor(tk.Accent).MinWidth(0).Grow(1)
								if st.input != "" || st.secret != "" {
									cVar := btnSecondary
									cLbl := "复制"
									cIc := icons.Copy
									if st.copiedKey == "hmac-main" {
										cVar = btnSuccess
										cLbl = "已复制"
										cIc = icons.Check
									}
									if toolBtn(c, toolBtnOpts{Key: "hmac-copy-box", Label: cLbl, Icon: cIc, Variant: cVar, Height: 26}).Clicked() {
										c.Services().WriteClipboard(hmacOut)
										st.copiedKey = "hmac-main"
									}
								}
							})

							if st.input != "" || st.secret != "" {
								hdrLine := fmt.Sprintf("X-Signature: %s=%s", strings.ToLower(algo.id), hmacOut)
								ui.Row(c).FillWidth().Padding(10, 14).Gap(10).AlignItems(ui.Center).Justify(ui.SpaceBetween).
									Radius(m.RadiusMD).Background(tk.BgApp).Border(m.Border, tk.Border).Children(func() {
									ui.Column(c).Grow(1).MinWidth(0).Gap(4).Children(func() {
										ui.Text(c, "请求头快捷预览 (可直接粘贴至 Headers)").FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary)
										ui.Text(c, hdrLine).Font("monospace").FontSize(m.FontSM).FixedLineHeight(19).
											TextColor(tk.TextPrimary).FillWidth().MinWidth(0)
									})
									hVar := btnGhost
									hLbl := "复制 Header"
									if st.copiedKey == "hmac-hdr" {
										hVar = btnSuccess
										hLbl = "已复制"
									}
									if toolBtn(c, toolBtnOpts{Key: "hmac-copy-hdr", Label: hLbl, Icon: icons.Copy, Variant: hVar, Height: 24, PadX: 8}).Clicked() {
										c.Services().WriteClipboard(hdrLine)
										st.copiedKey = "hmac-hdr"
									}
								})
							}
						})
					})
				}
			})
		})

		// 4. Footer (26px)
		fTxt := "就绪 · 所有计算均在本地离线执行"
		fCol := tk.TextMuted
		switch {
		case hasMatch:
			fTxt = "摘要校验通过 · 与 " + matchedAlg + " 完全一致"
			fCol = tk.Success
		case st.copiedKey != "":
			fTxt = "已复制到剪贴板"
			fCol = tk.Success
		case st.input != "":
			fTxt = "本地离线计算完成 · 密钥与明文不会离开本机"
			fCol = tk.Success
		}
		toolFooter(c, fTxt, fCol, "Native Go Crypto")
	})
}

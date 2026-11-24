package tools

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/devtools"
	"rddpostman/locales"

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
		toolHeader(c, icons.Lock, locales.ToolHashName(), locales.ToolHashDesc(), func() {
			if toolBtn(c, toolBtnOpts{Key: "hash-sample", Label: locales.LoadSample(), Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.input = sampleHashInput
				if isHmac && st.secret == "" {
					st.secret = sampleHMACSecret
				}
				st.copiedKey = ""
			}
			if toolBtn(c, toolBtnOpts{Key: "hash-clear", Label: locales.Clear(), Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.input = ""
				st.secret = ""
				st.compareVal = ""
				st.copiedKey = ""
			}
		})

		// 2. Toolbar (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "hash-mode", []string{locales.HashModeDigest(), locales.HashModeHMAC()}, st.mode, func(idx int) {
				st.mode = idx
				st.copiedKey = ""
			})
			toolDividerV(c)
			if !isHmac {
				caseIdx := 0
				if st.upperHex {
					caseIdx = 1
				}
				toolSegmented(c, "hash-case", []string{locales.HashCaseLower(), locales.HashCaseUpper()}, caseIdx, func(idx int) {
					st.upperHex = idx == 1
				})
				cmpVar := btnSecondary
				if st.compareOpen {
					cmpVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "hash-cmp-toggle", Label: locales.HashCompareToggle(), Icon: icons.Check, Variant: cmpVar}).Clicked() {
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
				lbl := locales.HashCopySig()
				ic := icons.Copy
				if isCopied {
					v = btnSuccess
					lbl = locales.HashCopiedSig()
					ic = icons.Check
				}
				if toolBtn(c, toolBtnOpts{Key: "hmac-copy-top", Label: lbl, Icon: ic, Variant: v, Disabled: st.input == "" && st.secret == ""}).Clicked() {
					c.Services().WriteClipboard(hmacOut)
					st.copiedKey = "hmac-main"
				}
			} else {
				allCopied := st.copiedKey == "hash-all"
				v := btnSecondary
				lbl := locales.HashCopyAll()
				ic := icons.Copy
				if allCopied {
					v = btnSuccess
					lbl = locales.HashCopiedAll()
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
				inTitle := locales.HashInputTitle()
				if isHmac {
					inTitle = locales.HashMsgTitle()
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
						toolPill(c, locales.WaitingForInput(), pillNeutral, false)
					} else {
						toolPill(c, locales.HashLiveCalc(), pillSuccess, true)
					}
				}, func() {
					ui.Box(c).FillWidth().Padding(8, 12).Children(func() {
						in := ui.TextAreaBase(c.Key("hash-input"), &st.input).
							FillWidth().MinHeight(72).MaxHeight(220).Padding(4, 4).
							Font("monospace").FontSize(13).FixedLineHeight(20).TextColor(tk.TextPrimary).
							Placeholder(locales.HashInputPlaceholder()).
							PlaceholderColor(tk.TextMuted).
							Label(locales.HashInputLabel())
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
							ui.Text(c, locales.HashCompareLabel()).FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
							cmpIn := ui.TextInputBase(c.Key("hash-cmp-input"), &st.compareVal).
								Grow(1).Height(28).Padding(0, 10).Radius(m.RadiusSM).
								Background(tk.BgBase).Border(m.Border, cmpBorder).
								Font("monospace").FontSize(m.FontXS).TextColor(tk.TextPrimary).
								Placeholder(locales.HashComparePlaceholder()).PlaceholderColor(tk.TextMuted).
								Label(locales.HashCompareInputLabel())
							if cmpIn.Focused() {
								cmpIn.Border(m.Border, tk.Focus)
							}
							if strings.TrimSpace(st.compareVal) != "" && st.input != "" {
								if hasMatch {
									toolPill(c, locales.HashMatchExact(matchedAlg), pillSuccess, true)
								} else {
									toolPill(c, locales.HashMismatch(), pillDanger, true)
								}
							}
						})
					}

					// 摘要结果表格卡片
					toolCard(c, "hash-results-card", tk.Border, tk.BgApp, func() {
						ui.Text(c, locales.HashResultsHeader()).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
					}, func() {
						ui.Text(c, locales.HashClickRowCopy()).FontSize(m.FontXS).TextColor(tk.TextMuted)
					}, func() {
						rows := []struct {
							alg  string
							bits string
							val  string
						}{
							{locales.HashMD532(), "128-bit", fmtHex(summary.MD5Lower)},
							{locales.HashMD516(), "64-bit", fmtHex(summary.MD516)},
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
									ui.Text(c, locales.HashWaitingRow()).FontSize(m.FontSM).TextColor(tk.TextDisabled).SingleLine().Grow(1)
								} else {
									vc := tk.TextPrimary
									if isMatch {
										vc = tk.Success
									}
									ui.Text(c, item.val).Font("monospace").FontSize(m.FontSM).FixedLineHeight(20).
										TextColor(vc).MinWidth(0).Grow(1)
								}
								if isMatch {
									toolPill(c, locales.HashMatchBadge(), pillSuccess, true)
								}
								if st.input != "" {
									bVar := btnGhost
									bLbl := locales.Copy()
									bIc := icons.Copy
									if isCopied {
										bVar = btnSuccess
										bLbl = locales.Copied()
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
							Placeholder(locales.HmacSecretPlaceholder()).PlaceholderColor(tk.TextMuted).
							Label("HMAC secret")
						if !st.showSecret {
							secIn.Password()
						}
						if secIn.Changed() {
							st.copiedKey = ""
						}
						toolPill(c, fmt.Sprintf("UTF-8 · %d B", len([]byte(st.secret))), pillNeutral, false)
						eyeLbl := locales.HmacShowSecret()
						if st.showSecret {
							eyeLbl = locales.HmacHideSecret()
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
						ui.Text(c, locales.HmacOutHeader(algo.label)).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						if st.input != "" || st.secret != "" {
							toolPill(c, fmtMeta, pillAccent, false)
						}
					}, func() {
						if st.input != "" || st.secret != "" {
							toolPill(c, locales.HmacGenerated(), pillSuccess, true)
						} else {
							toolPill(c, locales.WaitingForInput(), pillNeutral, false)
						}
					}, func() {
						ui.Column(c).FillWidth().Padding(16).Gap(14).Children(func() {
							sigDisp := hmacOut
							if st.input == "" && st.secret == "" {
								sigDisp = locales.HmacWaitingOut()
							}
							ui.Row(c).FillWidth().MinHeight(54).Padding(12, 14).Gap(12).
								AlignItems(ui.Center).Justify(ui.SpaceBetween).Radius(m.RadiusMD).
								Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.32)).Children(func() {
								ui.Text(c, sigDisp).Font("monospace").FontSize(13).FontWeight(500).
									FixedLineHeight(21).TextColor(tk.Accent).MinWidth(0).Grow(1)
								if st.input != "" || st.secret != "" {
									cVar := btnSecondary
									cLbl := locales.Copy()
									cIc := icons.Copy
									if st.copiedKey == "hmac-main" {
										cVar = btnSuccess
										cLbl = locales.Copied()
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
										ui.Text(c, locales.HmacHeaderPreview()).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary)
										ui.Text(c, hdrLine).Font("monospace").FontSize(m.FontSM).FixedLineHeight(19).
											TextColor(tk.TextPrimary).FillWidth().MinWidth(0)
									})
									hVar := btnGhost
									hLbl := locales.HmacCopyHeader()
									if st.copiedKey == "hmac-hdr" {
										hVar = btnSuccess
										hLbl = locales.Copied()
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
		fTxt := locales.HashFooterReady()
		fCol := tk.TextMuted
		switch {
		case hasMatch:
			fTxt = locales.HashFooterMatched(matchedAlg)
			fCol = tk.Success
		case st.copiedKey != "":
			fTxt = locales.HashFooterCopied()
			fCol = tk.Success
		case st.input != "":
			fTxt = locales.HashFooterDone()
			fCol = tk.Success
		}
		toolFooter(c, fTxt, fCol, "Native Go Crypto")
	})
}

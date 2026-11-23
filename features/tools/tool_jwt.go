package tools

import (
	"fmt"
	"strings"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/devtools"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
)

const sampleJWTSecret = "your-256-bit-secret-key-prod"

func makeSampleJWT() string {
	now := time.Now().Unix()
	hdr := `{"alg":"HS256","typ":"JWT","kid":"key-2026-prod"}`
	pay := fmt.Sprintf(`{"sub":"user_8842","name":"Alice Chen","role":"admin","scope":["orders:read","orders:write"],"iss":"rddpostman.auth","iat":%d,"nbf":%d,"exp":%d,"jti":"jwt_9f82c10a44e2"}`,
		now-1800, now-1800, now+7200)
	return devtools.SignJWTHS256(hdr, pay, sampleJWTSecret)
}

type jwtToolState struct {
	token        string
	secret       string
	secretBase64 bool
	verifyOpen   bool
	headerEd     *editor.State
	payloadEd    *editor.State
	copiedKey    string
}

func renderJWTTool(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-jwt-root")).Grow(1).FillWidth()
	st := *ui.Local(root, "jwt-state-v2", func() *jwtToolState {
		hBuf := editor.NewBuffer("")
		hBuf.SetLanguage(editor.LanguageNamed("JSON"))
		pBuf := editor.NewBuffer("")
		pBuf.SetLanguage(editor.LanguageNamed("JSON"))
		return &jwtToolState{
			headerEd:  &editor.State{Buffer: hBuf, ReadOnly: true, SoftWrap: true, LineNumbers: editor.LineNumbersOff},
			payloadEd: &editor.State{Buffer: pBuf, ReadOnly: true, SoftWrap: true, LineNumbers: editor.LineNumbersOff},
		}
	})

	trimmed := strings.TrimSpace(st.token)
	var decoded *devtools.JWTDecoded
	var decodeErr string
	if trimmed != "" {
		res, err := devtools.DecodeJWT(trimmed)
		if err != nil {
			decodeErr = err.Error()
		} else {
			decoded = res
			if st.headerEd.Buffer.String() != res.HeaderJSON {
				st.headerEd.Buffer.Reset(res.HeaderJSON)
			}
			if st.payloadEd.Buffer.String() != res.PayloadJSON {
				st.payloadEd.Buffer.Reset(res.PayloadJSON)
			}
		}
	} else {
		if st.headerEd.Buffer.String() != "" {
			st.headerEd.Buffer.Reset("")
		}
		if st.payloadEd.Buffer.String() != "" {
			st.payloadEd.Buffer.Reset("")
		}
	}

	// 本地签名校验
	sigVerified := false
	sigChecked := false
	sigErr := ""
	if decoded != nil && st.verifyOpen && strings.TrimSpace(st.secret) != "" {
		sigChecked = true
		ok, _, err := devtools.VerifyJWTHS(trimmed, st.secret, st.secretBase64)
		if err != nil {
			sigErr = err.Error()
		} else {
			sigVerified = ok
		}
	}

	root.Children(func() {
		// 1. Header (48px)
		toolHeader(c, icons.Eye, "JWT 令牌解析器", "自动剥离 Bearer 前缀 · 三段着色 · Claims 时间换算 · 本地验签", func() {
			if toolBtn(c, toolBtnOpts{Key: "jwt-sample", Label: "载入示例 JWT", Icon: icons.Sparkles, Variant: btnGhost, Height: 26}).Clicked() {
				st.token = makeSampleJWT()
				st.secret = sampleJWTSecret
				st.copiedKey = ""
			}
			if toolBtn(c, toolBtnOpts{Key: "jwt-clear", Label: "清空", Icon: icons.Trash, Variant: btnGhost, Height: 26}).Clicked() {
				st.token = ""
				st.secret = ""
				st.copiedKey = ""
			}
		})

		// 2. 主体滚动区
		ui.Scroll(c.Key("jwt-scroll")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(16, 24).Gap(12).Children(func() {
				// 顶部 ENCODED JWT TOKEN 输入与三段着色卡片
				tokBorder := tk.Border
				if decodeErr != "" {
					tokBorder = tk.Danger.Alpha(0.45)
				}
				toolCard(c, "jwt-token-card", tokBorder, tk.BgApp, func() {
					ui.Text(c, "ENCODED JWT TOKEN").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
					if decoded != nil {
						toolPill(c, "Header", pillAccent, false)
						toolPill(c, "Payload", pillPurple, false)
						toolPill(c, "Signature", pillSuccess, false)
					}
				}, func() {
					switch {
					case trimmed == "":
						toolPill(c, "等待输入", pillNeutral, false)
					case decodeErr != "":
						toolPill(c, "格式无效", pillDanger, true)
					case decoded.HasExp && decoded.Expired:
						toolPill(c, decoded.ExpiresRel, pillDanger, true)
					case sigChecked && sigVerified:
						toolPill(c, decoded.Alg+" · 验签通过", pillSuccess, true)
					default:
						if decoded.Alg != "" {
							toolPill(c, decoded.Alg, pillNeutral, false)
						}
						if decoded.HasExp {
							toolPill(c, "有效 · "+decoded.ExpiresRel, pillSuccess, true)
						} else {
							toolPill(c, "解析成功", pillSuccess, true)
						}
					}
				}, func() {
					ui.Column(c).FillWidth().Padding(10, 14).Gap(6).Children(func() {
						in := ui.TextInputBase(c.Key("jwt-input"), &st.token).
							FillWidth().Height(32).Padding(0, 8).Radius(m.RadiusSM).
							Background(tk.BgSubtle).Font("monospace").FontSize(m.FontSM).TextColor(tk.TextPrimary).
							Placeholder("粘贴 JWT 或 Authorization: Bearer eyJhbGci... 字符串，将自动剥离前缀并实时解析").
							PlaceholderColor(tk.TextMuted).
							Label("JWT token input")
						if in.Focused() {
							in.Border(m.Border, tk.Focus)
						}
						if in.Changed() {
							st.copiedKey = ""
						}

						// 三段语义着色预览条
						if decoded != nil {
							ui.Row(c).FillWidth().Padding(6, 8).Radius(m.RadiusSM).Background(tk.BgApp).
								Border(m.Border, tk.Border).Gap(0).Children(func() {
								ui.Text(c, truncMiddle(decoded.HeaderRaw, 38)).Font("monospace").FontSize(11).TextColor(tk.Accent).SingleLine()
								ui.Text(c, ".").Font("monospace").FontSize(11).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
								ui.Text(c, truncMiddle(decoded.PayloadRaw, 54)).Font("monospace").FontSize(11).TextColor(tk.MethodPatch).SingleLine()
								if decoded.SignatureRaw != "" {
									ui.Text(c, ".").Font("monospace").FontSize(11).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
									ui.Text(c, truncMiddle(decoded.SignatureRaw, 32)).Font("monospace").FontSize(11).TextColor(tk.Success).SingleLine()
								}
							})
						}
					})
				})

				// 空态引导面板
				if trimmed == "" {
					ui.Column(c).FillWidth().MinHeight(380).Center().Padding(24).Gap(14).
						Radius(m.RadiusLG).Background(tk.BgApp).Border(m.Border, tk.Border).Children(func() {
						ui.Box(c).Size(48, 48).Radius(12).Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.25)).Center().Children(func() {
							ui.Icon(c, icons.EyeBoldDuotone).FontSize(24).TextColor(tk.Accent)
						})
						ui.Text(c, "粘贴任意 JWT Token 开始离线解析").FontSize(m.FontMD).FontWeight(600).TextColor(tk.TextPrimary)
						ui.Text(c, "完整保留隐私：所有 Header / Payload 解码与 HS256 验签均在本地完成，不上传任何服务器").
							FontSize(m.FontSM).TextColor(tk.TextMuted)
						ui.Row(c).Gap(12).Padding(4, 0).Children(func() {
							for _, seg := range []struct {
								n, d string
								v    pillVariant
							}{
								{"1. Header", "算法与密钥标识", pillAccent},
								{"2. Payload", "业务载荷与时间声明", pillPurple},
								{"3. Signature", "本地密钥防篡改校验", pillSuccess},
							} {
								ui.Column(c).Width(190).Padding(12, 14).Gap(6).Radius(m.RadiusMD).
									Background(tk.BgBase).Border(m.Border, tk.Border).Children(func() {
									toolPill(c, seg.n, seg.v, false)
									ui.Text(c, seg.d).FontSize(m.FontXS).TextColor(tk.TextSecondary)
								})
							}
						})
						ui.Row(c).Gap(8).Padding(4, 0, 0, 0).Children(func() {
							if toolBtn(c, toolBtnOpts{Key: "jwt-empty-paste", Label: "从剪贴板粘贴", Icon: icons.Copy, Variant: btnPrimary, Height: 30}).Clicked() {
								if clip := c.Services().ReadClipboard(); strings.TrimSpace(clip) != "" {
									st.token = clip
								}
							}
							if toolBtn(c, toolBtnOpts{Key: "jwt-empty-sample", Label: "载入示例 JWT", Icon: icons.Sparkles, Variant: btnSecondary, Height: 30}).Clicked() {
								st.token = makeSampleJWT()
								st.secret = sampleJWTSecret
							}
						})
					})
					return
				}

				// 损坏/非法格式诊断视图
				if decodeErr != "" {
					toolInlineErrorCard(c, "jwt-err-banner", "JWT 格式损坏 · 无法完成完整解析", decodeErr, "替换为示例 JWT", func() {
						st.token = makeSampleJWT()
						st.secret = sampleJWTSecret
					})

					diags := devtools.DiagnoseJWT(trimmed)
					ui.Row(c).FillWidth().Gap(12).AlignItems(ui.Stretch).Children(func() {
						for i, d := range diags {
							dg := d
							bc := tk.Border
							hb := tk.BgApp
							if !dg.OK {
								bc = tk.Danger.Alpha(0.4)
								hb = tk.DangerSoft
							}
							ui.Box(c.Key(fmt.Sprintf("jwt-diag-%d", i))).Grow(1).Basis(0).Children(func() {
								toolCard(c, fmt.Sprintf("jwt-diag-card-%d", i), bc, hb, func() {
									tc := tk.TextSecondary
									if !dg.OK {
										tc = tk.Danger
									}
									ui.Text(c, dg.Title).FontSize(m.FontXS).FontWeight(600).TextColor(tc)
								}, func() {
									v := pillSuccess
									if !dg.OK {
										v = pillDanger
									}
									toolPill(c, dg.Badge, v, true)
								}, func() {
									ui.Box(c).FillWidth().MinHeight(86).Padding(12, 14).Children(func() {
										dc := tk.TextPrimary
										if !dg.OK {
											dc = tk.Danger
										}
										ui.Text(c, dg.Detail).FontSize(m.FontSM).TextColor(dc)
									})
								})
							})
						}
					})
					return
				}

				// 过期红色告警横幅
				if decoded.HasExp && decoded.Expired {
					ui.Row(c).FillWidth().Padding(8, 14).Gap(8).AlignItems(ui.Center).Justify(ui.SpaceBetween).
						Radius(m.RadiusMD).Background(tk.DangerSoft).Border(m.Border, tk.Danger.Alpha(0.38)).Children(func() {
						ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Icon(c, icons.TriangleAlert).FontSize(14).TextColor(tk.Danger)
							ui.Text(c, fmt.Sprintf("令牌已过期：exp (%s) 早于当前时间（%s），服务端鉴权将返回 401 Unauthorized",
								decoded.ExpiresAt, decoded.ExpiresRel)).
								FontSize(m.FontSM).FontWeight(500).TextColor(tk.Danger)
						})
						toolPill(c, "EXPIRED", pillDanger, false)
					})
				}

				// 双栏解析主体（左：Header + Time Claims；右：Payload + Signature Verify）
				ui.Row(c).FillWidth().Gap(14).AlignItems(ui.Start).Children(func() {
					// 左栏
					ui.Column(c).Grow(9).Basis(0).Gap(12).Children(func() {
						// HEADER 卡片
						toolCard(c, "jwt-hdr-card", tk.Border, tk.BgApp, func() {
							ui.Box(c).Size(7, 7).Radius(4).Background(tk.Accent)
							ui.Text(c, "HEADER · 算法与类型").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						}, func() {
							cVar := btnGhost
							cLbl := "复制"
							if st.copiedKey == "jwt-hdr" {
								cVar = btnSuccess
								cLbl = "已复制"
							}
							if toolBtn(c, toolBtnOpts{Key: "jwt-copy-hdr", Label: cLbl, Icon: icons.Copy, Variant: cVar, Height: 22, PadX: 6}).Clicked() {
								c.Services().WriteClipboard(decoded.HeaderJSON)
								st.copiedKey = "jwt-hdr"
							}
						}, func() {
							ui.Box(c.Key("jwt-hdr-ed")).FillWidth().Height(112).Padding(6, 10).Clip().Children(func() {
								editor.View(c, st.headerEd).Fill()
							})
						})

						// TIME CLAIMS 时间声明换算卡片
						tcBorder := tk.Border
						if decoded.HasExp && decoded.Expired {
							tcBorder = tk.Danger.Alpha(0.4)
						}
						toolCard(c, "jwt-claims-card", tcBorder, tk.BgApp, func() {
							ui.Text(c, "TIME CLAIMS · 时间声明换算").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						}, func() {
							ui.Text(c, "本地时区").FontSize(m.FontXS).TextColor(tk.TextMuted)
						}, func() {
							type claimRow struct {
								k, label, date string
								unix           int64
								rel            string
								variant        pillVariant
								bad            bool
								present        bool
							}
							rows := []claimRow{
								{"iat", "签发时间", decoded.IssuedAt, decoded.IssuedAtUnix, decoded.IssuedRel, pillNeutral, false, decoded.HasIat},
								{"nbf", "生效时间", decoded.NotBefore, decoded.NotBeforeUnix, "已生效", pillNeutral, false, decoded.HasNbf},
								{"exp", "过期时间", decoded.ExpiresAt, decoded.ExpiresAtUnix, decoded.ExpiresRel, map[bool]pillVariant{true: pillDanger, false: pillSuccess}[decoded.Expired], decoded.Expired, decoded.HasExp},
							}
							shown := 0
							for _, cr := range rows {
								if !cr.present {
									continue
								}
								r := ui.Row(c.Key("jwt-cl-" + cr.k)).FillWidth().MinHeight(50).Padding(6, 14).Gap(8).
									AlignItems(ui.Center).Justify(ui.SpaceBetween)
								if shown > 0 {
									r.BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)
								}
								if cr.bad {
									r.Background(tk.DangerSoft)
								}
								shown++
								r.Children(func() {
									ui.Column(c).Gap(2).Children(func() {
										ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
											kc := tk.MethodPatch
											if cr.bad {
												kc = tk.Danger
											}
											ui.Text(c, cr.k).Font("monospace").FontSize(m.FontSM).FontWeight(600).TextColor(kc)
											ui.Text(c, cr.label).FontSize(m.FontXS).TextColor(tk.TextSecondary)
										})
										dc := tk.TextPrimary
										if cr.bad {
											dc = tk.Danger
										}
										ui.Text(c, fmt.Sprintf("%s  (%d)", cr.date, cr.unix)).Font("monospace").FontSize(m.FontXS).TextColor(dc)
									})
									toolPill(c, cr.rel, cr.variant, cr.bad || cr.k == "exp")
								})
							}
							if shown == 0 {
								ui.Box(c).FillWidth().Padding(16).Center().Children(func() {
									ui.Text(c, "载荷中未包含 iat / nbf / exp 标准时间声明").FontSize(m.FontXS).TextColor(tk.TextMuted)
								})
							}
						})
					})

					// 右栏
					ui.Column(c).Grow(11).Basis(0).Gap(12).Children(func() {
						// PAYLOAD 卡片（附 RFC 7519 字段说明浮层）
						toolCard(c, "jwt-pay-card", tk.Border, tk.BgApp, func() {
							ui.Box(c).Size(7, 7).Radius(4).Background(tk.MethodPatch)
							ui.Text(c, "PAYLOAD · 业务载荷").FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						}, func() {
							infoBtn := toolBtn(c, toolBtnOpts{Key: "jwt-claims-info", Label: "字段说明", Icon: icons.Info, Variant: btnGhost, Height: 22, PadX: 6})
							infoOpen := ui.Local(infoBtn, "jwt-info-open", func() bool { return false })
							if infoBtn.Clicked() {
								*infoOpen = !*infoOpen
							}
							v2.Popover(c, infoBtn, infoOpen, v2.PopoverOpts{Width: 288}, func(panel ui.Element) {
								ui.Column(c).Padding(12).Gap(8).Children(func() {
									ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
										ui.Icon(c, icons.Info).FontSize(13).TextColor(tk.Accent)
										ui.Text(c, "RFC 7519 标准声明说明").FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary)
									})
									for _, item := range [][2]string{
										{"sub", "Subject — 令牌面向的用户唯一标识"},
										{"iss", "Issuer — 签发方服务标识"},
										{"aud", "Audience — 接收方受众标识"},
										{"exp / iat", "Expiration / Issued At — 过期与签发秒级时间戳"},
										{"jti", "JWT ID — 唯一编号，用于防重放校验"},
									} {
										ui.Row(c).Gap(8).AlignItems(ui.Start).Children(func() {
											ui.Text(c, item[0]).Font("monospace").FontSize(m.FontXS).FontWeight(600).TextColor(tk.Accent).Width(60)
											ui.Text(c, item[1]).FontSize(m.FontXS).TextColor(tk.TextSecondary).Grow(1)
										})
									}
								})
							})

							cVar := btnGhost
							cLbl := "复制 Payload"
							cIc := icons.Copy
							if st.copiedKey == "jwt-pay" {
								cVar = btnSuccess
								cLbl = "已复制 Payload"
								cIc = icons.Check
							}
							if toolBtn(c, toolBtnOpts{Key: "jwt-copy-payload", Label: cLbl, Icon: cIc, Variant: cVar, Height: 22, PadX: 8}).Clicked() {
								c.Services().WriteClipboard(decoded.PayloadJSON)
								st.copiedKey = "jwt-pay"
							}
						}, func() {
							ui.Box(c.Key("jwt-pay-ed")).FillWidth().Height(210).Padding(6, 10).Clip().Children(func() {
								editor.View(c, st.payloadEd).Fill()
							})
						})

						// SIGNATURE / VERIFY 卡片
						sigBorder := tk.Border
						sigHdrBg := tk.BgApp
						if sigChecked && sigVerified {
							sigBorder = tk.Success.Alpha(0.4)
							sigHdrBg = tk.Success.Alpha(0.12)
						} else if sigChecked && !sigVerified {
							sigBorder = tk.Danger.Alpha(0.4)
							sigHdrBg = tk.DangerSoft
						}
						toolCard(c, "jwt-sig-card", sigBorder, sigHdrBg, func() {
							ui.Box(c).Size(7, 7).Radius(4).Background(tk.Success)
							lbl := "SIGNATURE · 签名摘要"
							if st.verifyOpen {
								lbl = "VERIFY SIGNATURE · 签名校验"
							}
							ui.Text(c, lbl).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
						}, func() {
							switch {
							case sigChecked && sigVerified:
								toolPill(c, "Signature Verified ✓", pillSuccess, true)
							case sigChecked && !sigVerified:
								toolPill(c, "Invalid Signature", pillDanger, true)
							default:
								toolPill(c, "未验签 (仅解码)", pillNeutral, false)
							}
						}, func() {
							ui.Column(c).FillWidth().Padding(12, 14).Gap(10).Children(func() {
								if !st.verifyOpen {
									ui.Text(c, `HMACSHA256(base64UrlEncode(header) + "." + base64UrlEncode(payload), secret)`).
										Font("monospace").FontSize(m.FontXS).TextColor(tk.TextSecondary)
									ui.Row(c).Children(func() {
										if toolBtn(c, toolBtnOpts{Key: "jwt-open-verify", Label: "输入密钥验证签名", Icon: icons.Lock, Variant: btnSecondary, Height: 26}).Clicked() {
											st.verifyOpen = true
										}
									})
								} else {
									ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
										secIn := ui.TextInputBase(c.Key("jwt-secret-input"), &st.secret).
											Grow(1).Height(32).Padding(0, 10).Radius(m.RadiusSM).
											Background(tk.BgBase).Border(m.Border, sigBorder).
											Font("monospace").FontSize(m.FontSM).TextColor(tk.TextPrimary).
											Placeholder("输入 HS256 / HS384 / HS512 Secret 密钥...").
											PlaceholderColor(tk.TextMuted).
											Label("JWT verify secret")
										if secIn.Focused() {
											secIn.Border(m.Border, tk.Focus)
										}
										toolCheckboxChip(c, "jwt-sec-b64", "Base64 密钥", st.secretBase64, func() {
											st.secretBase64 = !st.secretBase64
										})
									})
									if sigErr != "" {
										ui.Text(c, sigErr).FontSize(m.FontXS).TextColor(tk.Danger)
									} else if sigChecked {
										bg := tk.DangerSoft
										fg := tk.Danger
										msg := "签名不匹配 · 请检查 Secret 密钥或确认载荷未被篡改"
										if sigVerified {
											bg = tk.Success.Alpha(0.12)
											fg = tk.Success
											msg = "签名完全匹配 · Header 与 Payload 完整性已通过本地校验"
										}
										ui.Row(c).FillWidth().Padding(8, 10).Gap(8).AlignItems(ui.Center).Radius(m.RadiusSM).Background(bg).Children(func() {
											ui.Icon(c, map[bool]*ui.SVG{true: icons.Check, false: icons.TriangleAlert}[sigVerified]).FontSize(13).TextColor(fg)
											ui.Text(c, msg).FontSize(m.FontXS).FontWeight(500).TextColor(fg)
										})
									}
								}
							})
						})
					})
				})
			})
		})

		// 3. Footer (26px)
		fTxt := "就绪 · 支持自动去除 Bearer 前缀"
		fCol := tk.TextMuted
		switch {
		case decodeErr != "":
			fTxt = "解析失败 · Token 格式不符合 JWT 规范"
			fCol = tk.Danger
		case decoded != nil && decoded.HasExp && decoded.Expired:
			fTxt = "注意：该 Token " + decoded.ExpiresRel
			fCol = tk.Danger
		case sigChecked && sigVerified:
			fTxt = decoded.Alg + " 签名验证通过 · 令牌有效"
			fCol = tk.Success
		case st.copiedKey == "jwt-pay":
			fTxt = "已复制格式化 Payload JSON 到剪贴板"
			fCol = tk.Success
		case decoded != nil:
			fTxt = "解析成功 · 本地离线解码"
			fCol = tk.Success
		}
		toolFooter(c, fTxt, fCol, "RFC 7519 · Local Decode")
	})
}

func truncMiddle(s string, max int) string {
	if len(s) <= max || max < 8 {
		return s
	}
	return s[:max-3] + "..."
}

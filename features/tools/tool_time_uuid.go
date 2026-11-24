package tools

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/devtools"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

type tzOption struct {
	name   string
	offset string
	loc    *time.Location
}

var tzOptions = []tzOption{
	{name: "Asia/Shanghai", offset: "UTC+8", loc: time.FixedZone("CST", 8*3600)},
	{name: "UTC", offset: "UTC+0", loc: time.UTC},
	{name: "America/New_York", offset: "UTC-5", loc: time.FixedZone("EST", -5*3600)},
	{name: "Europe/London", offset: "UTC+0", loc: time.FixedZone("GMT", 0)},
	{name: "Asia/Tokyo", offset: "UTC+9", loc: time.FixedZone("JST", 9*3600)},
}

// ---- 5A. 时间戳转换 ----

type tsToolState struct {
	tzIdx      int
	paused     bool
	pausedTime time.Time
	unitMs     bool // false=s, true=ms
	tsInput    string
	dtInput    string
	tsErr      string
	dtErr      string
	tsRes      devtools.TimestampInfo
	dtRes      devtools.TimestampInfo
	activePreset int // -1 none
	copiedKey  string
}

func renderTimestampTool(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-ts-root")).Grow(1).FillWidth()
	st := *ui.Local(root, "ts-state-v2", func() *tsToolState {
		now := time.Now()
		info := devtools.FromTimeInZone(now, tzOptions[0].loc)
		return &tsToolState{
			tzIdx:        0,
			pausedTime:   now,
			tsInput:      strconv.FormatInt(info.Seconds, 10),
			dtInput:      info.Local,
			tsRes:        info,
			dtRes:        info,
			activePreset: -1,
		}
	})

	tz := tzOptions[st.tzIdx]
	clockNow := time.Now()
	if st.paused {
		clockNow = st.pausedTime
	} else {
		// MyGo 是事件驱动渲染框架，需要主动通过 c.After 预约下一帧重绘；
		// 暂停或切离该 Tab 后不再调用，待机零 CPU 占用。
		c.After(200 * time.Millisecond)
	}
	clockInfo := devtools.FromTimeInZone(clockNow, tz.loc)

	convertTS := func() {
		unit := "s"
		if st.unitMs {
			unit = "ms"
		}
		res, err := devtools.TimestampToDateTimeInZone(st.tsInput, unit, tz.loc)
		if err != nil {
			st.tsErr = err.Error()
		} else {
			st.tsErr = ""
			st.tsRes = res
		}
	}
	convertDT := func() {
		res, err := devtools.DateTimeToTimestampInZone(st.dtInput, tz.loc)
		if err != nil {
			st.dtErr = err.Error()
		} else {
			st.dtErr = ""
			st.dtRes = res
		}
	}

	root.Children(func() {
		// 1. Header (48px) + 右侧时区下拉选择器
		toolHeader(c, icons.Clock, locales.ToolTimestampTitle(), locales.ToolTimestampDesc(), func() {
			tzBtn := toolBtn(c, toolBtnOpts{
				Key:     "ts-tz-btn",
				Label:   fmt.Sprintf("%s (%s)", tz.name, tz.offset),
				Icon:    icons.Globe,
				Variant: btnSecondary,
				Height:  26,
				Chevron: true,
			})
			tzOpen := ui.Local(tzBtn, "ts-tz-open", func() bool { return false })
			if tzBtn.Clicked() {
				*tzOpen = !*tzOpen
			}
			if *tzOpen {
				tzBtn.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.35))
			}
			v2.Popover(c, tzBtn, tzOpen, v2.PopoverOpts{Width: 280}, func(panel ui.Element) {
				ui.Column(c).Padding(4).Gap(2).Children(func() {
					now := time.Now()
					for i, opt := range tzOptions {
						idx, z := i, opt
						on := idx == st.tzIdx
						row := ui.ButtonBase(c.Key("ts-tz-opt-"+z.name)).
							FillWidth().Height(34).Padding(0, 10).Gap(8).
							Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.SpaceBetween).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(z.name)
						if on {
							row.Background(tk.AccentSoft)
						} else if row.Hovered() {
							row.Background(tk.BgHover)
						}
						if row.Clicked() {
							st.tzIdx = idx
							*tzOpen = false
							tz = tzOptions[st.tzIdx]
							convertTS()
							convertDT()
						}
						row.Children(func() {
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								fg := tk.TextPrimary
								w := 400
								if on {
									fg = tk.Accent
									w = 500
								}
								ui.Text(c, z.name).FontSize(m.FontSM).FontWeight(w).TextColor(fg).SingleLine()
								ui.Text(c, z.offset).Font("monospace").FontSize(10).TextColor(tk.TextMuted).SingleLine()
							})
							ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
								ui.Text(c, now.In(z.loc).Format("15:04:05")).Font("monospace").FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()
								if on {
									ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.Accent)
								}
							})
						})
					}
				})
			})
		})

		// 2. 主体滚动区
		ui.Scroll(c.Key("ts-scroll")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(20, 24).Gap(16).Children(func() {
				// 顶部实时时钟横幅卡片
				clkBorder := tk.Border
				if st.paused {
					clkBorder = tk.Warning.Alpha(0.45)
				}
				ui.Row(c).FillWidth().MinHeight(82).Padding(14, 20).Gap(20).
					AlignItems(ui.Center).Justify(ui.SpaceBetween).
					Radius(m.RadiusLG).Background(tk.BgApp).Border(m.Border, clkBorder).Children(func() {
					ui.Row(c).Gap(24).AlignItems(ui.Center).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Text(c, locales.TsCurrentSec()).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary)
								if st.paused {
									toolPill(c, locales.TsPaused(), pillWarning, true)
								} else {
									toolPill(c, locales.TsLive(), pillSuccess, true)
								}
							})
							ui.Text(c, strconv.FormatInt(clockInfo.Seconds, 10)).
								Font("monospace").FontSize(24).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
						})
						ui.Box(c).Size(1, 44).Background(tk.Border)
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, locales.TsCurrentMs()).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary)
							ui.Text(c, strconv.FormatInt(clockInfo.Milliseconds, 10)).
								Font("monospace").FontSize(18).FontWeight(500).TextColor(tk.Accent).SingleLine()
						})
					})

					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						pVar := btnSecondary
						pLbl := locales.TsPause()
						if st.paused {
							pVar = btnPrimary
							pLbl = locales.TsResume()
						}
						if toolBtn(c, toolBtnOpts{Key: "ts-pause-btn", Label: pLbl, Icon: icons.Play, Variant: pVar}).Clicked() {
							st.paused = !st.paused
							if st.paused {
								st.pausedTime = time.Now()
							}
						}
						cSecVar := btnSecondary
						cSecLbl := locales.TsCopySec()
						if st.copiedKey == "clk-sec" {
							cSecVar, cSecLbl = btnSuccess, locales.Copied()
						}
						if toolBtn(c, toolBtnOpts{Key: "ts-copy-sec", Label: cSecLbl, Icon: icons.Copy, Variant: cSecVar}).Clicked() {
							c.Services().WriteClipboard(strconv.FormatInt(clockInfo.Seconds, 10))
							st.copiedKey = "clk-sec"
						}
						cMsVar := btnSecondary
						cMsLbl := locales.TsCopyMs()
						if st.copiedKey == "clk-ms" {
							cMsVar, cMsLbl = btnSuccess, locales.Copied()
						}
						if toolBtn(c, toolBtnOpts{Key: "ts-copy-ms", Label: cMsLbl, Icon: icons.Copy, Variant: cMsVar}).Clicked() {
							c.Services().WriteClipboard(strconv.FormatInt(clockInfo.Milliseconds, 10))
							st.copiedKey = "clk-ms"
						}
					})
				})

				// 双栏 50/50 互转卡片
				ui.Row(c).FillWidth().Gap(16).AlignItems(ui.Start).Children(func() {
					// 左卡：时间戳 -> 日期时间
					ui.Box(c).Grow(1).Basis(0).Children(func() {
						toolCard(c, "ts-to-dt-card", tk.Border, tk.BgApp, func() {
							ui.Text(c, locales.TsToDtTitle()).FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary)
						}, func() {
							uIdx := 0
							if st.unitMs {
								uIdx = 1
							}
							toolSegmented(c, "ts-unit", []string{locales.TsUnitSec(), locales.TsUnitMs()}, uIdx, func(idx int) {
								st.unitMs = idx == 1
								convertTS()
							})
						}, func() {
							ui.Column(c).FillWidth().Padding(16).Gap(14).Children(func() {
								ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
									in := ui.TextInputBase(c.Key("ts-to-dt-input"), &st.tsInput).
										Grow(1).Height(34).Padding(0, 12).Radius(m.RadiusMD).
										Background(tk.BgBase).Border(m.Border, tk.Border).
										Font("monospace").FontSize(13).TextColor(tk.TextPrimary).
										Placeholder(locales.TsPlaceholder()).PlaceholderColor(tk.TextMuted).
										Label("Unix timestamp input")
									if in.Focused() {
										in.Border(m.Border, tk.Focus)
									}
									if in.Changed() || in.Submitted() {
										convertTS()
									}
									if toolBtn(c, toolBtnOpts{Key: "ts-fill-now", Label: locales.TsFillNow(), Variant: btnSecondary, Height: 34}).Clicked() {
										now := time.Now()
										info := devtools.FromTimeInZone(now, tz.loc)
										if st.unitMs {
											st.tsInput = strconv.FormatInt(info.Milliseconds, 10)
										} else {
											st.tsInput = strconv.FormatInt(info.Seconds, 10)
										}
										st.tsRes = info
										st.tsErr = ""
									}
								})

								if st.tsErr != "" {
									toolInlineErrorCard(c, "ts-err", locales.TsParseErrTitle(), st.tsErr, "", nil)
								}

								// 5 行多格式输出表
								ui.Column(c).FillWidth().Radius(m.RadiusMD).Border(m.Border, tk.Border).Background(tk.BgBase).Clip().Children(func() {
									fmts := []struct {
										k    string
										val  string
										mono bool
									}{
										{locales.TsLocalTime(tz.offset), st.tsRes.Local, true},
										{locales.TsUTCTime(), st.tsRes.UTC, true},
										{"ISO 8601", st.tsRes.ISO8601, true},
										{"RFC 2822", st.tsRes.RFC2822, true},
										{locales.TsRelativeTime(), st.tsRes.Relative, false},
									}
									for i, f := range fmts {
										idx, item := i, f
										rKey := fmt.Sprintf("ts-fmt-%d", idx)
										isCopied := st.copiedKey == rKey
										r := ui.Row(c.Key(rKey)).FillWidth().MinHeight(46).Padding(6, 12).Gap(8).
											AlignItems(ui.Center).Justify(ui.SpaceBetween)
										if idx > 0 {
											r.BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)
										}
										if isCopied {
											r.Background(tk.Success.Alpha(0.12))
										} else if r.Hovered() {
											r.Background(tk.BgHover)
										}
										r.Children(func() {
											ui.Column(c).Grow(1).Gap(2).Children(func() {
												ui.Text(c, item.k).FontSize(10).TextColor(tk.TextMuted).SingleLine()
												vt := ui.Text(c, item.val).FontSize(m.FontSM).FontWeight(500).SingleLine()
												if item.mono {
													vt.Font("monospace")
												}
												if isCopied {
													vt.TextColor(tk.Success)
												} else {
													vt.TextColor(tk.TextPrimary)
												}
											})
											bVar, bLbl, bIc := btnGhost, locales.Copy(), icons.Copy
											if isCopied {
												bVar, bLbl, bIc = btnSuccess, locales.Copied(), icons.Check
											}
											if toolBtn(c, toolBtnOpts{Key: rKey + "-btn", Label: bLbl, Icon: bIc, Variant: bVar, Height: 24, PadX: 8}).Clicked() {
												c.Services().WriteClipboard(item.val)
												st.copiedKey = rKey
											}
										})
									}
								})
							})
						})
					})

					// 右卡：日期时间 -> 时间戳
					ui.Box(c).Grow(1).Basis(0).Children(func() {
						toolCard(c, "dt-to-ts-card", tk.Border, tk.BgApp, func() {
							ui.Text(c, locales.DtToTsTitle()).FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary)
						}, func() {
							toolPill(c, tz.offset, pillNeutral, false)
						}, func() {
							ui.Column(c).FillWidth().Padding(16).Gap(14).Children(func() {
								in := ui.TextInputBase(c.Key("dt-to-ts-input"), &st.dtInput).
									FillWidth().Height(36).Padding(0, 12).Radius(m.RadiusMD).
									Background(tk.BgBase).Border(m.Border, tk.Border).
									Font("monospace").FontSize(13).TextColor(tk.TextPrimary).
									Placeholder("YYYY-MM-DD HH:mm:ss").PlaceholderColor(tk.TextMuted).
									Label("Date time input")
								if in.Focused() {
									in.Border(m.Border, tk.Focus)
								}
								if in.Changed() || in.Submitted() {
									st.activePreset = -1
									convertDT()
								}

								// 快捷预设胶囊
								presets := []struct {
									label string
									calc  func(now time.Time) time.Time
								}{
									{locales.DtPresetToday(), func(n time.Time) time.Time {
										return time.Date(n.Year(), n.Month(), n.Day(), 0, 0, 0, 0, tz.loc)
									}},
									{locales.DtPresetTomorrow(), func(n time.Time) time.Time {
										return time.Date(n.Year(), n.Month(), n.Day()+1, 0, 0, 0, 0, tz.loc)
									}},
									{locales.DtPresetYearEnd(), func(n time.Time) time.Time {
										return time.Date(n.Year(), 12, 31, 23, 59, 59, 0, tz.loc)
									}},
									{locales.DtPresetPlus1H(), func(n time.Time) time.Time { return n.Add(time.Hour) }},
									{locales.DtPresetPlus7D(), func(n time.Time) time.Time { return n.AddDate(0, 0, 7) }},
								}
								ui.Row(c).FillWidth().Gap(6).Children(func() {
									for i, p := range presets {
										idx, pr := i, p
										v := btnSecondary
										if st.activePreset == idx {
											v = btnActive
										}
										if toolBtn(c, toolBtnOpts{Key: fmt.Sprintf("dt-pre-%d", idx), Label: pr.label, Variant: v, Height: 24, PadX: 8}).Clicked() {
											target := pr.calc(time.Now().In(tz.loc))
											info := devtools.FromTimeInZone(target, tz.loc)
											st.dtInput = info.Local
											st.dtRes = info
											st.dtErr = ""
											st.activePreset = idx
										}
									}
								})

								if st.dtErr != "" {
									toolInlineErrorCard(c, "dt-err", locales.DtParseErrTitle(), st.dtErr, "", nil)
								}

								// 秒 / 毫秒结果卡
								for _, resItem := range []struct {
									key   string
									label string
									val   string
									hl    bool
								}{
									{"dt-res-sec", locales.DtResSec(), strconv.FormatInt(st.dtRes.Seconds, 10), true},
									{"dt-res-ms", locales.DtResMs(), strconv.FormatInt(st.dtRes.Milliseconds, 10), false},
								} {
									it := resItem
									isCopied := st.copiedKey == it.key
									bg := tk.BgApp
									bc := tk.Border
									vc := tk.TextPrimary
									if it.hl {
										bg = tk.AccentSoft
										bc = tk.Accent.Alpha(0.32)
										vc = tk.Accent
									}
									ui.Row(c).FillWidth().MinHeight(66).Padding(12, 14).Gap(10).
										AlignItems(ui.Center).Justify(ui.SpaceBetween).
										Radius(m.RadiusMD).Background(bg).Border(m.Border, bc).Children(func() {
										ui.Column(c).Gap(4).Children(func() {
											ui.Text(c, it.label).FontSize(m.FontXS).TextColor(tk.TextSecondary)
											ui.Text(c, it.val).Font("monospace").FontSize(18).FontWeight(600).TextColor(vc).SingleLine()
										})
										bVar, bLbl, bIc := btnSecondary, locales.Copy(), icons.Copy
										if it.hl {
											bVar = btnPrimary
										}
										if isCopied {
											bVar, bLbl, bIc = btnSuccess, locales.Copied(), icons.Check
										}
										if toolBtn(c, toolBtnOpts{Key: it.key + "-copy", Label: bLbl, Icon: bIc, Variant: bVar}).Clicked() {
											c.Services().WriteClipboard(it.val)
											st.copiedKey = it.key
										}
									})
								}

								// 距当前时间差值条
								ui.Row(c).FillWidth().Height(38).Padding(0, 12).AlignItems(ui.Center).Justify(ui.SpaceBetween).
									Radius(m.RadiusMD).Background(tk.BgApp).Border(m.Border, tk.Border).Children(func() {
									ui.Text(c, locales.DtDiffFromNow()).FontSize(m.FontXS).TextColor(tk.TextSecondary)
									ui.Text(c, st.dtRes.Relative).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
								})
							})
						})
					})
				})
			})
		})
	})
}

// ---- 5B. UUID / 高熵随机串生成 ----

const (
	idModeUUIDv4 = 0
	idModeUUIDv7 = 1
	idModeULID   = 2
	idModeRandom = 3
)

var idCounts = []int{1, 5, 10, 20}

type uuidToolState struct {
	mode             int
	countIdx         int // index into idCounts (default 1 -> 5)
	upper            bool
	noHyphens        bool
	randLen          float64
	optUpper         bool
	optLower         bool
	optDigits        bool
	optSymbols       bool
	optNoAmbiguous   bool
	items            []string
	copiedKey        string
}

func renderUUIDTool(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	root := ui.Column(c.Key("tool-uuid-root")).Grow(1).FillWidth()

	generate := func(st *uuidToolState) {
		n := idCounts[st.countIdx]
		out := make([]string, n)
		for i := 0; i < n; i++ {
			switch st.mode {
			case idModeUUIDv4:
				out[i] = devtools.FormatUUID(devtools.NewUUIDv4(), st.upper, st.noHyphens)
			case idModeUUIDv7:
				out[i] = devtools.FormatUUID(devtools.NewUUIDv7(), st.upper, st.noHyphens)
			case idModeULID:
				u := devtools.NewULID()
				if !st.upper {
					u = strings.ToLower(u)
				}
				out[i] = u
			case idModeRandom:
				out[i] = devtools.GenerateRandomString(devtools.RandomStringOpts{
					Length:           int(math.Round(st.randLen)),
					Upper:            st.optUpper,
					Lower:            st.optLower,
					Digits:           st.optDigits,
					Symbols:          st.optSymbols,
					ExcludeAmbiguous: st.optNoAmbiguous,
				})
			}
		}
		st.items = out
		st.copiedKey = ""
	}

	st := *ui.Local(root, "uuid-state-v2", func() *uuidToolState {
		s := &uuidToolState{
			mode:           idModeUUIDv4,
			countIdx:       1, // 5 items
			randLen:        32,
			optUpper:       true,
			optLower:       true,
			optDigits:      true,
			optSymbols:     true,
			optNoAmbiguous: true,
		}
		generate(s)
		return s
	})

	isRand := st.mode == idModeRandom
	count := idCounts[st.countIdx]

	root.Children(func() {
		// 1. Header (48px)
		toolHeader(c, icons.Sparkles, locales.ToolUUIDTitle(), locales.ToolUUIDDesc(), func() {
			if toolBtn(c, toolBtnOpts{Key: "uuid-regen", Label: locales.UuidRegenerate(), Icon: icons.Sparkles, Variant: btnPrimary, Height: 26}).Clicked() {
				generate(st)
			}
		})

		// 2. Toolbar (44px)
		toolToolbar(c, func() {
			toolSegmented(c, "uuid-mode", []string{"UUID v4", locales.UuidModeV7(), "ULID", locales.UuidModeRandom()}, st.mode, func(idx int) {
				st.mode = idx
				generate(st)
			})
			toolDividerV(c)
			ui.Text(c, locales.UuidCountLabel()).FontSize(m.FontXS).TextColor(tk.TextSecondary)
			toolSegmented(c, "uuid-count", []string{"1", "5", "10", "20"}, st.countIdx, func(idx int) {
				st.countIdx = idx
				generate(st)
			})
			if !isRand {
				uVar := btnSecondary
				if st.upper {
					uVar = btnActive
				}
				if toolBtn(c, toolBtnOpts{Key: "uuid-upper", Label: locales.UuidUpperBtn(), Variant: uVar}).Clicked() {
					st.upper = !st.upper
					generate(st)
				}
				if st.mode != idModeULID {
					nhVar := btnSecondary
					if st.noHyphens {
						nhVar = btnActive
					}
					if toolBtn(c, toolBtnOpts{Key: "uuid-nohyphen", Label: locales.UuidNoHyphens(), Variant: nhVar}).Clicked() {
						st.noHyphens = !st.noHyphens
						generate(st)
					}
				}
			}
		}, func() {
			allCopied := st.copiedKey == "uuid-all"
			v := btnSecondary
			lbl := locales.UuidCopyAll(strconv.Itoa(count))
			ic := icons.Copy
			if allCopied {
				v = btnSuccess
				lbl = locales.UuidCopiedAll(strconv.Itoa(count))
				ic = icons.Check
			}
			if toolBtn(c, toolBtnOpts{Key: "uuid-copy-all", Label: lbl, Icon: ic, Variant: v}).Clicked() {
				c.Services().WriteClipboard(strings.Join(st.items, "\n"))
				st.copiedKey = "uuid-all"
			}
		})

		// 3. 主体滚动区
		ui.Scroll(c.Key("uuid-scroll")).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(18, 24).Gap(14).Children(func() {
				// 随机字符串配置卡片（长度滑块 + 熵值指示 + 字符集复选）
				if isRand {
					opts := devtools.RandomStringOpts{
						Length:           int(math.Round(st.randLen)),
						Upper:            st.optUpper,
						Lower:            st.optLower,
						Digits:           st.optDigits,
						Symbols:          st.optSymbols,
						ExcludeAmbiguous: st.optNoAmbiguous,
					}
					bits := devtools.CalcEntropyBits(opts)
					strengthLbl, strengthVar := locales.UuidStrengthStrong(), pillSuccess
					if bits < 64 {
						strengthLbl, strengthVar = locales.UuidStrengthWeak(), pillWarning
					} else if bits < 128 {
						strengthLbl, strengthVar = locales.UuidStrengthMedium(), pillAccent
					}

					ui.Column(c).FillWidth().Padding(14, 16).Gap(12).Radius(m.RadiusLG).
						Background(tk.BgApp).Border(m.Border, tk.Border).Children(func() {
						ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
							ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
								ui.Text(c, locales.UuidLengthLabel()).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
								ui.Box(c).Height(28).Padding(0, 10).Radius(m.RadiusSM).Center().
									Background(tk.BgBase).Border(m.Border, tk.Accent).Children(func() {
									ui.Text(c, fmt.Sprintf("%d", int(math.Round(st.randLen)))).
										Font("monospace").FontSize(13).FontWeight(600).TextColor(tk.Accent)
								})
								sl := ui.Slider(c.Key("uuid-rand-slider"), &st.randLen, 8, 128).Width(220).Label("Random string length")
								if sl.Changed() {
									generate(st)
								}
							})
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								ui.Icon(c, icons.Lock).FontSize(13).TextColor(tk.Success)
								ui.Text(c, locales.UuidEntropyLabel(fmt.Sprintf("%.0f", bits))).
									Font("monospace").FontSize(m.FontXS).FontWeight(500).TextColor(tk.Success)
								toolPill(c, strengthLbl, strengthVar, true)
							})
						})

						ui.Row(c).FillWidth().Gap(8).AlignItems(ui.Center).Children(func() {
							ui.Text(c, locales.UuidCharsetLabel()).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary)
							toolCheckboxChip(c, "rand-up", locales.UuidCharUpper(), st.optUpper, func() { st.optUpper = !st.optUpper; generate(st) })
							toolCheckboxChip(c, "rand-lo", locales.UuidCharLower(), st.optLower, func() { st.optLower = !st.optLower; generate(st) })
							toolCheckboxChip(c, "rand-dg", locales.UuidCharDigits(), st.optDigits, func() { st.optDigits = !st.optDigits; generate(st) })
							toolCheckboxChip(c, "rand-sy", locales.UuidCharSymbols(), st.optSymbols, func() { st.optSymbols = !st.optSymbols; generate(st) })
							toolCheckboxChip(c, "rand-na", locales.UuidCharNoAmbig(), st.optNoAmbiguous, func() { st.optNoAmbiguous = !st.optNoAmbiguous; generate(st) })
						})
					})
				}

				// 生成结果列表卡片
				itemCountStr := strconv.Itoa(len(st.items))
				listTitle := locales.UuidListV4(itemCountStr)
				badgeText := "v4 · Variant 1"
				switch st.mode {
				case idModeUUIDv7:
					listTitle = locales.UuidListV7(itemCountStr)
					badgeText = "v7 · Time-Ordered"
				case idModeULID:
					listTitle = locales.UuidListULID(itemCountStr)
					badgeText = "ULID · 128-bit"
				case idModeRandom:
					listTitle = locales.UuidListRandom(itemCountStr, strconv.Itoa(int(math.Round(st.randLen))))
					badgeText = fmt.Sprintf("%d chars", int(math.Round(st.randLen)))
				}

				toolCard(c, "uuid-list-card", tk.Border, tk.BgApp, func() {
					ui.Text(c, listTitle).FontSize(m.FontXS).FontWeight(600).TextColor(tk.TextSecondary)
				}, func() {
					ui.Text(c, locales.UuidListHint()).FontSize(m.FontXS).TextColor(tk.TextMuted)
				}, func() {
					for i, val := range st.items {
						idx, item := i, val
						rKey := fmt.Sprintf("uuid-row-%d", idx)
						isCopied := st.copiedKey == rKey
						r := ui.Row(c.Key(rKey)).FillWidth().MinHeight(46).Padding(6, 16).Gap(12).
							AlignItems(ui.Center).Justify(ui.SpaceBetween)
						if idx > 0 {
							r.BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border)
						}
						if isCopied {
							r.Background(tk.Success.Alpha(0.1))
						} else if r.Hovered() {
							r.Background(tk.BgHover)
						}
						r.Children(func() {
							ui.Row(c).Grow(1).Gap(14).AlignItems(ui.Center).Children(func() {
								ui.Text(c, fmt.Sprintf("%02d", idx+1)).Font("monospace").FontSize(m.FontXS).TextColor(tk.TextMuted).Width(20)
								ui.Text(c, item).Font("monospace").FontSize(13).FontWeight(500).TextColor(tk.TextPrimary).SingleLine().Grow(1)
							})
							ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
								toolPill(c, badgeText, pillNeutral, false)
								bVar, bLbl, bIc := btnGhost, locales.Copy(), icons.Copy
								if isCopied {
									bVar, bLbl, bIc = btnSuccess, locales.Copied(), icons.Check
								}
								if toolBtn(c, toolBtnOpts{Key: rKey + "-copy", Label: bLbl, Icon: bIc, Variant: bVar, Height: 24, PadX: 8}).Clicked() {
									c.Services().WriteClipboard(item)
									st.copiedKey = rKey
								}
							})
						})
					}
				})
			})
		})
	})
}

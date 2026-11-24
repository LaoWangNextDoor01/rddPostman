package app

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

type globalsDialogState struct {
	vars        []domain.KV
	initialized bool
	savedMsg    string
	errMsg      string
}

// globalsDialog 绘制跨工作区共享的 Globals 全局变量编辑弹窗。
func (a *App) globalsDialog(c *ui.Context, open *bool) {
	if open == nil || a.store == nil {
		return
	}
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		st := ui.Local(panel, "globals-editor-state", func() globalsDialogState {
			return globalsDialogState{}
		})
		if !st.initialized {
			st.initialized = true
			existing := a.store.Globals()
			if len(existing) == 0 {
				st.vars = []domain.KV{{ID: domain.NewID(), Enabled: true}}
			} else {
				st.vars = make([]domain.KV, len(existing))
				copy(st.vars, existing)
			}
		}

		backdrop.Background(tk.Overlay)
		panel.Width(720).Height(500).Radius(m.RadiusLG).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Shadow(0, 12, 32, 0, tk.Overlay).
			Clip()

		panel.Children(func() {
			ui.Column(c).Fill().Children(func() {
				// 1. 顶部标题栏（精简分层按钮组）
				ui.Row(c).FillWidth().Height(52).Padding(0, m.Space5).Gap(10).
					AlignItems(ui.Center).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
					ui.Box(c).Size(26, 26).Radius(m.RadiusMD).
						Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.28)).
						Center().Children(func() {
						ui.Icon(c, icons.Globe).FontSize(14).TextColor(tk.Accent)
					})
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
							ui.Text(c, locales.Globals()).FontSize(m.FontMD).FontWeight(600).
								TextColor(tk.TextPrimary).SingleLine()
							if st.savedMsg != "" {
								ui.Row(c).Height(18).PaddingX(6).Gap(4).AlignItems(ui.Center).
									Radius(m.RadiusFull).Background(tk.Success.Alpha(0.12)).
									Border(m.Border, tk.Success.Alpha(0.32)).Children(func() {
									ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.Success)
									ui.Text(c, st.savedMsg).FontSize(10).FontWeight(500).TextColor(tk.Success).SingleLine()
								})
							}
						})
						ui.Text(c, locales.GlobalsSubtitle()).
							FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
					})

					saveBtn := ui.ButtonBase(c.Key("globals-save")).
						Height(28).PaddingX(12).Gap(5).Radius(m.RadiusSM).
						AlignItems(ui.Center).Justify(ui.Center).
						Background(tk.Accent).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.SaveGlobalsLabel()).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					if saveBtn.Pressed() {
						saveBtn.Background(tk.AccentPressed)
					} else if saveBtn.Hovered() {
						saveBtn.Background(tk.AccentHover)
					}
					if saveBtn.Clicked() {
						var cleaned []domain.KV
						for _, v := range st.vars {
							if strings.TrimSpace(v.Key) != "" {
								cleaned = append(cleaned, v)
							}
						}
						if err := a.store.SetGlobals(cleaned); err != nil {
							logger.Error("保存全局变量失败: %v", err)
							st.errMsg = err.Error()
						} else {
							st.errMsg = ""
							st.savedMsg = locales.Saved()
						}
					}
					saveBtn.Children(func() {
						ui.Text(c, locales.Save()).FontSize(m.FontSM).FontWeight(500).
							TextColor(tk.OnAccent).SingleLine()
					})

					ui.Box(c).Size(1, 16).Background(tk.Border)

					closeBtn := ui.ButtonBase(c.Key("globals-close")).
						Size(28, 28).Radius(m.RadiusSM).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.CloseGlobalsLabel()).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					if closeBtn.Hovered() {
						closeBtn.Background(tk.BgHover)
					}
					if closeBtn.Clicked() {
						*open = false
						st.initialized = false
						st.savedMsg = ""
						st.errMsg = ""
					}
					closeBtn.Children(func() {
						ui.Icon(c, icons.X).FontSize(m.IconSM).TextColor(tk.TextSecondary)
					})
				})

				if st.errMsg != "" {
					ui.Row(c).FillWidth().Padding(m.Space2, m.Space5).Background(tk.DangerSoft).Children(func() {
						ui.Text(c, st.errMsg).FontSize(m.FontXS).TextColor(tk.Danger)
					})
				}

				// 2. 电子表格变量表（居中卡片 + 单元格细线分割）
				ui.Scroll(c).Grow(1).FillWidth().Children(func() {
					ui.Column(c).FillWidth().Padding(14, m.Space5, 16, m.Space5).Children(func() {
						ui.Column(c).FillWidth().Radius(m.RadiusMD).
							Background(tk.BgBase).Border(m.Border, tk.Border).Clip().Children(func() {
							// 表头
							ui.Row(c).FillWidth().Height(30).AlignItems(ui.Center).
								Background(tk.BgApp).
								BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
								ui.Box(c).Width(34).FillHeight().BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
								ui.Row(c).Width(180).FillHeight().Padding(0, 10).AlignItems(ui.Center).
									BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
									ui.Text(c, locales.ColVariable()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary)
								})
								ui.Row(c).Grow(1).FillHeight().Padding(0, 10).AlignItems(ui.Center).
									BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
									ui.Text(c, locales.ColValue()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary)
								})
								ui.Row(c).Width(160).FillHeight().Padding(0, 10).AlignItems(ui.Center).
									BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
									ui.Text(c, locales.ColDescription()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary)
								})
								ui.Box(c).Width(34)
							})

							if len(st.vars) == 0 {
								ui.Column(c).FillWidth().Padding(28, m.Space5).Center().Children(func() {
									ui.Text(c, locales.NoGlobalVars()).FontSize(m.FontSM).TextColor(tk.TextMuted)
								})
							} else {
								pendingDeleteIdx := -1
								for i := range st.vars {
									idx := i
									item := &st.vars[idx]
									rowKey := fmt.Sprintf("glob-kv-%d-%s", idx, item.ID)

									row := ui.Row(c.Key(rowKey)).FillWidth().Height(32).AlignItems(ui.Center).
										BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
									if row.Hovered() {
										row.Background(tk.BgApp)
									}
									row.Children(func() {
										ui.Row(c).Width(34).FillHeight().Center().
											BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
											checked := item.Enabled
											cb := ui.CheckboxBase(c.Key(rowKey+"-chk"), &checked).
												Cursor(ui.CursorPointer).Label("Enable global " + item.Key)
											cb.OnClick(func() {
												item.Enabled = checked
												st.savedMsg = ""
											})
											cb.Children(func() {
												box := ui.Box(c).Size(15, 15).Radius(3).Center()
												if item.Enabled {
													box.Background(tk.Accent).Children(func() {
														ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.OnAccent)
													})
												} else {
													box.Background(tk.BgSurface).Border(1, tk.BorderStrong)
												}
											})
										})

										keyCell := ui.Row(c).Width(180).FillHeight().AlignItems(ui.Center).
											BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
										if keyCell.FocusWithin() {
											keyCell.Background(tk.AccentSoft.Alpha(0.45))
										}
										keyCell.Children(func() {
											keyIn := ui.TextInputBase(c.Key(rowKey+"-key"), &item.Key).
												Fill().Padding(0, 10).
												Font("monospace").FontSize(m.FontSM).TextColor(tk.TextPrimary).
												Placeholder(locales.GlobalVarPlaceholder()).PlaceholderColor(tk.TextMuted).
												Label(locales.GlobalVarKeyLabel())
											if keyIn.Changed() {
												st.savedMsg = ""
											}
										})

										valCell := ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Center).
											BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
										if valCell.FocusWithin() {
											valCell.Background(tk.AccentSoft.Alpha(0.45))
										}
										valCell.Children(func() {
											valIn := ui.TextInputBase(c.Key(rowKey+"-val"), &item.Value).
												Fill().Padding(0, 10).
												Font("monospace").FontSize(m.FontSM).TextColor(tk.TextPrimary).
												Placeholder(locales.ValueLabel()).PlaceholderColor(tk.TextMuted).
												Label(locales.GlobalVarValLabel())
											if valIn.Changed() {
												st.savedMsg = ""
											}
										})

										descCell := ui.Row(c).Width(160).FillHeight().AlignItems(ui.Center).
											BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
										if descCell.FocusWithin() {
											descCell.Background(tk.AccentSoft.Alpha(0.45))
										}
										descCell.Children(func() {
											descIn := ui.TextInputBase(c.Key(rowKey+"-desc"), &item.Desc).
												Fill().Padding(0, 10).
												FontSize(m.FontSM).TextColor(tk.TextSecondary).
												Placeholder(locales.OptionalShort()).PlaceholderColor(tk.TextMuted).
												Label(locales.GlobalVarDescLabel())
											if descIn.Changed() {
												st.savedMsg = ""
											}
										})

										ui.Row(c).Width(34).FillHeight().Center().Children(func() {
											delBtn := ui.ButtonBase(c.Key(rowKey+"-del")).
												Size(24, 24).Radius(m.RadiusSM).Center().
												Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.DeleteRowLabel())
											icCol := tk.TextMuted
											if delBtn.Hovered() {
												delBtn.Background(tk.DangerSoft)
												icCol = tk.Danger
											}
											if delBtn.Clicked() {
												pendingDeleteIdx = idx
											}
											delBtn.Children(func() {
												ui.Icon(c, icons.X).FontSize(m.IconXS).TextColor(icCol)
											})
										})
									})
								}
								if pendingDeleteIdx >= 0 && pendingDeleteIdx < len(st.vars) {
									st.vars = append(st.vars[:pendingDeleteIdx], st.vars[pendingDeleteIdx+1:]...)
									st.savedMsg = ""
									c.Invalidate()
								}
							}

							// 表尾内联新增行
							addBtn := ui.ButtonBase(c.Key("globals-add-var")).
								FillWidth().Height(32).Padding(0, 12).Gap(6).
								AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.AddVariableBtn())
							if addBtn.Hovered() {
								addBtn.Background(tk.BgApp)
							}
							if addBtn.Clicked() {
								st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
								st.savedMsg = ""
							}
							addBtn.Children(func() {
								fg := tk.TextSecondary
								if addBtn.Hovered() {
									fg = tk.Accent
								}
								ui.Icon(c, icons.Plus).FontSize(12).TextColor(fg)
								ui.Text(c, locales.AddVariable()).FontSize(m.FontXS).FontWeight(500).
									TextColor(fg).SingleLine()
							})
						})
					})
				})
			})
		})
	})
}

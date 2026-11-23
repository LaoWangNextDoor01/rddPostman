package app

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"

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
				// 1. 顶部标题栏
				ui.Row(c).FillWidth().Height(54).Padding(0, m.Space5).Gap(m.Space2).
					AlignItems(ui.Center).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
					ui.Box(c).Size(28, 28).Radius(m.RadiusSM).Background(tk.AccentSoft).Center().Children(func() {
						ui.Icon(c, icons.Globe).FontSize(m.IconSM).TextColor(tk.Accent)
					})
					ui.Column(c).Grow(1).Gap(1).Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
							ui.Text(c, "Globals").FontSize(m.FontLG).FontWeight(600).
								TextColor(tk.TextPrimary).SingleLine()
							if st.savedMsg != "" {
								ui.Text(c, st.savedMsg).FontSize(m.FontXS).TextColor(tk.Success).SingleLine()
							}
						})
						ui.Text(c, "Shared variables accessible in every workspace via {{variable}}.").
							FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
					})

					addBtn := ui.ButtonBase(c.Key("globals-add-var")).
						Height(28).PaddingX(10).Radius(m.RadiusSM).Center().
						Border(m.Border, tk.BorderPopover).Background(tk.BgSurface).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("+ Add Variable")
					if addBtn.Hovered() {
						addBtn.Background(tk.BgHover)
					}
					if addBtn.Clicked() {
						st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
						st.savedMsg = ""
					}
					addBtn.Children(func() {
						ui.Text(c, "+ Add Variable").FontSize(m.FontXS).FontWeight(500).
							TextColor(tk.TextPrimary).SingleLine()
					})

					saveBtn := ui.ButtonBase(c.Key("globals-save")).
						Height(28).PaddingX(12).Radius(m.RadiusSM).Center().
						Background(tk.Accent).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Save Globals")
					if saveBtn.Hovered() {
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
							st.savedMsg = "Saved"
						}
					}
					saveBtn.Children(func() {
						ui.Text(c, "Save").FontSize(m.FontXS).FontWeight(500).
							TextColor(tk.OnAccent).SingleLine()
					})

					closeBtn := ui.ButtonBase(c.Key("globals-close")).
						Size(28, 28).Radius(m.RadiusSM).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Close globals")
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

				// 2. 表头
				ui.Row(c).FillWidth().Height(32).Padding(0, m.Space5).Gap(m.Space2).
					AlignItems(ui.Center).Background(tk.BgApp).
					BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
					ui.Text(c, "").Width(24)
					ui.Text(c, "VARIABLE").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Width(180)
					ui.Text(c, "VALUE").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Grow(1)
					ui.Text(c, "DESCRIPTION").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Width(160)
					ui.Text(c, "").Width(28)
				})

				// 3. 变量编辑列表
				ui.Scroll(c).Grow(1).FillWidth().Children(func() {
					ui.Column(c).FillWidth().Children(func() {
						if len(st.vars) == 0 {
							ui.Column(c).FillWidth().Padding(32, m.Space5).Center().Children(func() {
								ui.Text(c, "No global variables yet.").FontSize(m.FontSM).TextColor(tk.TextMuted)
							})
							return
						}
						for i := range st.vars {
							idx := i
							item := &st.vars[idx]
							rowKey := fmt.Sprintf("glob-kv-%d-%s", idx, item.ID)

							ui.Row(c.Key(rowKey)).FillWidth().Padding(m.Space2, m.Space5).Gap(m.Space2).
								AlignItems(ui.Center).
								BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
								checked := item.Enabled
								cb := ui.CheckboxBase(c.Key(rowKey+"-chk"), &checked).
									Cursor(ui.CursorPointer).Label("Enable global " + item.Key)
								cb.OnClick(func() {
									item.Enabled = checked
									st.savedMsg = ""
								})
								cb.Children(func() {
									box := ui.Box(c).Size(16, 16).Radius(4).Center()
									if item.Enabled {
										box.Background(tk.Accent).Border(1, tk.Accent).Children(func() {
											ui.Icon(c, icons.Check).FontSize(11).TextColor(tk.OnAccent)
										})
									} else {
										box.Background(tk.BgSurface).Border(1, tk.BorderStrong)
									}
								})

								keyIn := ui.TextInputBase(c.Key(rowKey+"-key"), &item.Key).
									Width(180).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
									Background(tk.BgSurface).Border(1, tk.Border).
									FontSize(m.FontSM).TextColor(tk.TextPrimary).
									Placeholder("e.g. apiHost").PlaceholderColor(tk.TextMuted).
									Label("Global variable key")
								if keyIn.Focused() {
									keyIn.Border(1, tk.Focus)
								}
								if keyIn.Changed() {
									st.savedMsg = ""
								}

								valIn := ui.TextInputBase(c.Key(rowKey+"-val"), &item.Value).
									Grow(1).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
									Background(tk.BgSurface).Border(1, tk.Border).
									FontSize(m.FontSM).TextColor(tk.TextPrimary).
									Placeholder("Value").PlaceholderColor(tk.TextMuted).
									Label("Global variable value")
								if valIn.Focused() {
									valIn.Border(1, tk.Focus)
								}
								if valIn.Changed() {
									st.savedMsg = ""
								}

								descIn := ui.TextInputBase(c.Key(rowKey+"-desc"), &item.Desc).
									Width(160).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
									Background(tk.BgSurface).Border(1, tk.Border).
									FontSize(m.FontSM).TextColor(tk.TextPrimary).
									Placeholder("Optional").PlaceholderColor(tk.TextMuted).
									Label("Global variable description")
								if descIn.Focused() {
									descIn.Border(1, tk.Focus)
								}
								if descIn.Changed() {
									st.savedMsg = ""
								}

								delBtn := ui.ButtonBase(c.Key(rowKey+"-del")).
									Size(28, 28).Radius(m.RadiusSM).Center().
									Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Delete global row")
								if delBtn.Hovered() {
									delBtn.Background(tk.BgHover)
								}
								delBtn.OnClick(func() {
									if idx >= 0 && idx < len(st.vars) {
										st.vars = append(st.vars[:idx], st.vars[idx+1:]...)
										st.savedMsg = ""
									}
								})
								delBtn.Children(func() {
									ui.Icon(c, icons.X).FontSize(m.IconXS).TextColor(tk.TextMuted)
								})
							})
						}
					})
				})
			})
		})
	})
}

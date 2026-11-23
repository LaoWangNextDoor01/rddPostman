package environment

import (
	"fmt"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

type envEditorState struct {
	vars        []domain.KV
	initialized bool
	savedMsg    string
	errMsg      string
}

// EditorView 在 WorkbenchLayout 右侧多 Tab 区渲染环境变量键值编辑器。
func EditorView(c *ui.Context, db *store.Store, wsID, envID domain.ID) {
	m, tk := thV2.Metrics, thV2.Of(c)

	if db == nil || wsID == "" || envID == "" {
		return
	}
	snap := db.Snapshot()
	ws := snap.Workspace(wsID)
	if ws == nil {
		return
	}
	var currentEnv *domain.Environment
	for i := range ws.Environments {
		if ws.Environments[i].ID == envID {
			e := ws.Environments[i]
			currentEnv = &e
			break
		}
	}
	if currentEnv == nil {
		ui.Column(c).Fill().Center().Background(tk.BgBase).Gap(m.Space2).Children(func() {
			ui.Icon(c, icons.Globe).FontSize(24).TextColor(tk.TextMuted)
			ui.Text(c, "Environment not found").FontSize(m.FontMD).TextColor(tk.TextSecondary)
		})
		return
	}

	isActive := ws.SelectedEnvironmentID == envID
	root := ui.Column(c.Key("env-tab-editor-" + string(envID))).Fill().Background(tk.BgBase)
	st := ui.Local(root, "state", func() envEditorState {
		return envEditorState{}
	})
	deleteOpen := ui.Local(root, "env-ed-del-open", func() bool { return false })
	if !st.initialized {
		st.initialized = true
		if len(currentEnv.Vars) == 0 {
			st.vars = []domain.KV{{ID: domain.NewID(), Enabled: true}}
		} else {
			st.vars = make([]domain.KV, len(currentEnv.Vars))
			copy(st.vars, currentEnv.Vars)
		}
	}

	root.Children(func() {
		// 1. 顶部栏：环境名 + 激活态 + 操作按钮
		ui.Row(c).FillWidth().Padding(m.Space4, m.Space5).Gap(m.Space3).
			AlignItems(ui.Center).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
			ui.Box(c).Size(32, 32).Radius(m.RadiusMD).Background(tk.AccentSoft).Center().Children(func() {
				ui.Icon(c, icons.Globe).FontSize(m.IconMD).TextColor(tk.Accent)
			})
			ui.Column(c).Grow(1).Gap(2).Children(func() {
				ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
					ui.Text(c, currentEnv.Name).FontSize(m.FontLG).FontWeight(600).
						TextColor(tk.TextPrimary).SingleLine()
					if isActive {
						ui.Box(c).Padding(2, 8).Radius(m.RadiusFull).Background(tk.AccentSoft).Children(func() {
							ui.Text(c, "Active").FontSize(11).FontWeight(500).TextColor(tk.Accent).SingleLine()
						})
					}
					if st.savedMsg != "" {
						ui.Text(c, st.savedMsg).FontSize(m.FontXS).TextColor(tk.Success).SingleLine()
					}
				})
				ui.Text(c, "Use {{variable}} in request URL, params, headers, or body.").
					FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
			})

			// 设为当前激活环境
			if !isActive {
				actBtn := envSecondaryBtn(c, "env-ed-activate-"+string(envID), "Set Active")
				if actBtn.Clicked() {
					if err := db.SelectEnvironment(wsID, envID); err != nil {
						st.errMsg = err.Error()
					} else {
						st.errMsg = ""
					}
				}
			}

			// 添加变量行
			if envSecondaryBtn(c, "env-ed-add-"+string(envID), "+ Add Variable").Clicked() {
				st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
				st.savedMsg = ""
			}

			// 保存变量
			if envPrimaryBtn(c, "env-ed-save-"+string(envID), "Save").Clicked() {
				var cleaned []domain.KV
				for _, v := range st.vars {
					if strings.TrimSpace(v.Key) != "" {
						cleaned = append(cleaned, v)
					}
				}
				envCopy := *currentEnv
				envCopy.Vars = cleaned
				if err := db.SaveEnvironment(wsID, envCopy); err != nil {
					logger.Error("保存环境失败: %v", err)
					st.errMsg = err.Error()
				} else {
					st.errMsg = ""
					st.savedMsg = "Saved"
				}
			}

			// 删除环境（触发二次确认弹窗）
			if envDangerBtn(c, "env-ed-del-"+string(envID), "Delete").Clicked() {
				*deleteOpen = true
			}
		})

		if st.errMsg != "" {
			ui.Row(c).FillWidth().Padding(m.Space2, m.Space5).Background(tk.DangerSoft).Children(func() {
				ui.Text(c, st.errMsg).FontSize(m.FontXS).TextColor(tk.Danger)
			})
		}

		// 2. 变量表头
		ui.Row(c).FillWidth().Height(34).Padding(0, m.Space5).Gap(m.Space2).
			AlignItems(ui.Center).Background(tk.BgApp).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, "").Width(28)
			ui.Text(c, "VARIABLE").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Width(200)
			ui.Text(c, "VALUE").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Grow(1)
			ui.Text(c, "DESCRIPTION").FontSize(11).FontWeight(600).TextColor(tk.TextMuted).Width(200)
			ui.Text(c, "").Width(28)
		})

		// 3. 变量编辑行列表
		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				if len(st.vars) == 0 {
					ui.Column(c).FillWidth().Padding(32, m.Space5).Center().Gap(m.Space2).Children(func() {
						ui.Text(c, "No variables in this environment yet.").FontSize(m.FontSM).TextColor(tk.TextMuted)
					})
					return
				}
				for i := range st.vars {
					idx := i
					envVarRow(c, st, idx)
				}
			})
		})
	})

	v2.ConfirmDialog(c, deleteOpen, v2.ConfirmDialogOpts{
		Key:          "env-ed-del-confirm-" + string(envID),
		Title:        "删除环境",
		Description:  fmt.Sprintf("确定要删除环境「%s」及其全部环境变量吗？此操作不可撤销。", currentEnv.Name),
		ConfirmLabel: "确认删除",
		CancelLabel:  "取消",
		Danger:       true,
		OnConfirm: func() {
			if err := db.DeleteEnvironment(wsID, envID); err != nil {
				st.errMsg = err.Error()
			} else {
				st.errMsg = ""
				c.Invalidate()
			}
		},
	})
}

func envVarRow(c *ui.Context, st *envEditorState, idx int) {
	m, tk := thV2.Metrics, thV2.Of(c)
	item := &st.vars[idx]
	rowKey := fmt.Sprintf("env-kv-%d-%s", idx, item.ID)

	ui.Row(c.Key(rowKey)).FillWidth().Padding(m.Space2, m.Space5).Gap(m.Space2).
		AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		// 启用复选框
		ui.Row(c).Width(28).AlignItems(ui.Center).Children(func() {
			checked := item.Enabled
			cb := ui.CheckboxBase(c.Key(rowKey+"-chk"), &checked).
				Cursor(ui.CursorPointer).Label("Enable variable " + item.Key)
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
					bc := tk.BorderStrong
					if cb.Hovered() {
						bc = tk.Focus
					}
					box.Background(tk.BgSurface).Border(1, bc)
				}
			})
		})

		// Key
		keyIn := ui.TextInputBase(c.Key(rowKey+"-key"), &item.Key).
			Width(200).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
			Background(tk.BgSurface).FontSize(m.FontSM).TextColor(tk.TextPrimary).
			Placeholder("e.g. baseUrl").PlaceholderColor(tk.TextMuted).
			Label("Variable key")
		styleEnvInput(keyIn, tk)
		if keyIn.Changed() {
			st.savedMsg = ""
		}

		// Value
		valIn := ui.TextInputBase(c.Key(rowKey+"-val"), &item.Value).
			Grow(1).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
			Background(tk.BgSurface).FontSize(m.FontSM).TextColor(tk.TextPrimary).
			Placeholder("e.g. https://api.example.com").PlaceholderColor(tk.TextMuted).
			Label("Variable value")
		styleEnvInput(valIn, tk)
		if valIn.Changed() {
			st.savedMsg = ""
		}

		// Desc
		descIn := ui.TextInputBase(c.Key(rowKey+"-desc"), &item.Desc).
			Width(200).Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
			Background(tk.BgSurface).FontSize(m.FontSM).TextColor(tk.TextPrimary).
			Placeholder("Optional description").PlaceholderColor(tk.TextMuted).
			Label("Variable description")
		styleEnvInput(descIn, tk)
		if descIn.Changed() {
			st.savedMsg = ""
		}

		// 删除行
		delBtn := ui.ButtonBase(c.Key(rowKey+"-del")).
			Size(28, 28).Radius(m.RadiusSM).Center().
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Delete variable row")
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

func styleEnvInput(in ui.Element, tk thV2.Tokens) {
	switch {
	case in.Focused():
		in.Border(1, tk.Focus).Shadow(0, 0, 0, 2, tk.FocusRing)
	case in.Hovered():
		in.Border(1, tk.BorderStrong)
	default:
		in.Border(1, tk.Border)
	}
}

func envPrimaryBtn(c *ui.Context, key, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(12).Radius(m.RadiusSM).Center().
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.AccentPressed)
	case b.Hovered():
		b.Background(tk.AccentHover)
	default:
		b.Background(tk.Accent)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(tk.OnAccent).SingleLine()
	})
	return b
}

func envSecondaryBtn(c *ui.Context, key, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(10).Radius(m.RadiusSM).Center().
		Border(m.Border, tk.BorderPopover).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	if b.Hovered() {
		b.Background(tk.BgHover)
	} else {
		b.Background(tk.BgSurface)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()
	})
	return b
}

func envDangerBtn(c *ui.Context, key, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(10).Radius(m.RadiusSM).Center().
		Border(m.Border, tk.BorderPopover).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	if b.Hovered() {
		b.Background(tk.DangerSoft)
	} else {
		b.Background(tk.BgSurface)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(tk.Danger).SingleLine()
	})
	return b
}

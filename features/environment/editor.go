package environment

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

type envVarFilter int

const (
	filterAll envVarFilter = iota
	filterEnabled
	filterSecret
)

type envEditorState struct {
	vars         []domain.KV
	initialized  bool
	dirty        bool
	savedMsg     string
	errMsg       string
	search       string
	filter       envVarFilter
	bulkMode     bool
	bulkText     string
	revealed     map[domain.ID]bool
	renaming     bool
	renameDraft  string
	moreMenuOpen bool
}

// EditorView 在 WorkbenchLayout 右侧多 Tab 区渲染重新设计后的环境变量编辑器：
//   - 48px 轻盈标题条：环境徽标、可内联改名标题、状态胶囊，以及高层次感的精简动作组（激活 / 保存 / 更多菜单）；
//   - 42px 控制工具条：搜索框 + 全量/启用/密钥分段过滤 + 批量编辑切换 + 添加变量；
//   - 紧凑电子表格风格变量表：无多余悬浮内框、支持普通/密钥切换与掩码显隐、表尾快捷加行；
//   - 26px 底部轻状态条：变量统计、引用提示与快捷键提示。
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
			ui.Text(c, locales.EnvNotFound()).FontSize(m.FontMD).TextColor(tk.TextSecondary)
		})
		return
	}

	isActive := ws.SelectedEnvironmentID == envID
	isDefault := ws.DefaultEnvironmentID == envID || len(ws.Environments) <= 1

	root := ui.Column(c.Key("env-tab-editor-" + string(envID))).Fill().Background(tk.BgBase)
	st := ui.Local(root, "state", func() envEditorState {
		return envEditorState{revealed: map[domain.ID]bool{}}
	})
	deleteOpen := ui.Local(root, "env-ed-del-open", func() bool { return false })
	if st.revealed == nil {
		st.revealed = map[domain.ID]bool{}
	}
	if !st.initialized {
		st.initialized = true
		st.renameDraft = currentEnv.Name
		if len(currentEnv.Vars) == 0 {
			st.vars = []domain.KV{{ID: domain.NewID(), Enabled: true}}
		} else {
			st.vars = make([]domain.KV, len(currentEnv.Vars))
			copy(st.vars, currentEnv.Vars)
		}
	}

	saveAll := func() {
		if st.bulkMode {
			st.vars = parseBulkEnvVars(st.bulkText, st.vars)
		}
		var cleaned []domain.KV
		for _, v := range st.vars {
			if strings.TrimSpace(v.Key) != "" {
				cleaned = append(cleaned, v)
			}
		}
		envCopy := *currentEnv
		if st.renaming {
			if trimmed := strings.TrimSpace(st.renameDraft); trimmed != "" {
				envCopy.Name = trimmed
			}
			st.renaming = false
		}
		envCopy.Vars = cleaned
		if err := db.SaveEnvironment(wsID, envCopy); err != nil {
			logger.Error("保存环境失败: %v", err)
			st.errMsg = err.Error()
		} else {
			st.errMsg = ""
			st.dirty = false
			st.savedMsg = locales.Saved()
		}
	}

	commitRename := func() {
		st.renaming = false
		trimmed := strings.TrimSpace(st.renameDraft)
		if trimmed == "" || trimmed == currentEnv.Name {
			st.renameDraft = currentEnv.Name
			return
		}
		envCopy := *currentEnv
		envCopy.Name = trimmed
		if err := db.SaveEnvironment(wsID, envCopy); err != nil {
			st.errMsg = err.Error()
			st.renameDraft = currentEnv.Name
		} else {
			st.errMsg = ""
		}
	}

	// 支持 Cmd/Ctrl + S 快捷保存环境变量
	if !*deleteOpen && c.Shortcut(ui.Cmd, ui.KeyS) {
		saveAll()
	}

	// 统计变量数（仅统计已填写 Key 的非空行）
	totalVars, enabledVars, secretVars := countEnvStats(st.vars)

	root.Children(func() {
		// 1. 顶部 48px 标题栏（左：徽标 + 名称 + 状态胶囊；右：克制分层的精简动作组）
		ui.Row(c).FillWidth().Height(48).Padding(0, m.Space5).Gap(10).
			AlignItems(ui.Center).Background(tk.BgBase).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
			// 环境徽标
			ui.Box(c).Size(26, 26).Radius(m.RadiusMD).
				Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.28)).
				Center().Children(func() {
				ui.Icon(c, icons.Globe).FontSize(14).TextColor(tk.Accent)
			})

			// 标题与状态胶囊区
			ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
				if st.renaming {
					nameIn := ui.TextInputBase(c.Key("env-ed-rename-"+string(envID)), &st.renameDraft).
						Width(220).Height(26).Padding(0, 8).Radius(m.RadiusSM).
						Background(tk.BgSurface).Border(m.Border, tk.Focus).
						FontSize(m.FontMD).FontWeight(600).TextColor(tk.TextPrimary).
						Label(locales.EnvNameLabel())
					if nameIn.Submitted() || nameIn.FocusLeft() {
						commitRename()
					}
				} else {
					titleBtn := ui.ButtonBase(c.Key("env-ed-title-"+string(envID))).
						Height(26).Padding(0, 6, 0, 2).Radius(m.RadiusSM).
						AlignItems(ui.Center).Cursor(ui.CursorPointer).
						Role(ui.RoleButton).Label(currentEnv.Name).Tooltip(locales.Rename()).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					if titleBtn.Hovered() {
						titleBtn.Background(tk.BgHover)
					}
					if titleBtn.Clicked() {
						st.renameDraft = currentEnv.Name
						st.renaming = true
					}
					titleBtn.Children(func() {
						ui.Text(c, currentEnv.Name).FontSize(m.FontMD).FontWeight(600).
							TextColor(tk.TextPrimary).SingleLine()
					})
				}

				if isActive {
					envStatusPill(c, locales.EnvActiveBadge(), tk.Success.Alpha(0.14), tk.Success.Alpha(0.35), tk.Success, true, nil)
				}
				switch {
				case st.dirty:
					envStatusPill(c, locales.EnvUnsaved(), tk.Warning.Alpha(0.14), tk.Warning.Alpha(0.35), tk.Warning, true, nil)
				case st.savedMsg != "":
					envStatusPill(c, st.savedMsg, tk.Success.Alpha(0.12), tk.Success.Alpha(0.32), tk.Success, false, icons.Check)
				}
			})

			// 右侧精简动作组：[设为激活(仅未激活时)] [保存(主蓝)] | [...]
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				if !isActive {
					if envSoftAccentBtn(c, "env-ed-activate-"+string(envID), icons.Check, locales.EnvSetActiveBtn()).Clicked() {
						if err := db.SelectEnvironment(wsID, envID); err != nil {
							st.errMsg = err.Error()
						} else {
							st.errMsg = ""
						}
					}
				}

				if envPrimaryBtn(c, "env-ed-save-"+string(envID), nil, locales.Save()).Clicked() {
					saveAll()
				}

				ui.Box(c).Size(1, 16).Background(tk.Border)

				moreBtn := v2.IconButton(c, v2.IconButtonOpts{
					Key:      "env-ed-more-" + string(envID),
					Label:    locales.MoreActions(),
					Tooltip:  locales.MoreActions(),
					Icon:     icons.Ellipsis,
					Size:     28,
					IconSize: 14,
					Radius:   m.RadiusSM,
					Selected: st.moreMenuOpen,
				})
				if moreBtn.Clicked() {
					st.moreMenuOpen = !st.moreMenuOpen
				}

				v2.Popover(c, moreBtn, &st.moreMenuOpen, v2.PopoverOpts{Width: 196}, func(panel ui.Element) {
					ui.Column(c).Padding(4).Gap(2).Children(func() {
						if !isActive {
							if v2.MenuButton(c, v2.MenuButtonProps{
								Icon:  icons.Check,
								Key:   "env-menu-act-" + string(envID),
								Label: locales.EnvSetActive(),
							}).Clicked() {
								if err := db.SelectEnvironment(wsID, envID); err != nil {
									st.errMsg = err.Error()
								} else {
									st.errMsg = ""
								}
								st.moreMenuOpen = false
							}
						}
						if v2.MenuButton(c, v2.MenuButtonProps{
							Icon:  icons.Copy,
							Key:   "env-menu-dup-" + string(envID),
							Label: locales.EnvDuplicate(),
						}).Clicked() {
							cloneName := nextClonedEnvName(ws.Environments, currentEnv.Name)
							if _, err := db.CloneEnvironment(wsID, envID, cloneName); err != nil {
								st.errMsg = err.Error()
							} else {
								st.errMsg = ""
							}
							st.moreMenuOpen = false
						}
						v2.PopoverDivider(c)
						delItem := v2.MenuButton(c, v2.MenuButtonProps{
							Icon:   icons.Trash,
							Key:    "env-menu-del-" + string(envID),
							Label:  locales.TreeDelete(),
							Danger: true,
						})
						if isDefault {
							delItem.Disabled(true).Opacity(0.45)
						} else if delItem.Clicked() {
							st.moreMenuOpen = false
							*deleteOpen = true
						}
					})
				})
			})
		})

		// 2. 42px 控制工具条（左：搜索框 + 分段过滤；右：批量编辑 + 添加变量）
		ui.Row(c).FillWidth().Height(42).Padding(0, m.Space5).Gap(10).
			AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Background(tk.BgApp).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
			// 左侧搜索与状态筛选
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				searchBox := ui.Row(c.Key("env-var-search-box-" + string(envID))).
					Width(220).Height(26).Padding(0, 8).Gap(6).
					AlignItems(ui.Center).Radius(m.RadiusMD).Background(tk.BgSurface).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case searchBox.FocusWithin():
					searchBox.Border(m.Border, tk.Focus)
				case searchBox.Hovered():
					searchBox.Border(m.Border, tk.BorderStrong)
				default:
					searchBox.Border(m.Border, tk.BorderPopover)
				}
				searchBox.Children(func() {
					ui.Icon(c, icons.Search).FontSize(12).TextColor(tk.TextMuted)
					ui.TextInputBase(c.Key("env-var-search-"+string(envID)), &st.search).Grow(1).
						FontSize(m.FontXS).TextColor(tk.TextPrimary).
						Placeholder(locales.EnvFilterVarsPlaceholder()).
						PlaceholderColor(tk.TextMuted).
						Label(locales.EnvFilterLabel())
					if st.search != "" {
						clr := v2.IconButton(c, v2.IconButtonOpts{
							Key:      "env-var-search-clr-" + string(envID),
							Label:    locales.ClearFilter(),
							Icon:     icons.X,
							Size:     16,
							IconSize: 10,
							Color:    tk.TextMuted,
						})
						if clr.Clicked() {
							st.search = ""
						}
					}
				})

				if !st.bulkMode {
					envFilterSegmented(c, "env-flt-"+string(envID), st, totalVars, enabledVars, secretVars)
				}
			})

			// 右侧表格操作：批量编辑切换 + 添加变量
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				bulkLabel := locales.ReqBulkEdit()
				bulkIcon := icons.Code
				if st.bulkMode {
					bulkLabel = locales.ReqKeyValueEdit()
					bulkIcon = icons.List
				}
				if envGhostBtn(c, "env-ed-bulk-"+string(envID), bulkIcon, bulkLabel, st.bulkMode).Clicked() {
					if !st.bulkMode {
						st.bulkText = formatBulkEnvVars(st.vars)
						st.bulkMode = true
					} else {
						st.vars = parseBulkEnvVars(st.bulkText, st.vars)
						st.bulkMode = false
						st.dirty = true
						st.savedMsg = ""
					}
				}

				if !st.bulkMode {
					if envSecondaryBtn(c, "env-ed-add-"+string(envID), icons.Plus, locales.AddVariable()).Clicked() {
						st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
						st.filter = filterAll
						st.search = ""
						st.dirty = true
						st.savedMsg = ""
					}
				}
			})
		})

		// 错误提示横幅
		if st.errMsg != "" {
			ui.Row(c).FillWidth().Padding(8, m.Space5).Gap(8).AlignItems(ui.Center).
				Background(tk.DangerSoft).BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Danger.Alpha(0.3)).Children(func() {
				ui.Icon(c, icons.TriangleAlert).FontSize(13).TextColor(tk.Danger)
				ui.Text(c, st.errMsg).FontSize(m.FontXS).TextColor(tk.Danger).Grow(1)
			})
		}

		// 3. 主体滚动区：电子表格风格一体化变量卡片
		ui.Scroll(c.Key("env-vars-scroll-" + string(envID))).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(16, m.Space5, 24, m.Space5).Children(func() {
				ui.Column(c.Key("env-table-card-" + string(envID))).
					FillWidth().Radius(m.RadiusMD).
					Background(tk.BgBase).Border(m.Border, tk.Border).Clip().Children(func() {
					if st.bulkMode {
						renderEnvBulkEditor(c, st, string(envID))
					} else {
						renderEnvKVTable(c, st, string(envID))
					}
				})
			})
		})

		// 4. 底部 26px 轻状态条
		saveKey := "Ctrl+S"
		if runtime.GOOS == "darwin" {
			saveKey = "⌘S"
		}
		ui.Row(c).FillWidth().Height(26).Padding(0, m.Space5).Gap(m.Space3).
			AlignItems(ui.Center).Justify(ui.SpaceBetween).
			Background(tk.BgApp).
			BorderWidth(m.Border, 0, 0, 0).BorderColor(tk.Border).Children(func() {
			summary := locales.EnvFooterSummary(
				strconv.Itoa(totalVars),
				strconv.Itoa(enabledVars),
				strconv.Itoa(secretVars),
			)
			ui.Text(c, summary).FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()

			ui.Row(c).Gap(14).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.EnvEditorHint()).FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
				ui.Text(c, locales.EnvSaveShortcutHint(saveKey)).FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
			})
		})
	})

	v2.ConfirmDialog(c, deleteOpen, v2.ConfirmDialogOpts{
		Key:          "env-ed-del-confirm-" + string(envID),
		Title:        locales.EnvDeleteTitle(),
		Description:  locales.EnvDeleteDesc(currentEnv.Name),
		ConfirmLabel: locales.ConfirmDelete(),
		CancelLabel:  locales.Cancel(),
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

func renderEnvBulkEditor(c *ui.Context, st *envEditorState, envKey string) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).FillWidth().Height(32).Padding(0, 12).AlignItems(ui.Center).
		Background(tk.BgApp).BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		ui.Text(c, locales.ReqBulkHeader()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary)
	})
	ui.Column(c).FillWidth().Padding(12).Gap(8).Children(func() {
		in := ui.TextAreaBase(c.Key("env-bulk-area-"+envKey), &st.bulkText).
			FillWidth().MinHeight(200).MaxHeight(420).Padding(8, 10).
			Radius(m.RadiusSM).Background(tk.BgSubtle).Border(m.Border, tk.Border).
			Font("monospace").FontSize(m.FontSM).FixedLineHeight(20).TextColor(tk.TextPrimary).
			Placeholder("baseUrl: https://api.example.com\n// disabledKey: value").
			PlaceholderColor(tk.TextMuted)
		if in.Focused() {
			in.Border(m.Border, tk.Focus)
		}
		if in.Changed() {
			st.dirty = true
			st.savedMsg = ""
		}
		ui.Text(c, locales.ReqBulkHint()).FontSize(m.FontXS).TextColor(tk.TextMuted)
	})
}

func renderEnvKVTable(c *ui.Context, st *envEditorState, envKey string) {
	m, tk := thV2.Metrics, thV2.Of(c)

	// 计算筛选后可见行下标
	kw := strings.ToLower(strings.TrimSpace(st.search))
	isFiltered := kw != "" || st.filter != filterAll
	visibleIndices := make([]int, 0, len(st.vars))
	for i := range st.vars {
		v := st.vars[i]
		if st.filter == filterEnabled && !v.Enabled {
			continue
		}
		if st.filter == filterSecret && !v.Secret {
			continue
		}
		if kw != "" {
			if !strings.Contains(strings.ToLower(v.Key), kw) &&
				!strings.Contains(strings.ToLower(v.Value), kw) &&
				!strings.Contains(strings.ToLower(v.Desc), kw) {
				continue
			}
		}
		visibleIndices = append(visibleIndices, i)
	}

	// 判断是否全部启用（用于表头全选复选框）
	allEnabled := len(st.vars) > 0
	for _, v := range st.vars {
		if !v.Enabled {
			allEnabled = false
			break
		}
	}

	// 表头（32px，浅灰底 + 单元格竖线分隔，与请求 Params 表保持统一质感）
	ui.Row(c).FillWidth().Height(32).AlignItems(ui.Center).
		Background(tk.BgApp).BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).Children(func() {
		// 全选复选框列
		ui.Row(c).Width(36).FillHeight().Center().
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			allChk := ui.ButtonBase(c.Key("env-chk-all-"+envKey)).
				Size(15, 15).Radius(3).Center().Cursor(ui.CursorPointer).
				Role(ui.RoleCheckBox).Label(locales.SelectAll()).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if allEnabled {
				allChk.Background(tk.Accent)
				allChk.Children(func() {
					ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.OnAccent)
				})
			} else {
				bc := tk.BorderStrong
				if allChk.Hovered() {
					bc = tk.Accent
				}
				allChk.Background(tk.BgSurface).Border(1, bc)
			}
			if allChk.Clicked() {
				next := !allEnabled
				for i := range st.vars {
					st.vars[i].Enabled = next
				}
				st.dirty = true
				st.savedMsg = ""
			}
		})

		// VARIABLE
		ui.Row(c).Width(210).FillHeight().Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.ColVariable()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary).SingleLine()
		})

		// TYPE
		ui.Row(c).Width(92).FillHeight().Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.EnvColType()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary).SingleLine()
		})

		// VALUE
		ui.Row(c).Grow(1).FillHeight().Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.ColValue()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary).SingleLine()
		})

		// DESCRIPTION
		ui.Row(c).Width(210).FillHeight().Padding(0, 10).AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			ui.Text(c, locales.ColDescription()).FontSize(11).FontWeight(600).TextColor(tk.TextSecondary).SingleLine()
		})

		// 删除列占位
		ui.Box(c).Width(36)
	})

	// 空态或过滤无结果
	if len(visibleIndices) == 0 {
		msg := locales.EnvEmptyVars()
		if isFiltered {
			msg = locales.EnvNoFilterMatch()
		}
		ui.Column(c).FillWidth().Padding(32, 16).Center().Gap(8).Children(func() {
			ui.Text(c, msg).FontSize(m.FontSM).TextColor(tk.TextMuted)
			if isFiltered {
				if envGhostBtn(c, "env-clear-filter-"+envKey, icons.X, locales.ClearFilter(), false).Clicked() {
					st.search = ""
					st.filter = filterAll
				}
			}
		})
	} else {
		var pendingDeleteIdx = -1
		for pos, idx := range visibleIndices {
			i := idx
			envVarSpreadsheetRow(c, st, envKey, i, pos < len(visibleIndices)-1 || !isFiltered, !isFiltered, func() {
				pendingDeleteIdx = i
			})
		}
		if pendingDeleteIdx >= 0 && pendingDeleteIdx < len(st.vars) {
			delete(st.revealed, st.vars[pendingDeleteIdx].ID)
			st.vars = append(st.vars[:pendingDeleteIdx], st.vars[pendingDeleteIdx+1:]...)
			st.dirty = true
			st.savedMsg = ""
			c.Invalidate()
		}
	}

	// 表尾内联「+ 添加新变量」快捷行
	if !isFiltered {
		addRow := ui.ButtonBase(c.Key("env-inline-add-"+envKey)).
			FillWidth().Height(32).Padding(0, 12).Gap(6).
			AlignItems(ui.Center).Justify(ui.Start).
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.EnvInlineAddRow()).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
		switch {
		case addRow.Pressed():
			addRow.Background(tk.BgSelected)
		case addRow.Hovered():
			addRow.Background(tk.BgApp)
		default:
			addRow.Background(tk.BgBase)
		}
		if addRow.Clicked() {
			st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
			st.dirty = true
			st.savedMsg = ""
		}
		addRow.Children(func() {
			icColor := tk.TextMuted
			txtColor := tk.TextSecondary
			if addRow.Hovered() {
				icColor = tk.Accent
				txtColor = tk.Accent
			}
			ui.Icon(c, icons.Plus).FontSize(12).TextColor(icColor)
			ui.Text(c, locales.EnvInlineAddRow()).FontSize(m.FontXS).FontWeight(500).
				TextColor(txtColor).SingleLine()
		})
	}
}

func envVarSpreadsheetRow(
	c *ui.Context,
	st *envEditorState,
	envKey string,
	idx int,
	hasBottomBorder bool,
	autoAppendTail bool,
	onDelete func(),
) {
	m, tk := thV2.Metrics, thV2.Of(c)
	item := &st.vars[idx]
	rowKey := fmt.Sprintf("env-kv-%s-%d-%s", envKey, idx, item.ID)

	row := ui.Row(c.Key(rowKey)).FillWidth().Height(34).AlignItems(ui.Center).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	if hasBottomBorder {
		row.BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border)
	}
	if row.Hovered() {
		row.Background(tk.BgApp)
	} else {
		row.Background(tk.BgBase)
	}

	rowHovered := row.Hovered()
	isEmptyTail := strings.TrimSpace(item.Key) == "" && strings.TrimSpace(item.Value) == ""
	textColor := tk.TextPrimary
	descColor := tk.TextSecondary
	if !item.Enabled {
		textColor = tk.TextMuted
		descColor = tk.TextMuted
	}

	row.Children(func() {
		// 1. 启用复选框列
		ui.Row(c).Width(36).FillHeight().Center().
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			checked := item.Enabled
			cb := ui.CheckboxBase(c.Key(rowKey+"-chk"), &checked).
				Cursor(ui.CursorPointer).Label("Enable variable " + item.Key)
			cb.OnClick(func() {
				item.Enabled = checked
				st.dirty = true
				st.savedMsg = ""
			})
			cb.Children(func() {
				box := ui.Box(c).Size(15, 15).Radius(3).Center()
				if item.Enabled && !isEmptyTail {
					box.Background(tk.Accent).Children(func() {
						ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.OnAccent)
					})
				} else {
					bc := tk.BorderStrong
					if cb.Hovered() {
						bc = tk.Accent
					}
					box.Background(tk.BgSurface).Border(1, bc)
				}
			})
		})

		// 2. Key 单元格（无冗余边框，聚焦时温和底色提示）
		keyCell := ui.Row(c.Key(rowKey+"-key-cell")).Width(210).FillHeight().AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
		if keyCell.FocusWithin() {
			keyCell.Background(tk.AccentSoft.Alpha(0.45))
		}
		keyCell.Children(func() {
			keyIn := ui.TextInputBase(c.Key(rowKey+"-key"), &item.Key).
				Fill().Padding(0, 10).
				Font("monospace").FontSize(m.FontSM).TextColor(textColor).
				Placeholder(locales.EnvKeyPlaceholder()).PlaceholderColor(tk.TextMuted).
				Label(locales.EnvKeyLabel())
			if keyIn.Changed() {
				st.dirty = true
				st.savedMsg = ""
				if autoAppendTail && idx == len(st.vars)-1 && strings.TrimSpace(item.Key) != "" {
					st.vars = append(st.vars, domain.KV{ID: domain.NewID(), Enabled: true})
				}
			}
		})

		// 3. Type 列（Default / Secret 一键切换胶囊）
		ui.Row(c).Width(92).FillHeight().Padding(0, 8).AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border).Children(func() {
			typeLbl := locales.EnvTypeDefault()
			if item.Secret {
				typeLbl = locales.EnvTypeSecret()
			}
			typeBtn := ui.ButtonBase(c.Key(rowKey+"-type")).
				Height(22).PaddingX(7).Gap(4).Radius(m.RadiusFull).
				AlignItems(ui.Center).Justify(ui.Center).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(typeLbl).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if item.Secret {
				typeBtn.Background(tk.Warning.Alpha(0.14)).Border(m.Border, tk.Warning.Alpha(0.35))
			} else {
				bg := tk.BgSubtle
				if typeBtn.Hovered() {
					bg = tk.BgHover
				}
				typeBtn.Background(bg)
			}
			if typeBtn.Clicked() {
				item.Secret = !item.Secret
				st.dirty = true
				st.savedMsg = ""
			}
			typeBtn.Children(func() {
				if item.Secret {
					ui.Icon(c, icons.Lock).FontSize(10).TextColor(tk.Warning)
					ui.Text(c, typeLbl).FontSize(11).FontWeight(500).TextColor(tk.Warning).SingleLine()
				} else {
					ui.Text(c, typeLbl).FontSize(11).FontWeight(500).TextColor(tk.TextSecondary).SingleLine()
				}
			})
		})

		// 4. Value 单元格（等宽字体，Secret 类型支持密码掩码 + 眼睛显隐按钮）
		valCell := ui.Row(c.Key(rowKey+"-val-cell")).Grow(1).FillHeight().AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
		if valCell.FocusWithin() {
			valCell.Background(tk.AccentSoft.Alpha(0.45))
		}
		valCell.Children(func() {
			valIn := ui.TextInputBase(c.Key(rowKey+"-val"), &item.Value).
				Grow(1).FillHeight().Padding(0, 10).
				Font("monospace").FontSize(m.FontSM).TextColor(textColor).
				Placeholder(locales.EnvValPlaceholder()).PlaceholderColor(tk.TextMuted).
				Label(locales.EnvValLabel())
			revealed := st.revealed[item.ID]
			if item.Secret && !revealed {
				valIn.Password()
			}
			if valIn.Changed() {
				st.dirty = true
				st.savedMsg = ""
			}

			if item.Secret {
				eyeTip := locales.EnvToggleSecretShow()
				if revealed {
					eyeTip = locales.EnvToggleSecretHide()
				}
				eyeBtn := v2.IconButton(c, v2.IconButtonOpts{
					Key:          rowKey + "-reveal",
					Label:        eyeTip,
					Tooltip:      eyeTip,
					Icon:         icons.Eye,
					Size:         22,
					IconSize:     12,
					Radius:       m.RadiusSM,
					Color:        tk.TextMuted,
					ActiveAccent: revealed,
				})
				if eyeBtn.Clicked() {
					st.revealed[item.ID] = !revealed
				}
				ui.Box(c).Width(6)
			}
		})

		// 5. Description 单元格
		descCell := ui.Row(c.Key(rowKey+"-desc-cell")).Width(210).FillHeight().AlignItems(ui.Center).
			BorderWidth(0, m.Border, 0, 0).BorderColor(tk.Border)
		if descCell.FocusWithin() {
			descCell.Background(tk.AccentSoft.Alpha(0.45))
		}
		descCell.Children(func() {
			descIn := ui.TextInputBase(c.Key(rowKey+"-desc"), &item.Desc).
				Fill().Padding(0, 10).
				FontSize(m.FontSM).TextColor(descColor).
				Placeholder(locales.EnvDescPlaceholder()).PlaceholderColor(tk.TextMuted).
				Label(locales.EnvDescLabel())
			if descIn.Changed() {
				st.dirty = true
				st.savedMsg = ""
			}
		})

		// 6. 删除行图标按钮（行悬停高亮，Hover 自身转危险红提示）
		ui.Row(c).Width(36).FillHeight().Center().Children(func() {
			delBtn := ui.ButtonBase(c.Key(rowKey+"-del")).
				Size(24, 24).Radius(m.RadiusSM).Center().
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.DeleteRowLabel()).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			icColor := tk.TextMuted
			switch {
			case delBtn.Pressed():
				delBtn.Background(tk.DangerSoft)
				icColor = tk.Danger
			case delBtn.Hovered():
				delBtn.Background(tk.DangerSoft)
				icColor = tk.Danger
			case !rowHovered:
				icColor = tk.TextDisabled
			}
			if delBtn.Clicked() {
				if onDelete != nil {
					onDelete()
				}
			}
			delBtn.Children(func() {
				ui.Icon(c, icons.X).FontSize(12).TextColor(icColor)
			})
		})
	})
}

func envFilterSegmented(c *ui.Context, keyPrefix string, st *envEditorState, total, enabled, secret int) {
	m, tk := thV2.Metrics, thV2.Of(c)
	items := []struct {
		filter envVarFilter
		label  string
	}{
		{filterAll, fmt.Sprintf("%s %d", locales.EnvFilterAll(), total)},
		{filterEnabled, fmt.Sprintf("%s %d", locales.EnvFilterEnabled(), enabled)},
		{filterSecret, fmt.Sprintf("%s %d", locales.EnvFilterSecret(), secret)},
	}
	ui.Row(c.Key(keyPrefix)).Height(26).Padding(2).Gap(2).
		AlignItems(ui.Center).Radius(m.RadiusMD).Background(tk.BgSubtle).Children(func() {
		for _, it := range items {
			item := it
			on := st.filter == item.filter
			seg := ui.ButtonBase(c.Key(fmt.Sprintf("%s-%d", keyPrefix, item.filter))).
				Height(22).PaddingX(8).Radius(m.RadiusSM).Center().
				Cursor(ui.CursorPointer).Role(ui.RoleTab).Label(item.label).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case on:
				seg.Background(tk.BgSurface).Border(m.Border, tk.Border).
					Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.05))
			case seg.Pressed():
				seg.Background(tk.BgSelected).Border(m.Border, ui.Transparent)
			case seg.Hovered():
				seg.Background(tk.BgHover).Border(m.Border, ui.Transparent)
			default:
				seg.Background(ui.Transparent).Border(m.Border, ui.Transparent)
			}
			if seg.Clicked() {
				st.filter = item.filter
			}
			seg.Children(func() {
				fg := tk.TextSecondary
				if on {
					fg = tk.TextPrimary
				}
				ui.Text(c, item.label).FontSize(11).FontWeight(500).TextColor(fg).SingleLine()
			})
		}
	})
}

func envStatusPill(c *ui.Context, label string, bg, border, fg ui.Color, withDot bool, ic *ui.SVG) {
	m := thV2.Metrics
	ui.Row(c).Height(20).PaddingX(7).Gap(4).AlignItems(ui.Center).
		Radius(m.RadiusFull).Background(bg).Border(m.Border, border).Children(func() {
		if withDot {
			ui.Box(c).Size(6, 6).Radius(3).Background(fg)
		} else if ic != nil {
			ui.Icon(c, ic).FontSize(11).TextColor(fg)
		}
		ui.Text(c, label).FontSize(11).FontWeight(500).TextColor(fg).SingleLine()
	})
}

func envPrimaryBtn(c *ui.Context, key string, ic *ui.SVG, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(12).Gap(5).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).
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
		if ic != nil {
			ui.Icon(c, ic).FontSize(12).TextColor(tk.OnAccent)
		}
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(500).TextColor(tk.OnAccent).SingleLine()
	})
	return b
}

func envSoftAccentBtn(c *ui.Context, key string, ic *ui.SVG, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(28).PaddingX(10).Gap(5).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.Accent.Alpha(0.18)).Border(m.Border, tk.Accent.Alpha(0.42))
	case b.Hovered():
		b.Background(tk.Accent.Alpha(0.13)).Border(m.Border, tk.Accent.Alpha(0.38))
	default:
		b.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.28))
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(12).TextColor(tk.Accent)
		}
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(500).TextColor(tk.Accent).SingleLine()
	})
	return b
}

func envSecondaryBtn(c *ui.Context, key string, ic *ui.SVG, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(26).PaddingX(10).Gap(5).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.BgSelected).Border(m.Border, tk.BorderStrong)
	case b.Hovered():
		b.Background(tk.BgHover).Border(m.Border, tk.BorderStrong)
	default:
		b.Background(tk.BgSurface).Border(m.Border, tk.BorderPopover)
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(12).TextColor(tk.TextSecondary)
		}
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()
	})
	return b
}

func envGhostBtn(c *ui.Context, key string, ic *ui.SVG, label string, active bool) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).Height(26).PaddingX(8).Gap(5).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Center).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	fg := tk.TextSecondary
	switch {
	case active:
		b.Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.32))
		fg = tk.Accent
	case b.Pressed():
		b.Background(tk.BgSelected)
		fg = tk.TextPrimary
	case b.Hovered():
		b.Background(tk.BgHover)
		fg = tk.TextPrimary
	default:
		b.Background(ui.Transparent)
	}
	b.Children(func() {
		if ic != nil {
			ui.Icon(c, ic).FontSize(12).TextColor(fg)
		}
		ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
	})
	return b
}

func countEnvStats(vars []domain.KV) (total, enabled, secret int) {
	for _, v := range vars {
		if strings.TrimSpace(v.Key) == "" {
			continue
		}
		total++
		if v.Enabled {
			enabled++
		}
		if v.Secret {
			secret++
		}
	}
	return total, enabled, secret
}

func nextClonedEnvName(existing []domain.Environment, baseName string) string {
	used := make(map[string]bool, len(existing))
	for _, e := range existing {
		used[strings.ToLower(strings.TrimSpace(e.Name))] = true
	}
	cand := baseName + " Copy"
	if !used[strings.ToLower(cand)] {
		return cand
	}
	for i := 2; ; i++ {
		c := fmt.Sprintf("%s Copy %d", baseName, i)
		if !used[strings.ToLower(c)] {
			return c
		}
	}
}

func formatBulkEnvVars(vars []domain.KV) string {
	var lines []string
	for _, v := range vars {
		k := strings.TrimSpace(v.Key)
		if k == "" && strings.TrimSpace(v.Value) == "" {
			continue
		}
		line := k + ": " + v.Value
		if !v.Enabled {
			line = "// " + line
		}
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}

func parseBulkEnvVars(text string, existing []domain.KV) []domain.KV {
	// 保留已有同名变量的 ID、Desc 和 Secret 属性
	byKey := make(map[string]domain.KV, len(existing))
	for _, v := range existing {
		if k := strings.TrimSpace(v.Key); k != "" {
			byKey[k] = v
		}
	}
	var out []domain.KV
	for _, rawLine := range strings.Split(text, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" {
			continue
		}
		enabled := true
		if strings.HasPrefix(line, "//") {
			enabled = false
			line = strings.TrimSpace(strings.TrimPrefix(line, "//"))
		}
		k, val, ok := strings.Cut(line, ":")
		if !ok {
			k, val = line, ""
		}
		k = strings.TrimSpace(k)
		val = strings.TrimSpace(val)
		if k == "" {
			continue
		}
		kv := domain.KV{ID: domain.NewID(), Key: k, Value: val, Enabled: enabled}
		if prev, exists := byKey[k]; exists {
			kv.ID = prev.ID
			kv.Desc = prev.Desc
			kv.Secret = prev.Secret
		}
		out = append(out, kv)
	}
	if len(out) == 0 {
		out = []domain.KV{{ID: domain.NewID(), Enabled: true}}
	}
	return out
}

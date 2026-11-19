// Package environment 实现真实的环境变量管理。
package environment

import (
	"fmt"
	"strings"

	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/logger"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制真实的环境变量管理页面。
func View(c *ui.Context, db *store.Store, n nav.Navigator) {
	ui.Scroll(c).Grow(1).Children(func() {
		content(c, db, n)
	})
}

func content(c *ui.Context, db *store.Store, n nav.Navigator) {
	m := theme.Metrics
	tk := theme.Of(c)
	v1.PageHeader(c, locales.EnvironmentTitle(), locales.EnvironmentSubtitle())

	if db == nil {
		v1.EmptyState(c, "存储未就绪")
		return
	}

	snap := db.Snapshot()
	ws := snap.ActiveWorkspace()
	if ws == nil {
		v1.EmptyState(c, "未选择工作区")
		return
	}

	envs := ws.Environments
	selectedEnvID := ws.SelectedEnvironmentID
	if selectedEnvID == "" && len(envs) > 0 {
		selectedEnvID = envs[0].ID
	}

	root := ui.Column(c).Grow(1)

	// 维护当前正在查看/编辑的环境 ID
	activeEnvID := ui.Local(root, "selected-id", func() domain.ID {
		return selectedEnvID
	})

	// 新建环境弹层状态
	newEnvOpen := ui.Local(root, "new-env-open", func() bool { return false })
	newEnvName := ui.Local(root, "new-env-name", func() string { return "" })
	newEnvErr := ui.Local(root, "new-env-err", func() string { return "" })

	root.Children(func() {
		// 顶部操作栏：新建环境按钮 + 环境切换指示器
		ui.Row(c).Padding(0, m.PagePaddingX, m.Space3, m.PagePaddingX).
			Gap(m.Space2).AlignItems(ui.Center).Children(func() {
			newBtn := ui.PrimaryButton(c, locales.NewEnvironment())
			if newBtn.Clicked() {
				*newEnvOpen = !*newEnvOpen
				*newEnvName = ""
				*newEnvErr = ""
			}

			// 新建环境弹出卡片，锚定在 newBtn 上
			ui.PopoverBase(c, newBtn, newEnvOpen, func(panel ui.Element) {
				panel.Width(320).Padding(m.Space3).Gap(m.Space2)
				theme.PopoverPanel(c, panel)
				panel.Children(func() {
					ui.Text(c, "新建环境").FontSize(m.FontMD).Bold().TextColor(tk.TextPrimary)
					ui.TextInputBase(c, newEnvName).Placeholder("环境名称 (例如: 测试环境 / 生产环境)").
						Padding(m.Space2).Radius(m.RadiusMD).Background(tk.BgElevated).
						Border(m.Border, tk.Border)

					if *newEnvErr != "" {
						ui.Text(c, *newEnvErr).FontSize(m.FontXS).TextColor(tk.Danger)
					}

					ui.Row(c).AlignItems(ui.Center).Children(func() {
						ui.Spacer(c)
						cancelBtn := ui.Button(c, "取消")
						if cancelBtn.Clicked() {
							*newEnvOpen = false
						}
						confirmBtn := ui.PrimaryButton(c, "确定创建")
						if confirmBtn.Clicked() {
							trimmed := strings.TrimSpace(*newEnvName)
							if trimmed == "" {
								*newEnvErr = "环境名称不能为空"
							} else {
								created, err := db.CreateEnvironment(ws.ID, trimmed)
								if err != nil {
									*newEnvErr = err.Error()
								} else {
									*activeEnvID = created.ID
									*newEnvOpen = false
									c.Invalidate()
								}
							}
						}
					})
				})
			})

			ui.Spacer(c)

			if len(envs) > 0 {
				ui.Text(c, "切换激活环境:").FontSize(m.FontXS).TextColor(tk.TextMuted)
				for _, e := range envs {
					e := e
					isCurrentActive := e.ID == ws.SelectedEnvironmentID
					badge := ui.ButtonBase(c.Key("switch-active-"+string(e.ID))).
						Padding(m.Space1, m.Space2).Radius(m.RadiusSM)
					if isCurrentActive {
						badge.Background(tk.AccentSoft)
					} else if badge.Hovered() {
						badge.Background(tk.HoverSurface)
					}
					if badge.Clicked() {
						_ = db.SelectEnvironment(ws.ID, e.ID)
						*activeEnvID = e.ID
					}
					badge.Children(func() {
						txtColor := tk.TextSecondary
						if isCurrentActive {
							txtColor = tk.Accent
						}
						prefix := ""
						if isCurrentActive {
							prefix = "✓ "
						}
						ui.Text(c, prefix+e.Name).FontSize(m.FontXS).TextColor(txtColor)
					})
				}
			}
		})

		if len(envs) == 0 {
			v1.EmptyState(c, "当前工作区暂无环境，点击上方按钮新建一个环境")
			return
		}

		// 找到当前选中的环境数据
		var currentEnv *domain.Environment
		for i := range envs {
			if envs[i].ID == *activeEnvID {
				currentEnv = &envs[i]
				break
			}
		}
		if currentEnv == nil && len(envs) > 0 {
			currentEnv = &envs[0]
			*activeEnvID = currentEnv.ID
		}

		// 环境导航 Tab 列表（水平并列选择查看哪个环境的变量）
		ui.Row(c).Padding(0, m.PagePaddingX, m.Space2, m.PagePaddingX).
			Gap(m.Space1).AlignItems(ui.Center).Children(func() {
			for _, e := range envs {
				e := e
				isSelected := e.ID == *activeEnvID
				tabBtn := ui.ButtonBase(c.Key("env-tab-"+string(e.ID))).
					Padding(m.Space2, m.Space3).Radius(m.RadiusMD)
				if isSelected {
					tabBtn.Background(tk.BgSurface).Border(m.Border, tk.Border)
				} else if tabBtn.Hovered() {
					tabBtn.Background(tk.HoverSurface)
				}
				if tabBtn.Clicked() {
					*activeEnvID = e.ID
				}
				tabBtn.Children(func() {
					titleColor := tk.TextSecondary
					if isSelected {
						titleColor = tk.TextPrimary
					}
					ui.Text(c, e.Name).FontSize(m.FontSM).Bold().TextColor(titleColor)
					countStr := fmt.Sprintf("(%d)", len(e.Vars))
					ui.Text(c, countStr).FontSize(m.FontXS).TextColor(tk.TextMuted)
				})
			}
		})

		if currentEnv == nil {
			return
		}

		// 变量编辑表格卡片
		renderEnvironmentEditor(c, db, ws.ID, currentEnv)
	})
}

func renderEnvironmentEditor(c *ui.Context, db *store.Store, wsID domain.ID, env *domain.Environment) {
	m := theme.Metrics
	tk := theme.Of(c)

	card := ui.Column(c.Key("env-editor-"+string(env.ID))).
		Margin(0, m.PagePaddingX, m.Space4, m.PagePaddingX).
		Background(tk.BgSurface).Border(m.Border, tk.Border).
		Radius(m.RadiusMD)

	// 本地维护当前变量列表的草稿
	draftVars := ui.Local(card, "vars-draft", func() []domain.KV {
		if len(env.Vars) == 0 {
			return []domain.KV{{ID: domain.NewID(), Enabled: true}}
		}
		out := make([]domain.KV, len(env.Vars))
		copy(out, env.Vars)
		return out
	})

	card.Children(func() {
		// 变量表头部说明与保存按钮
		ui.Row(c).Padding(m.Space3, m.Space3).AlignItems(ui.Center).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
			Children(func() {
				ui.Column(c).Gap(1).Children(func() {
					ui.Text(c, env.Name).FontSize(m.FontMD).Bold().TextColor(tk.TextPrimary)
					ui.Text(c, "配置此环境下的变量，在请求 URL、Headers、Body 中使用 {{变量名}} 即可引用").
						FontSize(m.FontXS).TextColor(tk.TextMuted)
				})

				ui.Spacer(c)

				ui.Row(c).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
					// 添加行按钮
					addVarBtn := ui.Button(c, "+ 添加变量")
					if addVarBtn.Clicked() {
						*draftVars = append(*draftVars, domain.KV{
							ID:      domain.NewID(),
							Enabled: true,
						})
					}

					// 保存按钮
					saveBtn := ui.PrimaryButton(c, "保存变量")
					if saveBtn.Clicked() {
						var cleaned []domain.KV
						for _, v := range *draftVars {
							if strings.TrimSpace(v.Key) != "" {
								cleaned = append(cleaned, v)
							}
						}
						envCopy := *env
						envCopy.Vars = cleaned
						if err := db.SaveEnvironment(wsID, envCopy); err != nil {
							logger.Error("保存环境失败: %v", err)
						} else {
							c.Invalidate()
						}
					}

					// 删除环境（仅非默认环境可删除）
					delBtn := ui.Button(c, "删除环境")
					if delBtn.Clicked() {
						if err := db.DeleteEnvironment(wsID, env.ID); err != nil {
							logger.Warn("删除环境失败: %v", err)
						} else {
							c.Invalidate()
						}
					}
				})
			})

		// 表头
		ui.Row(c).Gap(m.Space2).Padding(m.Space2, m.Space3).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
			Background(tk.BgElevated).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "启用").FontSize(m.FontXS).TextColor(tk.TextMuted).Width(40)
			ui.Text(c, locales.VariableName()).FontSize(m.FontXS).TextColor(tk.TextMuted).Width(180)
			ui.Text(c, "变量值 (Value)").FontSize(m.FontXS).TextColor(tk.TextMuted).Grow(1)
			ui.Text(c, "描述 / 备注").FontSize(m.FontXS).TextColor(tk.TextMuted).Width(180)
			ui.Text(c, "操作").FontSize(m.FontXS).TextColor(tk.TextMuted).Width(40)
		})

		// 表体各行
		if len(*draftVars) == 0 {
			ui.Box(c).Padding(m.Space4).Center().Children(func() {
				ui.Text(c, "暂无变量，点击右上角「+ 添加变量」添加").FontSize(m.FontSM).TextColor(tk.TextMuted)
			})
		} else {
			for i := range *draftVars {
				idx := i
				kvRow(c, draftVars, idx)
			}
		}
	})
}

func kvRow(c *ui.Context, list *[]domain.KV, idx int) {
	m := theme.Metrics
	tk := theme.Of(c)
	item := &(*list)[idx]

	ui.Row(c.Key(fmt.Sprintf("kv-row-%d-%s", idx, item.ID))).
		Gap(m.Space2).Padding(m.Space2, m.Space3).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		AlignItems(ui.Center).Children(func() {
		// 启用 Checkbox
		chk := ui.Checkbox(c, &item.Enabled, "")
		chk.Width(40)

		// 变量名 Key
		ui.TextInputBase(c, &item.Key).Placeholder("例如: baseUrl").
			Width(180).FontSize(m.FontSM).Padding(m.Space1).
			Background(tk.BgElevated).Radius(m.RadiusSM)

		// 变量值 Value
		ui.TextInputBase(c, &item.Value).Placeholder("例如: https://api.example.com").
			Grow(1).FontSize(m.FontSM).Padding(m.Space1).
			Background(tk.BgElevated).Radius(m.RadiusSM)

		// 备注 Desc
		ui.TextInputBase(c, &item.Desc).Placeholder("选填说明").
			Width(180).FontSize(m.FontSM).Padding(m.Space1).
			Background(tk.BgElevated).Radius(m.RadiusSM)

		// 删除行按钮
		delRowBtn := ui.ButtonBase(c.Key(fmt.Sprintf("del-row-%d", idx))).
			Width(24).Height(24).Radius(m.RadiusSM).Center()
		if delRowBtn.Hovered() {
			delRowBtn.Background(tk.HoverSurface)
		}
		if delRowBtn.Clicked() {
			*list = append((*list)[:idx], (*list)[idx+1:]...)
		}
		delRowBtn.Children(func() {
			ui.Icon(c, icons.Get("x")).FontSize(m.IconXS).TextColor(tk.TextMuted)
		})
	})
}

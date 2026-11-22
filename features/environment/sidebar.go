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

// SidebarView 在 WorkbenchLayout 左侧面板内绘制紧凑的环境变量管理侧栏。
// 点击环境项通过 openEnv 在右侧常驻多 Tab 工作区打开变量编辑 Tab；
// 点击右侧勾选按钮可直接切换当前激活环境。
func SidebarView(c *ui.Context, db *store.Store, wsID domain.ID, openEnv func(envID domain.ID)) {
	m, tk := thV2.Metrics, thV2.Of(c)

	col := ui.Column(c).Fill().Background(tk.BgApp)
	addOpen := ui.Local(col, "env-sb-add-open", func() bool { return false })
	newName := ui.Local(col, "env-sb-new-name", func() string { return "" })
	newErr := ui.Local(col, "env-sb-new-err", func() string { return "" })
	selectedEnvID := ui.Local(col, "env-sb-selected-id", func() domain.ID { return "" })

	var envs []domain.Environment
	var activeEnvID domain.ID
	if db != nil {
		snap := db.Snapshot()
		if ws := snap.Workspace(wsID); ws != nil {
			envs = ws.Environments
			activeEnvID = ws.SelectedEnvironmentID
		}
	}

	if *selectedEnvID != "" {
		found := false
		for _, e := range envs {
			if e.ID == *selectedEnvID {
				found = true
				break
			}
		}
		if !found {
			*selectedEnvID = ""
		}
	}

	col.Children(func() {
		// 顶部标题行：ENVIRONMENTS + 新建按钮
		ui.Row(c).FillWidth().Height(32).Padding(0, m.Space3).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "ENVIRONMENTS").FontSize(11).FontWeight(600).
				TextColor(tk.TextMuted).SingleLine().Grow(1)

			addBtn := ui.ButtonBase(c.Key("wb-env-add")).
				Size(22, 22).Radius(m.RadiusSM).Center().
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("New environment")
			if addBtn.Hovered() {
				addBtn.Background(tk.BgHover)
			}
			if addBtn.Clicked() {
				*addOpen = !*addOpen
				*newName = ""
				*newErr = ""
			}
			addBtn.Children(func() {
				ui.Icon(c, icons.Plus).FontSize(m.IconXS).TextColor(tk.TextSecondary)
			})

			v2.Popover(c, addBtn, addOpen, v2.PopoverOpts{Width: 240}, func(panel ui.Element) {
				ui.Column(c).Padding(m.Space3).Gap(m.Space2).Children(func() {
					ui.Text(c, "New Environment").FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextPrimary)
					in := ui.TextInputBase(c.Key("wb-env-name-input"), newName).
						FillWidth().Height(28).Padding(0, m.Space2).Radius(m.RadiusSM).
						Background(tk.BgSurface).Border(m.Border, tk.BorderPopover).
						FontSize(m.FontXS).TextColor(tk.TextPrimary).
						Placeholder("e.g. Local / Staging / Prod").
						Label("Environment name")
					if in.Focused() {
						in.Border(m.Border, tk.Focus)
					}
					if *newErr != "" {
						ui.Text(c, *newErr).FontSize(m.FontMicro).TextColor(tk.Danger)
					}

					submit := func() {
						name := strings.TrimSpace(*newName)
						if name == "" {
							*newErr = "Please enter a name."
							return
						}
						if db == nil {
							return
						}
						env, err := db.CreateEnvironment(wsID, name)
						if err != nil {
							*newErr = err.Error()
							return
						}
						_ = db.SelectEnvironment(wsID, env.ID)
						if openEnv != nil {
							openEnv(env.ID)
						}
						*selectedEnvID = env.ID
						*addOpen = false
						*newName = ""
						*newErr = ""
					}
					if in.Submitted() {
						submit()
					}
					ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space15).Children(func() {
						cancelBtn := ui.ButtonBase(c.Key("wb-env-cancel")).
							Height(24).PaddingX(8).Radius(m.RadiusSM).Center().
							Border(m.Border, tk.BorderPopover).Label("Cancel")
						if cancelBtn.Hovered() {
							cancelBtn.Background(tk.BgHover)
						}
						if cancelBtn.Clicked() {
							*addOpen = false
						}
						cancelBtn.Children(func() {
							ui.Text(c, "Cancel").FontSize(m.FontXS).TextColor(tk.TextSecondary)
						})

						okBtn := ui.ButtonBase(c.Key("wb-env-submit")).
							Height(24).PaddingX(10).Radius(m.RadiusSM).Center().
							Background(tk.Accent).Label("Create")
						if okBtn.Hovered() {
							okBtn.Background(tk.AccentHover)
						}
						if okBtn.Clicked() {
							submit()
						}
						okBtn.Children(func() {
							ui.Text(c, "Create").FontSize(m.FontXS).FontWeight(500).TextColor(tk.OnAccent)
						})
					})
				})
			})
		})

		if len(envs) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space2).Children(func() {
				ui.Icon(c, icons.Globe).FontSize(22).TextColor(tk.TextMuted)
				ui.Text(c, "No environments").FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextSecondary)
				ui.Text(c, "Click + above to create an environment.").FontSize(m.FontXS).TextColor(tk.TextMuted)
			})
			return
		}

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, m.Space1, m.Space3, m.Space1).Gap(2).Children(func() {
				for _, e := range envs {
					env := e
					isActive := env.ID == activeEnvID
					isSelected := env.ID == *selectedEnvID

					line := ui.Row(c.Key("wb-env-line-"+string(env.ID))).
						FillWidth().Height(30).Padding(0, m.Space1).Gap(2).
						Radius(m.RadiusSM).AlignItems(ui.Center)
					if isSelected {
						line.Background(tk.BgSelected)
					} else if line.Hovered() {
						line.Background(tk.BgHover)
					}

					line.Children(func() {
						// 左侧主点击区：在右侧打开该环境的编辑 Tab，并将该项设为选中态
						row := ui.ButtonBase(c.Key("wb-env-row-"+string(env.ID))).
							Grow(1).FillHeight().Padding(0, m.Space15).Gap(m.Space2).
							AlignItems(ui.Center).Justify(ui.Start).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(env.Name)
						if row.Clicked() {
							*selectedEnvID = env.ID
							if openEnv != nil {
								openEnv(env.ID)
							}
						}
						row.Children(func() {
							iconColor := tk.TextSecondary
							if isSelected {
								iconColor = tk.TextPrimary
							}
							ui.Icon(c, icons.Globe).FontSize(m.IconXS).TextColor(iconColor)
							ui.Text(c, env.Name).FontSize(m.FontXS).
								FontWeight(map[bool]int{true: 600, false: 400}[isSelected]).
								TextColor(tk.TextPrimary).SingleLine().Grow(1)
							ui.Text(c, fmt.Sprintf("%d", len(env.Vars))).
								FontSize(10).TextColor(tk.TextMuted).SingleLine()
						})

						// 右侧独立按钮：一键切换激活环境（仅当前生效的环境显示高亮对钩，其余项不显示任何对号）
						actBtn := ui.ButtonBase(c.Key("wb-env-act-"+string(env.ID))).
							Size(22, 22).Radius(m.RadiusSM).Center().
							Cursor(ui.CursorPointer).Role(ui.RoleButton).
							Label("Activate " + env.Name)
						if isActive {
							actBtn.Tooltip("Active environment (click to deactivate)")
						} else {
							actBtn.Tooltip("Set active environment")
						}
						if actBtn.Hovered() {
							actBtn.Background(tk.BgHover)
						}
						if actBtn.Clicked() && db != nil {
							if err := db.SelectEnvironment(wsID, env.ID); err != nil {
								logger.Error("切换环境失败: %v", err)
							}
						}
						actBtn.Children(func() {
							if isActive {
								ui.Icon(c, icons.Check).FontSize(12).TextColor(tk.Accent)
							}
						})
					})
				}
			})
		})
	})
}

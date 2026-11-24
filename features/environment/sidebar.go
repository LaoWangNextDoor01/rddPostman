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
	search := ui.Local(col, "env-sb-search", func() string { return "" })
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

	kw := strings.ToLower(strings.TrimSpace(*search))

	col.Children(func() {
		// 1. 统一 34px 顶部搜索过滤条 + 新建「+」按钮（与集合/小工具侧栏完全齐平）
		ui.Row(c).FillWidth().Height(34).Padding(0, m.Space3).Gap(6).
			AlignItems(ui.Center).Children(func() {
			searchBox := ui.Row(c.Key("env-sb-search-box")).Grow(1).Height(26).
				Padding(0, 8, 0, 6).Gap(6).AlignItems(ui.Center).
				Radius(m.RadiusMD).Background(tk.BgSurface).
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
				ui.Icon(c, icons.ListFilter).FontSize(13).TextColor(tk.TextMuted)
				ui.TextInputBase(c.Key("env-sb-search-input"), search).Grow(1).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).
					Placeholder("Search").PlaceholderColor(tk.TextMuted).
					Label("Filter environments")
				if *search != "" {
					clr := v2.IconButton(c, v2.IconButtonOpts{
						Key:      "env-sb-search-clear",
						Label:    "Clear filter",
						Icon:     icons.X,
						Size:     18,
						IconSize: 11,
						Color:    tk.TextMuted,
					})
					if clr.Clicked() {
						*search = ""
					}
				}
			})

			addBtn := v2.IconButton(c, v2.IconButtonOpts{
				Key:      "wb-env-add",
				Label:    "New environment",
				Tooltip:  "New environment",
				Icon:     icons.Plus,
				Size:     24,
				IconSize: 14,
				Radius:   m.RadiusSM,
			})
			if addBtn.Clicked() {
				*addOpen = !*addOpen
				*newName = ""
				*newErr = ""
			}

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

		var filtered []domain.Environment
		for _, e := range envs {
			if kw == "" || strings.Contains(strings.ToLower(e.Name), kw) {
				filtered = append(filtered, e)
				continue
			}
			for _, kv := range e.Vars {
				if strings.Contains(strings.ToLower(kv.Key), kw) {
					filtered = append(filtered, e)
					break
				}
			}
		}

		if len(filtered) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space1).Children(func() {
				ui.Text(c, "No matching environments").FontSize(m.FontXS).TextColor(tk.TextMuted)
			})
			return
		}

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6, m.Space2, m.Space4, m.Space2).Gap(2).Children(func() {
				for _, e := range filtered {
					env := e
					isActive := env.ID == activeEnvID
					isSelected := env.ID == *selectedEnvID

					line := ui.Row(c.Key("wb-env-line-"+string(env.ID))).
						FillWidth().Height(m.RowHeight).Padding(0, 4, 0, 8).Gap(4).
						Radius(m.RadiusSM).AlignItems(ui.Center).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					switch {
					case isSelected:
						line.Background(tk.BgSelected)
					case line.Hovered():
						line.Background(tk.BgHover)
					}

					line.Children(func() {
						// 左侧主点击区：在右侧打开该环境的编辑 Tab，并将该项设为选中态
						row := ui.ButtonBase(c.Key("wb-env-row-"+string(env.ID))).
							Grow(1).FillHeight().Gap(6).
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
							weight := 400
							if isSelected {
								iconColor = tk.TextPrimary
								weight = 500
							}
							ui.Icon(c, icons.Globe).FontSize(13).TextColor(iconColor)
							ui.Text(c, env.Name).FontSize(m.FontSM).FontWeight(weight).
								TextColor(tk.TextPrimary).SingleLine().Grow(1)
							ui.Text(c, fmt.Sprintf("%d", len(env.Vars))).
								FontSize(10).TextColor(tk.TextMuted).SingleLine()
						})

						// 右侧独立按钮：一键切换激活环境（仅当前生效的环境显示高亮对钩，其余项不显示任何对号）
						tipText := "Set active environment"
						if isActive {
							tipText = "Active environment"
						}
						actBtn := ui.ButtonBase(c.Key("wb-env-act-"+string(env.ID))).
							Size(20, 20).Radius(m.RadiusSM).Center().
							Cursor(ui.CursorPointer).Role(ui.RoleButton).
							Label("Activate " + env.Name).
							Description(tipText).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						switch {
						case actBtn.Pressed():
							actBtn.Background(tk.BgSelected)
						case actBtn.Hovered():
							actBtn.Background(tk.BgSelected)
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
						ui.TooltipBase(c, actBtn, func(tip ui.Element) {
							tip.AttachTo(actBtn, ui.AnchorRight, ui.AnchorLeft).
								MarginStart(8).Padding(5, 8).Radius(m.RadiusMD).
								Background(tk.BgSurface).Border(m.Border, tk.BorderPopover).
								Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06)).
								Shadow(0, 6, 16, 0, ui.RGBA(0, 0, 0, 0.10))
							tip.Children(func() {
								ui.Text(c, tipText).FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()
							})
						})
					})
				}
			})
		})
	})
}

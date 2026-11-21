package workspace

import (
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

// createWorkspaceView 按设计稿 29 / 31（Dark）绘制「Create your workspace」表单页。
// 左侧仍保留 Hub 侧栏，点击 Cancel 返回 Workspaces 列表，点击 Create Workspace 真实写入 Store。
func createWorkspaceView(c *ui.Context, db *store.Store, _ nav.Navigator, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.Scroll(c).Fill().Background(tk.BgBase).Children(func() {
		ui.Column(c).FillWidth().Padding(32, m.Space6, 40, m.Space6).AlignItems(ui.Center).Children(func() {
			ui.Column(c).FillWidth().MaxWidth(520).Gap(m.Space4).Children(func() {
				// 标题
				ui.Text(c, "Create your workspace").
					FontSize(m.FontXL).Bold().TextColor(tk.TextPrimary).SingleLine()

				// 1. Workspace name
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Workspace name").FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					nameIn := ui.TextInputBase(c.Key("ws-create-name"), &st.createName).
						Bind(&st.createNameHandle).
						FillWidth().Height(m.ButtonLarge).PaddingX(m.Space3).
						Radius(m.RadiusSM).Background(tk.BgSurface).
						FontSize(m.FontSM).TextColor(tk.TextPrimary).
						Placeholder("A team name, project or service...").
						PlaceholderColor(tk.TextMuted).
						Label("Workspace name")

					switch {
					case st.createErr != "":
						nameIn.Border(m.Border, tk.Danger)
					case nameIn.Focused():
						nameIn.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
					case nameIn.Hovered():
						nameIn.Border(m.Border, tk.BorderStrong)
					default:
						nameIn.Border(m.Border, tk.BorderPopover)
					}

					if st.createErr != "" {
						ui.Text(c, st.createErr).FontSize(m.FontXS).TextColor(tk.Danger)
					}
					if nameIn.Submitted() {
						submitCreateWorkspace(db, st)
					}
				})

				// 2. Select workspace type
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Select workspace type").FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					ui.Row(c).FillWidth().Gap(m.Space2).AlignItems(ui.Stretch).Children(func() {
						wsTypeCard(c, st, "Internal", "Share APIs with your team.", "")
						wsTypeCard(c, st, "Partner", "Share APIs with your partners.", "Upgrade plan")
						wsTypeCard(c, st, "Public", "Everyone can access your APIs.", "")
					})
				})

				// 3. Who can access
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Who can access").FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()

					accessBtn := ui.ButtonBase(c.Key("ws-create-access")).
						FillWidth().Height(34).PaddingX(m.Space3).
						Radius(m.RadiusSM).Background(tk.BgSurface).
						AlignItems(ui.Center).Justify(ui.SpaceBetween).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Who can access").
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					accessOpen := ui.Local(accessBtn, "open", func() bool { return false })

					switch {
					case *accessOpen:
						accessBtn.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
					case accessBtn.Hovered():
						accessBtn.Border(m.Border, tk.BorderStrong)
					default:
						accessBtn.Border(m.Border, tk.BorderPopover)
					}

					if accessBtn.Clicked() {
						*accessOpen = !*accessOpen
					}

					accessBtn.Children(func() {
						ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
							ui.Icon(c, icons.Users).FontSize(m.IconSM).TextColor(tk.TextSecondary)
							ui.Text(c, st.createAccess).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
						})
						ui.Icon(c, icons.ChevronDown).FontSize(m.IconXS).TextColor(tk.TextMuted)
					})

					v2.Popover(c, accessBtn, accessOpen, v2.PopoverOpts{Width: 320}, func(panel ui.Element) {
						ui.Column(c).FillWidth().Padding(m.Space15).Gap(m.Space05).Children(func() {
							for _, opt := range []string{
								"Everyone in wang's Team",
								"Only invited team members",
							} {
								val := opt
								wsPopoverOption(c, "ws-acc-"+val, val, st.createAccess == val, func() {
									st.createAccess = val
									*accessOpen = false
								})
							}
						})
					})
				})

				// 4. Choose how you'd like to set up this workspace
				ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
					ui.Text(c, "Choose how you'd like to set up this workspace").
						FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()

					ui.Row(c).FillWidth().Gap(m.Space3).AlignItems(ui.Stretch).Children(func() {
						wsSetupBlankCard(c, st)
						wsSetupLocalCard(c, st)
					})
				})

				// 5. Create with Postman AI（UI 占位）
				ui.Column(c).FillWidth().Gap(m.Space1).Children(func() {
					ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
						ui.Icon(c, icons.Sparkles).FontSize(m.IconSM).TextColor(tk.MethodPatch)
						ui.Text(c, "Create with Postman AI").FontSize(m.FontSM).FontWeight(500).
							TextColor(tk.TextPrimary).SingleLine()
					})
					ui.Text(c, "Ask AI to set up collections, specs, and environments for your API.").
						FontSize(m.FontXS).TextColor(tk.TextMuted)

					aiIn := ui.TextAreaBase(c.Key("ws-create-ai"), &st.createAI).
						Margin(m.Space1, 0, 0, 0).FillWidth().Height(58).Padding(m.Space2, m.Space3).
						Radius(m.RadiusSM).Background(tk.BgSurface).
						FontSize(m.FontSM).TextColor(tk.TextPrimary).
						Placeholder("Describe what you want to build (e.g., Payments API).").
						PlaceholderColor(tk.TextMuted).
						Label("Create with Postman AI")
					if aiIn.Focused() {
						aiIn.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
					} else {
						aiIn.Border(m.Border, tk.BorderPopover)
					}
				})

				// 6. 底部按钮行：Cancel + Create Workspace
				ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space2, 0, 0, 0).Children(func() {
					if wsSecondaryButton(c, "ws-create-cancel", "Cancel").Clicked() {
						st.creating = false
						st.createErr = ""
					}
					disabled := strings.TrimSpace(st.createName) == ""
					if wsPrimaryButton(c, "ws-create-submit", "Create Workspace", disabled).Clicked() {
						submitCreateWorkspace(db, st)
					}
				})
			})
		})
	})
}

func submitCreateWorkspace(db *store.Store, st *wsContentState) {
	name := strings.TrimSpace(st.createName)
	if name == "" {
		st.createErr = "Please enter a workspace name."
		return
	}
	if db != nil {
		ws, err := db.CreateWorkspace(name)
		if err != nil {
			st.createErr = err.Error()
			return
		}
		if ws != nil {
			st.types[ws.ID] = st.createType
			_ = db.SetActiveWorkspace(ws.ID)
		}
	}
	st.createErr = ""
	st.creating = false
}

func wsTypeCard(c *ui.Context, st *wsContentState, title, desc, badge string) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createType == title

	card := ui.ButtonBase(c.Key("ws-type-"+title)).
		Grow(1).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).
		AlignItems(ui.Start).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label(title).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	if card.Clicked() {
		st.createType = title
	}

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
				wsRadioSquare(c, selected)
				ui.Text(c, title).FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			ui.Text(c, desc).FontSize(m.FontXS).TextColor(tk.TextMuted)
			if badge != "" {
				ui.Text(c, badge).FontSize(m.FontXS).TextColor(tk.Info).SingleLine()
			}
		})
	})
}

func wsSetupBlankCard(c *ui.Context, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createSetup == "blank"

	card := ui.ButtonBase(c.Key("ws-setup-blank")).
		Grow(1).MinHeight(144).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).
		AlignItems(ui.Start).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label("Blank workspace").
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	if card.Clicked() {
		st.createSetup = "blank"
	}

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
				wsRadioSquare(c, selected)
				ui.Text(c, "Blank workspace").FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			ui.Text(c, "I want to start from scratch.").FontSize(m.FontXS).TextColor(tk.TextMuted)
		})
	})
}

func wsSetupLocalCard(c *ui.Context, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)
	selected := st.createSetup == "local"

	card := ui.ButtonBase(c.Key("ws-setup-local")).
		Grow(1).MinHeight(144).Padding(m.Space3).Radius(m.RadiusSM).
		Background(tk.BgSurface).
		AlignItems(ui.Start).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleRadio).Label("Open a local folder").
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case selected:
		card.Border(m.Border, tk.Focus)
	case card.Hovered():
		card.Border(m.Border, tk.BorderStrong)
	default:
		card.Border(m.Border, tk.BorderPopover)
	}

	if card.Clicked() {
		st.createSetup = "local"
	}

	card.Children(func() {
		ui.Column(c).FillWidth().Gap(m.Space15).Children(func() {
			ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
				wsRadioSquare(c, selected)
				ui.Text(c, "Open a local folder").FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			ui.Column(c).Gap(m.Space1).Padding(m.Space1, 0, 0, 0).Children(func() {
				for _, folder := range []string{"fe-topic", "wc-miniapp-ms", "mstand", "fe-crm"} {
					ui.Row(c).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
						ui.Icon(c, icons.Folder).FontSize(m.IconXS).TextColor(tk.TextMuted)
						ui.Text(c, folder).FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()
					})
				}
			})
			ui.Row(c).Margin(m.Space1, 0, 0, 0).Children(func() {
				ui.Box(c).Height(24).PaddingX(m.Space2).Center().
					Radius(m.RadiusSM).Border(m.Border, tk.BorderPopover).Background(tk.BgSurface).
					Children(func() {
						ui.Text(c, "Open New Folder").FontSize(m.FontXS).TextColor(tk.TextPrimary).SingleLine()
					})
			})
		})
	})
}

func wsRadioSquare(c *ui.Context, selected bool) {
	m, tk := thV2.Metrics, thV2.Of(c)
	box := ui.Box(c).Size(14, 14).Radius(3).Shrink(0)
	if selected {
		box.Background(tk.Focus).Border(m.Border, tk.Focus)
	} else {
		box.Background(tk.BgSurface).Border(m.Border, tk.BorderPopover)
	}
}

package collection

import (
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

type moveDestination struct {
	collID   domain.ID
	parentID domain.ID
	name     string
	depth    int
	isColl   bool
	current  bool
}

// collectMoveDestinations 收集可供 pending 节点移动的目标容器列表（集合与目录），
// 自动排除节点自身及其所有子孙目录，并标记当前所在父容器。
func collectMoveDestinations(collections []*domain.Collection, pending *treeNode) []moveDestination {
	if pending == nil {
		return nil
	}
	var out []moveDestination
	for _, coll := range collections {
		isCurrCollRoot := string(coll.ID) == pending.collID && pending.parentID == ""
		out = append(out, moveDestination{
			collID:   coll.ID,
			parentID: "",
			name:     coll.Name,
			depth:    0,
			isColl:   true,
			current:  isCurrCollRoot,
		})
		var walk func(items []*domain.Node, depth int)
		walk = func(items []*domain.Node, depth int) {
			for _, n := range items {
				if n.Kind != domain.KindFolder {
					continue
				}
				// 禁止将目录移入自身或任意子孙目录。
				if pending.kind == treeFolder && string(n.ID) == pending.id {
					continue
				}
				isCurr := string(coll.ID) == pending.collID && string(n.ID) == pending.parentID
				out = append(out, moveDestination{
					collID:   coll.ID,
					parentID: n.ID,
					name:     n.Name,
					depth:    depth,
					isColl:   false,
					current:  isCurr,
				})
				walk(n.Children, depth+1)
			}
		}
		walk(coll.Items, 1)
	}
	return out
}

// moveNodeDialog 渲染「移动到...」模态对话框，支持将目录或请求移动到同一工作区内的任意集合或目录下。
func moveNodeDialog(c *ui.Context, props pageProps) {
	state := props.state
	if state == nil || state.movePending == nil {
		return
	}
	m, tk := thV2.Metrics, thV2.Of(c)
	target := state.movePending
	dests := collectMoveDestinations(props.collections, target)

	ui.DialogBase(c, &state.moveDialogOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(440).Padding(m.Space5).Gap(m.Space3).
			Background(tk.BgSurface).
			Border(m.Border, tk.BorderPopover).
			Radius(m.RadiusLG).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.18))

		panel.Children(func() {
			// 标题行
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(10).Children(func() {
				ui.Box(c).Size(30, 30).Radius(m.RadiusMD).
					Background(tk.AccentSoft).Border(m.Border, tk.Accent.Alpha(0.28)).
					Center().Children(func() {
					ui.Icon(c, icons.ArrowRight).FontSize(15).TextColor(tk.Accent)
				})
				ui.Text(c, locales.MoveNodeTitle()).FontSize(m.FontMD).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine().Grow(1)
			})

			ui.Text(c, locales.MoveNodeDesc(target.name)).
				FontSize(m.FontSM).FixedLineHeight(20).TextColor(tk.TextSecondary).FillWidth()

			// 可滚动目标容器列表
			hasOtherTarget := false
			for _, d := range dests {
				if !d.current {
					hasOtherTarget = true
					break
				}
			}

			ui.Box(c).FillWidth().MaxHeight(260).Radius(m.RadiusMD).
				Background(tk.BgApp).Border(m.Border, tk.Border).Children(func() {
				if !hasOtherTarget {
					ui.Box(c).FillWidth().Padding(m.Space5).Center().Children(func() {
						ui.Text(c, locales.MoveNodeNoTargets()).FontSize(m.FontSM).TextColor(tk.TextMuted)
					})
					return
				}
				ui.Scroll(c).FillWidth().MaxHeight(258).Children(func() {
					ui.Column(c).FillWidth().Padding(4).Gap(2).Children(func() {
						for _, d := range dests {
							dest := d
							selected := state.moveTargetCollID == dest.collID &&
								state.moveTargetParentID == dest.parentID &&
								state.moveTargetCollID != ""
							rowKey := "move-dest-" + string(dest.collID) + "-" + string(dest.parentID)
							row := ui.ButtonBase(c.Key(rowKey)).
								FillWidth().Height(28).
								Padding(0, 10).PaddingStart(10 + 16*float32(dest.depth)).
								Gap(6).Radius(m.RadiusSM).
								AlignItems(ui.Center).Justify(ui.Start).
								Role(ui.RoleOption).Label(dest.name).
								Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

							if dest.current {
								row.Disabled(true)
							} else {
								row.Cursor(ui.CursorPointer)
								switch {
								case selected:
									row.Background(tk.AccentSoft)
								case row.Pressed():
									row.Background(tk.BgSelected)
								case row.Hovered():
									row.Background(tk.BgHover)
								}
								if row.Clicked() {
									state.moveTargetCollID = dest.collID
									state.moveTargetParentID = dest.parentID
								}
							}

							row.Children(func() {
								ic := icons.Folder
								if dest.isColl {
									ic = icons.Library
								}
								icColor := tk.TextSecondary
								if selected {
									icColor = tk.Accent
								} else if dest.current {
									icColor = tk.TextMuted
								}
								ui.Icon(c, ic).FontSize(13).TextColor(icColor)

								nameColor := tk.TextPrimary
								weight := 400
								if selected {
									weight = 500
								} else if dest.current {
									nameColor = tk.TextMuted
								}
								ui.Text(c, dest.name).FontSize(m.FontSM).FontWeight(weight).
									TextColor(nameColor).SingleLine().Grow(1)

								if dest.current {
									ui.Text(c, locales.MoveNodeCurrent()).FontSize(m.FontXS).
										TextColor(tk.TextMuted).SingleLine()
								} else if selected {
									ui.Icon(c, icons.Check).FontSize(13).TextColor(tk.Accent)
								}
							})
						}
					})
				})
			})

			// 底部操作按钮
			canConfirm := state.moveTargetCollID != "" &&
				!(string(state.moveTargetCollID) == target.collID && string(state.moveTargetParentID) == target.parentID)

			ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space1, 0, 0, 0).Children(func() {
				cancelBtn := ui.ButtonBase(c.Key("move-node-cancel")).
					Height(m.ButtonLarge).PaddingX(m.Space4).Center().
					Radius(m.RadiusSM).Cursor(ui.CursorPointer).
					Role(ui.RoleButton).Label(locales.Cancel()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case cancelBtn.Pressed():
					cancelBtn.Background(tk.BgSelected).Border(m.Border, tk.BorderStrong)
				case cancelBtn.Hovered():
					cancelBtn.Background(tk.BgHover).Border(m.Border, tk.BorderStrong)
				default:
					cancelBtn.Background(tk.BgSurface).Border(m.Border, tk.BorderPopover)
				}
				if cancelBtn.Clicked() {
					state.moveDialogOpen = false
					state.movePending = nil
				}
				cancelBtn.Children(func() {
					ui.Text(c, locales.Cancel()).FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()
				})

				confirmBtn := ui.ButtonBase(c.Key("move-node-confirm")).
					Height(m.ButtonLarge).PaddingX(m.Space4).Center().
					Radius(m.RadiusSM).
					Role(ui.RoleButton).Label(locales.MoveNodeConfirm()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				if !canConfirm {
					confirmBtn.Disabled(true).Background(tk.Accent.Alpha(0.45))
				} else {
					confirmBtn.Cursor(ui.CursorPointer)
					switch {
					case confirmBtn.Pressed():
						confirmBtn.Background(tk.AccentPressed)
					case confirmBtn.Hovered():
						confirmBtn.Background(tk.AccentHover)
					default:
						confirmBtn.Background(tk.Accent)
					}
					if confirmBtn.Clicked() {
						srcCollID := domain.ID(target.collID)
						nodeID := domain.ID(target.id)
						dstCollID := state.moveTargetCollID
						dstParentID := state.moveTargetParentID
						state.moveDialogOpen = false
						state.movePending = nil
						if props.actions.moveNode != nil {
							props.actions.moveNode(srcCollID, nodeID, dstCollID, dstParentID, -1)
						}
					}
				}
				confirmBtn.Children(func() {
					ui.Text(c, locales.MoveNodeConfirm()).FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.OnAccent).SingleLine()
				})
			})
		})
	})
}

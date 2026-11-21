package workspace

import (
	"fmt"
	"slices"
	"strings"
	"time"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/nav"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

// wsTab 表示 Workspaces 列表页顶部的筛选标签。
type wsTab int

const (
	wsTabAll wsTab = iota
	wsTabProject
	wsTabExternal
)

// wsSortOrder 表示名称列排序状态。
type wsSortOrder int

const (
	wsSortNone wsSortOrder = iota
	wsSortAsc
	wsSortDesc
)

// wsContentState 保存 Workspaces 右侧内容区的跨帧交互状态（通过 ui.Local 按窗口隔离）。
type wsContentState struct {
	tab        wsTab
	query      string
	typeFilter string // "" | "Internal" | "Partner" | "Public"
	sortOrder  wsSortOrder

	checked   map[domain.ID]bool
	iconCache map[string]cachedIcon

	// Create workspace 子页面状态
	createName       string
	createNameAuto   bool // 标记 createName 是否由选中目录自动带入
	createNameHandle ui.Handle
	createFocused    bool // 首次进入创建页时自动聚焦名称输入框
	createIcon       string
	createIconErr    string
	createIconPick   *imagePickResult
	createType       string // "Internal" | "Partner" | "Public"
	createSetup      string // "blank" | "local"
	createLocalDir   string
	createLocalErr   string
	createFolderPick *imagePickResult
	createFolderScan *folderScanResult
	localFolders     []localFolderItem
	createDesc       string
	createErr        string

	// 重命名弹窗状态
	renameOpen bool
	renameID   domain.ID
	renameName string
	renameErr  string

	// 删除确认弹窗状态
	deleteOpen bool
	deleteID   domain.ID
	deleteName string
	deleteErr  string
}

func newContentState() wsContentState {
	return wsContentState{
		checked:          make(map[domain.ID]bool),
		iconCache:        make(map[string]cachedIcon),
		createIconPick:   &imagePickResult{},
		createFolderPick: &imagePickResult{},
		createFolderScan: &folderScanResult{},
		createType:       "Internal",
		createSetup:      "blank",
	}
}

func (s *wsContentState) resetCreateForm() {
	s.createName = ""
	s.createNameAuto = false
	s.createIcon = ""
	s.createIconErr = ""
	s.createType = "Internal"
	s.createSetup = "blank"
	s.createLocalDir = ""
	s.createLocalErr = ""
	s.createDesc = ""
	s.createErr = ""
	if s.createFolderScan != nil {
		s.createFolderScan.reset()
	}
}

// wsRowItem 是离线首期精简后的工作区列表展示模型（对应设计稿 28/30/35–38）。
// 仅保留 Name、Creator（暂写死）、Last activity 与收藏/更多操作。
type wsRowItem struct {
	ID           domain.ID
	Name         string
	Default      bool
	Type         string
	Icon         string
	Starred      bool
	Creator      string
	LastActivity string
}

func formatRelativeActivity(t time.Time) string {
	if t.IsZero() {
		return "Just now"
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "Just now"
	case d < time.Hour:
		mins := int(d.Minutes())
		if mins == 1 {
			return "1 minute ago"
		}
		return fmt.Sprintf("%d minutes ago", mins)
	case d < 24*time.Hour:
		hrs := int(d.Hours())
		if hrs == 1 {
			return "1 hour ago"
		}
		return fmt.Sprintf("%d hours ago", hrs)
	case d < 48*time.Hour:
		return "Yesterday"
	case d < 7*24*time.Hour:
		return fmt.Sprintf("%d days ago", int(d.Hours()/24))
	default:
		return t.Format("2006-01-02")
	}
}

// ListView 绘制 Workspaces 列表页（挂载在 HubLayout 右侧子路由 /hub/workspaces）。
func ListView(c *ui.Context, db *store.Store, r nav.Navigator) {
	root := ui.Box(c).Fill()
	st := ui.Local(root, "ws-list-state", newContentState)
	root.Children(func() {
		workspacesListView(c, db, r, st)
	})
}

// CreateView 绘制「Create your workspace」独立表单页（挂载在 HubLayout 右侧子路由 /hub/workspaces/new）。
func CreateView(c *ui.Context, db *store.Store, r nav.Navigator) {
	root := ui.Box(c).Fill()
	st := ui.Local(root, "ws-create-state", newContentState)
	if !st.createFocused {
		st.createFocused = true
		st.createNameHandle.Focus(c)
	}
	root.Children(func() {
		createWorkspaceView(c, db, r, st)
	})
}

// 表格列宽约定（对齐 Pixso 设计稿 28/30/35–38：左右 12px padding，精简为 复选框 + Name + Creator + Last activity + 操作列）。
const (
	wsRowPadX     float32 = 12
	wsColCheckW   float32 = 36
	wsColCreatorW float32 = 180
	wsColActiveW  float32 = 195
	wsColActionW  float32 = 100
)

// workspacesListView 按审核通过的 Pixso 28 / 30 / 35–38 绘制 Workspaces 管理页。
func workspacesListView(c *ui.Context, db *store.Store, r nav.Navigator, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	items := filteredWorkspaces(db, st)

	ui.Column(c).Fill().Padding(22, m.Space6).Background(tk.BgBase).Children(func() {
		// 1. 顶部标题栏：Workspaces + Create workspace 主按钮（科技蓝 #336FFF）
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, "Workspaces").FontSize(m.FontXXL).Bold().TextColor(tk.TextPrimary).SingleLine()

			if wsPrimaryButton(c, "ws-create-btn", "Create workspace", false).Clicked() {
				if r != nil {
					r.Push(routes.HubWorkspaceNew)
				}
			}
		})

		ui.Box(c).Height(18)

		// 2. 筛选标签行：All / Project Workspaces / External
		ui.Row(c).Gap(m.Space15).AlignItems(ui.Center).Children(func() {
			wsFilterTab(c, "ws-tab-all", "All", st.tab == wsTabAll, func() {
				st.tab = wsTabAll
			})
			wsFilterTab(c, "ws-tab-project", "Project Workspaces", st.tab == wsTabProject, func() {
				st.tab = wsTabProject
			})
			wsFilterTab(c, "ws-tab-external", "External", st.tab == wsTabExternal, func() {
				st.tab = wsTabExternal
			})
		})

		ui.Box(c).Height(14)

		// 3. 搜索框 + 筛选按钮工具条（对齐设计稿 37）
		wsToolbar(c, st)

		ui.Box(c).Height(14)

		// 4. 工作区表格（表头 + 可滚动数据行 + 底部计数）
		wsTableHeader(c, st, items)

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Children(func() {
				if len(items) == 0 {
					wsEmptyState(c, st)
				} else {
					for _, item := range items {
						wsTableRow(c, db, r, st, item)
					}
				}
				wsTableFooter(c, st, items)
			})
		})
	})

	// 重命名与删除确认弹窗
	wsRenameDialog(c, db, st)
	wsDeleteDialog(c, db, st)
}

func wsFilterTab(c *ui.Context, key, label string, active bool, onSelect func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	btn := ui.ButtonBase(c.Key(key)).
		Height(26).PaddingX(10).Center().
		Radius(m.RadiusSM).Cursor(ui.CursorPointer).
		Role(ui.RoleTab).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case active:
		btn.Background(tk.BgSubtle).TextColor(tk.TextPrimary)
	case btn.Pressed():
		btn.Background(tk.BgHover).TextColor(tk.TextPrimary)
	case btn.Hovered():
		btn.Background(tk.BgSubtle).TextColor(tk.TextPrimary)
	default:
		btn.Background(ui.Transparent).TextColor(tk.TextSecondary)
	}

	if btn.Clicked() && onSelect != nil {
		onSelect()
	}

	btn.Children(func() {
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(500).SingleLine()
	})
}

func wsToolbar(c *ui.Context, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.Row(c).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
		// 搜索输入框：宽 302、高 32、圆角 4、前置放大镜、聚焦 1px Focus + 3px FocusRing、右侧清空按钮
		searchBox := ui.Row(c.Key("ws-search-box")).
			Width(302).Height(m.ButtonLarge).
			PaddingX(10).Gap(m.Space15).
			AlignItems(ui.Center).
			Radius(m.RadiusSM).
			Background(tk.BgSurface).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

		focused := searchBox.FocusWithin()
		switch {
		case focused:
			searchBox.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
		case searchBox.Hovered():
			searchBox.Border(m.Border, tk.BorderStrong)
		default:
			searchBox.Border(m.Border, tk.BorderPopover)
		}

		searchBox.Children(func() {
			iconColor := tk.TextMuted
			if focused || st.query != "" {
				iconColor = tk.TextPrimary
			}
			ui.Icon(c, icons.Search).FontSize(m.IconSM).TextColor(iconColor)
			ui.TextInputBase(c.Key("ws-search-input"), &st.query).
				Grow(1).FontSize(m.FontSM).TextColor(tk.TextPrimary).
				Placeholder("Search workspaces").PlaceholderColor(tk.TextMuted).
				Label("Search workspaces")

			if st.query != "" {
				clearBtn := ui.ButtonBase(c.Key("ws-search-clear")).
					Size(18, 18).Center().Radius(m.RadiusSM).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Clear search").
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				if clearBtn.Hovered() {
					clearBtn.Background(tk.BgHover).TextColor(tk.TextPrimary)
				} else {
					clearBtn.TextColor(tk.TextMuted)
				}
				if clearBtn.Clicked() {
					st.query = ""
				}
				clearBtn.Children(func() {
					ui.Icon(c, icons.X).FontSize(m.IconXS)
				})
			}
		})

		// 筛选按钮：32×32，激活态边框走 tk.Focus，点击弹出筛选/排序浮层
		filterBtn := ui.ButtonBase(c.Key("ws-filter-btn")).
			Size(m.SizeXL, m.SizeXL).Center().
			Radius(m.RadiusSM).Cursor(ui.CursorPointer).
			Role(ui.RoleButton).Label("Filter workspaces").
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

		filterOpen := ui.Local(filterBtn, "open", func() bool { return false })
		hasFilter := st.typeFilter != "" || st.sortOrder != wsSortNone

		switch {
		case filterBtn.Pressed():
			filterBtn.Background(tk.BgSelected).Border(m.Border, tk.BorderStrong).TextColor(tk.TextPrimary)
		case *filterOpen || hasFilter:
			filterBtn.Background(tk.BgSubtle).Border(m.Border, tk.Focus).TextColor(tk.TextPrimary)
		case filterBtn.Hovered():
			filterBtn.Background(tk.BgSubtle).Border(m.Border, tk.BorderStrong).TextColor(tk.TextPrimary)
		default:
			filterBtn.Background(tk.BgSurface).Border(m.Border, tk.BorderPopover).TextColor(tk.TextSecondary)
		}

		if filterBtn.Clicked() {
			*filterOpen = !*filterOpen
		}
		filterBtn.Children(func() {
			ui.Icon(c, icons.SlidersHorizontal).FontSize(m.IconSM)
		})

		v2.Popover(c, filterBtn, filterOpen, v2.PopoverOpts{Width: 196}, func(panel ui.Element) {
			ui.Column(c).FillWidth().Padding(m.Space15).Gap(m.Space05).Children(func() {
				ui.Text(c, "Workspace type").FontSize(m.FontMicro).FontWeight(500).
					TextColor(tk.TextMuted).Padding(m.Space1, m.Space2)

				typeOpt := func(key, label, val string) {
					wsPopoverOption(c, key, label, st.typeFilter == val, func() {
						st.typeFilter = val
						*filterOpen = false
					})
				}
				typeOpt("ws-f-all", "All types", "")
				typeOpt("ws-f-internal", "Internal", "Internal")
				typeOpt("ws-f-partner", "Partner", "Partner")
				typeOpt("ws-f-public", "Public", "Public")

				v2.PopoverDivider(c).MarginY(m.Space1)

				ui.Text(c, "Sort by name").FontSize(m.FontMicro).FontWeight(500).
					TextColor(tk.TextMuted).Padding(m.Space1, m.Space2)

				wsPopoverOption(c, "ws-s-none", "Default order", st.sortOrder == wsSortNone, func() {
					st.sortOrder = wsSortNone
					*filterOpen = false
				})
				wsPopoverOption(c, "ws-s-asc", "Name (A–Z)", st.sortOrder == wsSortAsc, func() {
					st.sortOrder = wsSortAsc
					*filterOpen = false
				})
				wsPopoverOption(c, "ws-s-desc", "Name (Z–A)", st.sortOrder == wsSortDesc, func() {
					st.sortOrder = wsSortDesc
					*filterOpen = false
				})
			})
		})
	})
}

func wsPopoverOption(c *ui.Context, key, label string, selected bool, onClick func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).FillWidth().Height(28).
		PaddingX(m.Space2).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.SpaceBetween).
		Cursor(ui.CursorPointer).Role(ui.RoleMenuItem).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.BgSelected).TextColor(tk.TextPrimary)
	case b.Hovered():
		b.Background(tk.BgSubtle).TextColor(tk.TextPrimary)
	default:
		b.TextColor(tk.TextPrimary)
	}
	if b.Clicked() && onClick != nil {
		onClick()
	}
	b.Children(func() {
		weight := 400
		if selected {
			weight = 500
		}
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(weight).SingleLine()
		if selected {
			ui.Icon(c, icons.Check).FontSize(m.IconXS).TextColor(tk.Focus)
		}
	})
}

func wsTableHeader(c *ui.Context, st *wsContentState, items []wsRowItem) {
	m, tk := thV2.Metrics, thV2.Of(c)

	// 计算全选复选框三态（未选 / 半选 / 全选）
	checkedCount := 0
	for _, it := range items {
		if st.checked[it.ID] {
			checkedCount++
		}
	}
	allState := ui.CheckOff
	if len(items) > 0 && checkedCount == len(items) {
		allState = ui.CheckOn
	} else if checkedCount > 0 {
		allState = ui.CheckMixed
	}

	// 表头左右留 12px padding，与数据行严格对齐
	ui.Row(c).FillWidth().Height(36).PaddingX(wsRowPadX).AlignItems(ui.Center).
		BorderWidth(m.Border, 0, m.Border, 0).BorderColor(tk.Border).
		Children(func() {
			// 全选复选框
			ui.Row(c).Width(wsColCheckW).AlignItems(ui.Center).Children(func() {
				cb := ui.CheckboxStateBase(c.Key("ws-select-all"), &allState).
					Cursor(ui.CursorPointer).Label("Select all workspaces")
				cb.OnClick(func() {
					want := allState == ui.CheckOn
					for _, it := range items {
						if want {
							st.checked[it.ID] = true
						} else {
							delete(st.checked, it.ID)
						}
					}
				})
				cb.Children(func() {
					wsCheckboxVisual(c, allState, cb.Hovered())
				})
			})

			// Name（可点击切换排序）
			ui.Row(c).Grow(1).MinWidth(240).AlignItems(ui.Center).Children(func() {
				nameHdr := ui.ButtonBase(c.Key("ws-hdr-name")).
					Gap(m.Space1).AlignItems(ui.Center).Justify(ui.Start).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Sort by Name").
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				if nameHdr.Hovered() {
					nameHdr.TextColor(tk.TextPrimary)
				} else {
					nameHdr.TextColor(tk.TextSecondary)
				}
				if nameHdr.Clicked() {
					switch st.sortOrder {
					case wsSortNone:
						st.sortOrder = wsSortAsc
					case wsSortAsc:
						st.sortOrder = wsSortDesc
					default:
						st.sortOrder = wsSortNone
					}
				}
				nameHdr.Children(func() {
					ui.Text(c, "Name").FontSize(m.FontXS).FontWeight(500).SingleLine()
					switch st.sortOrder {
					case wsSortAsc:
						ui.Text(c, "↑").FontSize(m.FontXS).TextColor(tk.Focus).SingleLine()
					case wsSortDesc:
						ui.Text(c, "↓").FontSize(m.FontXS).TextColor(tk.Focus).SingleLine()
					}
				})
			})

			wsHeaderCell(c, "Creator", wsColCreatorW)
			wsHeaderCell(c, "Last activity", wsColActiveW)

			// 右侧操作列占位
			ui.Box(c).Width(wsColActionW)
		})
}

func wsHeaderCell(c *ui.Context, title string, width float32) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Row(c).Width(width).AlignItems(ui.Center).Children(func() {
		ui.Text(c, title).FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary).SingleLine()
	})
}

func wsTableRow(c *ui.Context, db *store.Store, r nav.Navigator, st *wsContentState, item wsRowItem) {
	m, tk := thV2.Metrics, thV2.Of(c)

	isChecked := st.checked[item.ID]

	row := ui.Row(c.Key("ws-row-"+string(item.ID))).
		FillWidth().Height(50).PaddingX(wsRowPadX).AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.BgSubtle).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	hovered := row.Hovered()
	switch {
	case isChecked:
		row.Background(tk.AccentSoft)
	case hovered:
		row.Background(tk.BgBase.Mix(tk.TextPrimary, 0.04))
	default:
		row.Background(ui.Transparent)
	}

	openWorkspace := func() {
		if db != nil {
			_ = db.SetActiveWorkspace(item.ID)
		}
		if r != nil {
			r.Push(routes.WorkbenchCollectionsOf(string(item.ID)))
		}
	}

	row.Children(func() {
		// 1. 行复选框
		ui.Row(c).Width(wsColCheckW).AlignItems(ui.Center).Children(func() {
			checkedVal := isChecked
			cb := ui.CheckboxBase(c.Key("ws-chk-"+string(item.ID)), &checkedVal).
				Cursor(ui.CursorPointer).Label("Select " + item.Name)
			cb.OnClick(func() {
				if checkedVal {
					st.checked[item.ID] = true
				} else {
					delete(st.checked, item.ID)
				}
			})
			state := ui.CheckOff
			if isChecked {
				state = ui.CheckOn
			}
			cb.Children(func() {
				wsCheckboxVisual(c, state, cb.Hovered())
			})
		})

		// 2. Name 列（26×26 圆形图标：支持本地自定义图片替换默认图标 + 双行名称/类型）
		ui.Row(c).Grow(1).MinWidth(240).AlignItems(ui.Center).Padding(0, m.Space2, 0, 0).Children(func() {
			nameBtn := ui.ButtonBase(c.Key("ws-open-"+string(item.ID))).
				FillWidth().Justify(ui.Start).AlignItems(ui.Center).
				Gap(10).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label(item.Name)
			if nameBtn.Clicked() {
				openWorkspace()
			}
			nameBtn.Children(func() {
				wsIconBadge(c, st, item.Icon, 26, false)

				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, item.Name).FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.TextPrimary).SingleLine()
					ui.Text(c, item.Type).FontSize(m.FontXS).
						TextColor(tk.TextMuted).SingleLine()
				})
			})
		})

		// 3. Creator 列（淡科技蓝圆点 + 名称，一期离线暂写死 "You"）
		ui.Row(c).Width(wsColCreatorW).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
			ui.Box(c).Size(18, 18).Radius(m.RadiusFull).Shrink(0).
				Background(tk.BgBase.Mix(tk.Accent, 0.24))
			ui.Text(c, item.Creator).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 4. Last activity 列
		ui.Row(c).Width(wsColActiveW).AlignItems(ui.Center).Children(func() {
			ui.Text(c, item.LastActivity).FontSize(m.FontSM).TextColor(tk.TextSecondary).SingleLine()
		})

		// 5. 右侧操作列：收藏星标（持久化） + 更多菜单（对齐设计稿 35/38）
		ui.Row(c).Width(wsColActionW).AlignItems(ui.Center).Justify(ui.End).Gap(m.Space15).Children(func() {
			starBtn := ui.ButtonBase(c.Key("ws-star-"+string(item.ID))).
				Size(m.SizeMD, m.SizeMD).Center().Radius(m.RadiusSM).
				Cursor(ui.CursorPointer).Role(ui.RoleToggleButton).
				Label("Favorite " + item.Name).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case starBtn.Pressed():
				starBtn.Background(tk.BgSelected)
			case starBtn.Hovered():
				starBtn.Background(tk.BgHover)
			}
			starColor := tk.TextMuted
			if item.Starred {
				starColor = tk.Warning
			} else if starBtn.Hovered() || hovered {
				starColor = tk.TextPrimary
			}
			if starBtn.Clicked() && db != nil {
				_ = db.SetWorkspaceStarred(item.ID, !item.Starred)
			}
			starBtn.Children(func() {
				ui.Icon(c, icons.Star).FontSize(m.IconSM).TextColor(starColor)
			})

			moreBtn := ui.ButtonBase(c.Key("ws-more-"+string(item.ID))).
				Size(m.SizeMD, m.SizeMD).Center().Radius(m.RadiusSM).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).
				Label("More actions for " + item.Name).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			moreOpen := ui.Local(moreBtn, "open", func() bool { return false })
			switch {
			case moreBtn.Pressed() || *moreOpen:
				moreBtn.Background(tk.BgSelected).TextColor(tk.TextPrimary)
			case moreBtn.Hovered():
				moreBtn.Background(tk.BgHover).TextColor(tk.TextPrimary)
			case hovered:
				moreBtn.TextColor(tk.TextPrimary)
			default:
				moreBtn.TextColor(tk.TextMuted)
			}
			if moreBtn.Clicked() {
				*moreOpen = !*moreOpen
			}
			moreBtn.Children(func() {
				ui.Icon(c, icons.Ellipsis).FontSize(m.IconSM)
			})

			v2.Popover(c, moreBtn, moreOpen, v2.PopoverOpts{Width: 168}, func(panel ui.Element) {
				ui.Column(c).FillWidth().Padding(m.Space15).Gap(m.Space05).Children(func() {
					if v2.MenuButton(c, v2.MenuButtonProps{
						Key:   "ws-act-open-" + string(item.ID),
						Label: "Open workspace",
						Icon:  icons.ArrowRight,
					}).Clicked() {
						*moreOpen = false
						openWorkspace()
					}

					if v2.MenuButton(c, v2.MenuButtonProps{
						Key:   "ws-act-rename-" + string(item.ID),
						Label: "Rename",
						Icon:  icons.FilePen,
					}).Clicked() {
						*moreOpen = false
						st.renameID = item.ID
						st.renameName = item.Name
						st.renameErr = ""
						st.renameOpen = true
					}

					if !item.Default {
						v2.PopoverDivider(c).MarginY(m.Space1)
						delBtn := ui.ButtonBase(c.Key("ws-act-del-"+string(item.ID))).
							FillWidth().Height(30).PaddingX(8).Radius(m.RadiusSM).
							AlignItems(ui.Center).Justify(ui.Start).Gap(m.Space2).
							Cursor(ui.CursorPointer).Role(ui.RoleMenuItem).Label("Delete").
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						if delBtn.Hovered() {
							delBtn.Background(tk.DangerSoft)
						}
						if delBtn.Clicked() {
							*moreOpen = false
							st.deleteID = item.ID
							st.deleteName = item.Name
							st.deleteErr = ""
							st.deleteOpen = true
						}
						delBtn.Children(func() {
							ui.Icon(c, icons.Trash).FontSize(m.IconSM).TextColor(tk.Danger)
							ui.Text(c, "Delete").FontSize(m.FontSM).FontWeight(500).
								TextColor(tk.Danger).SingleLine()
						})
					}
				})
			})
		})
	})
}

func wsCheckboxVisual(c *ui.Context, state ui.CheckState, hovered bool) {
	m, tk := thV2.Metrics, thV2.Of(c)
	box := ui.Box(c).Size(14, 14).Radius(3).Center().
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch state {
	case ui.CheckOn:
		box.Background(tk.Accent).Border(m.Border, tk.Accent)
		box.Children(func() {
			ui.Icon(c, icons.Check).FontSize(10).TextColor(tk.OnAccent)
		})
	case ui.CheckMixed:
		box.Background(tk.Accent).Border(m.Border, tk.Accent)
		box.Children(func() {
			ui.Box(c).Size(8, 2).Radius(1).Background(tk.OnAccent)
		})
	default:
		border := tk.BorderPopover
		if hovered {
			border = tk.BorderStrong
		}
		box.Background(tk.BgSurface).Border(m.Border, border)
	}
}

func wsTableFooter(c *ui.Context, st *wsContentState, items []wsRowItem) {
	m, tk := thV2.Metrics, thV2.Of(c)

	countLabel := fmt.Sprintf("%d workspaces", len(items))
	if len(items) == 1 {
		countLabel = "1 workspace"
	}

	checkedCount := 0
	for _, it := range items {
		if st.checked[it.ID] {
			checkedCount++
		}
	}

	ui.Row(c).FillWidth().Padding(m.Space3, wsRowPadX).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
		ui.Text(c, countLabel).FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()

		if checkedCount > 0 {
			ui.Text(c, fmt.Sprintf("· %d selected", checkedCount)).
				FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextSecondary).SingleLine()

			clr := ui.ButtonBase(c.Key("ws-clear-sel")).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Clear selection")
			if clr.Hovered() {
				clr.TextColor(tk.AccentHover)
			} else {
				clr.TextColor(tk.Focus)
			}
			if clr.Clicked() {
				clear(st.checked)
			}
			clr.Children(func() {
				ui.Text(c, "Clear selection").FontSize(m.FontXS).FontWeight(500).SingleLine()
			})
		}
	})
}

func wsEmptyState(c *ui.Context, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	hasFilter := st.query != "" || st.typeFilter != "" || st.tab != wsTabAll
	title := "No workspaces yet"
	desc := "Create a workspace to organize your collections, environments, and APIs."
	if hasFilter {
		title = "No matching workspaces"
		desc = "Try adjusting your search keyword or active filters."
	}

	ui.Column(c).FillWidth().Padding(48, m.Space4).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
		ui.Box(c).Size(44, 44).Radius(m.RadiusFull).Background(tk.BgSubtle).Center().Children(func() {
			icon := icons.LayoutGrid
			if hasFilter {
				icon = icons.Search
			}
			ui.Icon(c, icon).FontSize(m.IconLG).TextColor(tk.TextMuted)
		})
		ui.Text(c, title).FontSize(m.FontMD).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
		ui.Text(c, desc).FontSize(m.FontSM).TextColor(tk.TextSecondary)

		if hasFilter {
			resetBtn := ui.ButtonBase(c.Key("ws-reset-filters")).
				Margin(m.Space2, 0, 0, 0).Height(m.ButtonHeight).PaddingX(m.Space3).
				Radius(m.RadiusSM).Border(m.Border, tk.BorderPopover).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Clear filters").
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if resetBtn.Hovered() {
				resetBtn.Background(tk.BgHover)
			} else {
				resetBtn.Background(tk.BgSurface)
			}
			if resetBtn.Clicked() {
				st.tab = wsTabAll
				st.query = ""
				st.typeFilter = ""
			}
			resetBtn.Children(func() {
				ui.Text(c, "Clear filters").FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
			})
		}
	})
}

func wsRenameDialog(c *ui.Context, db *store.Store, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.DialogBase(c, &st.renameOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(380).Padding(m.Space5).Gap(m.Space3).
			Background(tk.BgSurface).Border(m.Border, tk.BorderPopover).Radius(m.RadiusLG).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.18))

		panel.Children(func() {
			ui.Text(c, "Rename workspace").FontSize(m.FontLG).FontWeight(600).TextColor(tk.TextPrimary)

			in := ui.TextInputBase(c.Key("ws-rename-input"), &st.renameName).
				FillWidth().Height(m.ButtonLarge).PaddingX(m.Space3).
				Radius(m.RadiusSM).Background(tk.BgSurface).
				FontSize(m.FontSM).TextColor(tk.TextPrimary).
				Placeholder("Workspace name").PlaceholderColor(tk.TextMuted).
				Label("Workspace name")
			switch {
			case st.renameErr != "":
				in.Border(m.Border, tk.Danger)
			case in.Focused():
				in.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
			default:
				in.Border(m.Border, tk.BorderPopover)
			}

			if st.renameErr != "" {
				ui.Text(c, st.renameErr).FontSize(m.FontXS).TextColor(tk.Danger)
			}

			submit := func() {
				name := strings.TrimSpace(st.renameName)
				if name == "" {
					st.renameErr = "Workspace name cannot be empty."
					return
				}
				if db != nil {
					if err := db.RenameWorkspace(st.renameID, name); err != nil {
						st.renameErr = err.Error()
						return
					}
				}
				st.renameErr = ""
				st.renameOpen = false
			}
			if in.Submitted() {
				submit()
			}

			ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space1, 0, 0, 0).Children(func() {
				cancelBtn := wsSecondaryButton(c, "ws-rename-cancel", "Cancel")
				if cancelBtn.Clicked() {
					st.renameOpen = false
					st.renameErr = ""
				}
				saveBtn := wsPrimaryButton(c, "ws-rename-save", "Save", strings.TrimSpace(st.renameName) == "")
				if saveBtn.Clicked() {
					submit()
				}
			})
		})
	})
}

func wsDeleteDialog(c *ui.Context, db *store.Store, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	ui.DialogBase(c, &st.deleteOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(380).Padding(m.Space5).Gap(m.Space3).
			Background(tk.BgSurface).Border(m.Border, tk.BorderPopover).Radius(m.RadiusLG).
			Shadow(0, 12, 32, 0, ui.RGBA(0, 0, 0, 0.18))

		panel.Children(func() {
			ui.Text(c, "Delete workspace").FontSize(m.FontLG).FontWeight(600).TextColor(tk.TextPrimary)
			ui.Text(c, fmt.Sprintf("Remove \"%s\" from your workspace list? Files on disk remain untouched.", st.deleteName)).
				FontSize(m.FontSM).TextColor(tk.TextSecondary)

			if st.deleteErr != "" {
				ui.Text(c, st.deleteErr).FontSize(m.FontXS).TextColor(tk.Danger)
			}

			ui.Row(c).FillWidth().Justify(ui.End).Gap(m.Space2).Margin(m.Space1, 0, 0, 0).Children(func() {
				cancelBtn := wsSecondaryButton(c, "ws-del-cancel", "Cancel")
				if cancelBtn.Clicked() {
					st.deleteOpen = false
					st.deleteErr = ""
				}
				delBtn := ui.ButtonBase(c.Key("ws-del-confirm")).
					Height(m.ButtonLarge).PaddingX(m.Space4).Center().
					Radius(m.RadiusSM).Cursor(ui.CursorPointer).
					Role(ui.RoleButton).Label("Delete").
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				if delBtn.Hovered() {
					delBtn.Background(tk.Danger.Alpha(0.88))
				} else {
					delBtn.Background(tk.Danger)
				}
				if delBtn.Clicked() {
					if db != nil {
						if err := db.RemoveWorkspace(st.deleteID); err != nil {
							st.deleteErr = err.Error()
							return
						}
					}
					delete(st.checked, st.deleteID)
					st.deleteErr = ""
					st.deleteOpen = false
				}
				delBtn.Children(func() {
					ui.Text(c, "Delete").FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.OnAccent).SingleLine()
				})
			})
		})
	})
}

func wsPrimaryButton(c *ui.Context, key, label string, disabled bool) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).
		Height(m.ButtonLarge).PaddingX(m.Space4).Center().
		Radius(m.RadiusSM).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	if disabled {
		b.Disabled(true).Background(tk.BgBase.Mix(tk.Accent, 0.35))
	} else {
		b.Cursor(ui.CursorPointer)
		switch {
		case b.Pressed():
			b.Background(tk.AccentPressed)
		case b.Hovered():
			b.Background(tk.AccentHover)
		default:
			b.Background(tk.Accent)
		}
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(600).TextColor(tk.OnAccent).SingleLine()
	})
	return b
}

func wsSecondaryButton(c *ui.Context, key, label string) ui.Element {
	m, tk := thV2.Metrics, thV2.Of(c)
	b := ui.ButtonBase(c.Key(key)).
		Height(m.ButtonLarge).PaddingX(m.Space4).Center().
		Radius(m.RadiusSM).Border(m.Border, tk.BorderPopover).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case b.Pressed():
		b.Background(tk.BgSelected)
	case b.Hovered():
		b.Background(tk.BgHover)
	default:
		b.Background(tk.BgSurface)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(m.FontSM).FontWeight(500).TextColor(tk.TextPrimary).SingleLine()
	})
	return b
}

func filteredWorkspaces(db *store.Store, st *wsContentState) []wsRowItem {
	if db == nil {
		return nil
	}
	snap := db.Snapshot()
	if snap == nil {
		return nil
	}

	kw := strings.ToLower(strings.TrimSpace(st.query))
	out := make([]wsRowItem, 0, len(snap.Workspaces))

	for _, w := range snap.Workspaces {
		wsType := strings.TrimSpace(w.Type)
		if wsType == "" {
			wsType = "Internal"
		}

		// 顶部 Tab 筛选
		switch st.tab {
		case wsTabProject:
			if wsType != "Internal" {
				continue
			}
		case wsTabExternal:
			if wsType != "Partner" && wsType != "Public" {
				continue
			}
		}

		// 筛选弹层类型过滤
		if st.typeFilter != "" && wsType != st.typeFilter {
			continue
		}

		// 关键字搜索（匹配名称、类型、描述）
		if kw != "" {
			match := strings.Contains(strings.ToLower(w.Name), kw) ||
				strings.Contains(strings.ToLower(wsType), kw) ||
				strings.Contains(strings.ToLower(w.Description), kw)
			if !match {
				continue
			}
		}

		out = append(out, wsRowItem{
			ID:           w.ID,
			Name:         w.Name,
			Default:      w.Default,
			Type:         wsType,
			Icon:         w.Icon,
			Starred:      w.Starred,
			Creator:      "You",
			LastActivity: formatRelativeActivity(w.UpdatedAt),
		})
	}

	switch st.sortOrder {
	case wsSortAsc:
		slices.SortStableFunc(out, func(a, b wsRowItem) int {
			return strings.Compare(strings.ToLower(a.Name), strings.ToLower(b.Name))
		})
	case wsSortDesc:
		slices.SortStableFunc(out, func(a, b wsRowItem) int {
			return strings.Compare(strings.ToLower(b.Name), strings.ToLower(a.Name))
		})
	}

	return out
}


package workspace

import (
	"fmt"
	"slices"
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/internal/nav"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

const (
	minSidebarWidth float32 = 180 // 最小宽度
	maxSidebarWidth float32 = 460 // 最大宽度
	defaultWidth    float32 = 232 // 默认宽度
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

	checked map[domain.ID]bool
	starred map[domain.ID]bool
	types   map[domain.ID]string // 本次会话创建的工作区类型映射，默认 "Internal"

	// Create workspace 子页面状态
	creating         bool
	createName       string
	createNameHandle ui.Handle
	createType       string // "Internal" | "Partner" | "Public"
	createAccess string
	createSetup  string // "blank" | "local"
	createAI     string
	createErr    string

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
		checked:      make(map[domain.ID]bool),
		starred:      make(map[domain.ID]bool),
		types:        make(map[domain.ID]string),
		createType:   "Internal",
		createAccess: "Everyone in wang's Team",
		createSetup:  "blank",
	}
}

func (s *wsContentState) typeOf(id domain.ID) string {
	if t, ok := s.types[id]; ok && t != "" {
		return t
	}
	return "Internal"
}

func (s *wsContentState) resetCreateForm() {
	s.createName = ""
	s.createType = "Internal"
	s.createAccess = "Everyone in wang's Team"
	s.createSetup = "blank"
	s.createAI = ""
	s.createErr = ""
}

// wsRowItem 是工作区列表的展示模型，元数据字段在 domain 未扩展前遵循 03-page-inventory.md §5 约定。
type wsRowItem struct {
	ID           domain.ID
	Name         string
	Default      bool
	Type         string
	Creator      string
	Contributors int
	LastActivity string
	Access       string
	Role         string
}

func accessLabelFor(wsType string) string {
	switch wsType {
	case "Partner":
		return "Team and partners"
	case "Public":
		return "Anyone on Postman"
	default:
		return "Everyone in your team"
	}
}

func View(c *ui.Context, db *store.Store, r nav.Navigator) {
	root := ui.Box(c).Grow(1)
	width := ui.Local(root, "ws-sidebar-width", func() float32 {
		return defaultWidth
	})
	activeMenu := ui.Local(root, "ws-sidebar-active", func() string {
		return "Workspaces"
	})
	st := ui.Local(root, "ws-content-state", newContentState)

	// 限制拖动范围
	if *width < minSidebarWidth {
		*width = minSidebarWidth
	} else if *width > maxSidebarWidth {
		*width = maxSidebarWidth
	}
	root.Children(func() {
		ui.Split(c, width, func() {
			siderBar(c, activeMenu, st)
		}, func() {
			content(c, db, r, *activeMenu, st)
		}).Fill().SplitReset(defaultWidth)
	})
}

func siderBar(c *ui.Context, activeMenu *string, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Column(c).Fill().MinWidth(minSidebarWidth).Background(tk.BgBase).Children(func() {
		// 1. 团队信息栏 (48px)
		ui.Row(c).FillWidth().Height(48).AlignItems(ui.Center).Padding(m.Space2, m.Space3).Gap(m.Space2).Children(func() {
			// 头像：26x26，圆形，浅灰底，描边，字母 "W"
			ui.Avatar(c, "W", nil).Size(26, 26).Border(1, tk.BorderPopover).
				Background(tk.BgSubtle).TextColor(tk.TextSecondary)

			ui.Column(c).Children(func() {
				ui.Text(c, "wang's Team").FontSize(13).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
				ui.Text(c, "1 member").FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()
			})

			ui.Spacer(c)

			// Invite 按钮：高度 22px，圆角 4px，边框 BorderPopover，字号 11px
			inviteBtn := ui.ButtonBase(c.Key("team-invite")).
				Height(22).PaddingX(8).Radius(m.RadiusSM).
				Border(m.Border, tk.BorderPopover).
				Cursor(ui.CursorPointer).
				Role(ui.RoleButton).
				Label("Invite").
				AlignItems(ui.Center).
				Justify(ui.Center).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			if inviteBtn.Hovered() {
				inviteBtn.Background(tk.BgHover).TextColor(tk.TextPrimary)
			} else {
				inviteBtn.TextColor(tk.TextSecondary)
			}
			inviteBtn.Children(func() {
				ui.Text(c, "Invite").FontSize(m.FontXS).SingleLine()
			})
		})

		// 分割线：左右留白 12px，下方留白 6px
		ui.Divider(c).BorderColor(tk.BgSubtle).Margin(0, m.Space3, m.Space15, m.Space3)

		// 2. 菜单项（每项高 30px，内部图标 14px，文字 12px）
		ui.Column(c).PaddingX(m.Space2).Children(func() {
			menuItem := func(key, label string, icon *ui.SVG) {
				btn := v2.MenuButton(c, v2.MenuButtonProps{
					Icon:     icon,
					Key:      key,
					Label:    label,
					Selected: *activeMenu == key,
				})
				if btn.Clicked() {
					*activeMenu = key
					st.creating = false
				}
			}

			// Group 1
			menuItem("Home", "Home", icons.House)
			menuItem("Workspaces", "Workspaces", icons.LayoutGrid)
			menuItem("ApplicationInventory", "Application Inventory", icons.Boxes)
			menuItem("APICatalog", "API Catalog", icons.BookOpen)

			ui.Divider(c).BorderColor(tk.BgSubtle).MarginX(m.Space1).MarginY(m.Space15)

			// Group 2
			menuItem("PrivateAPINetwork", "Private API Network", icons.Lock)
			menuItem("PublicAPINetwork", "Public API Network", icons.Globe)

			ui.Divider(c).BorderColor(tk.BgSubtle).MarginX(m.Space1).MarginY(m.Space15)

			// Group 3
			menuItem("AgentContext", "Agent Context", icons.Sparkles)
			menuItem("Integrations", "Integrations", icons.Plug)
			menuItem("Reports", "Reports", icons.ChartColumn)
		})

		// 3. 弹性空白
		ui.Spacer(c)

		// 4. 底部帮助链接（字号 10px，灰色 TextMuted，底部 padding 14px）
		bottomLinks(c)
	})
}

func bottomLinks(c *ui.Context) {
	m, tk := thV2.Metrics, thV2.Of(c)
	links := []string{
		"What is Postman",
		"How to Publish",
		"Learning Center",
		"Support Center",
		"Postman Enterprise",
	}

	ui.Column(c).Padding(0, m.Space4, 14, m.Space4).Gap(9).Children(func() {
		for _, title := range links {
			t := title
			link := ui.ButtonBase(c.Key("link-" + t)).
				Role(ui.RoleLink).
				Cursor(ui.CursorPointer).
				Label(t).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast}).
				Justify(ui.Start)

			if link.Hovered() {
				link.TextColor(tk.TextPrimary)
			} else {
				link.TextColor(tk.TextMuted)
			}

			link.Children(func() {
				ui.Text(c, t).FontSize(m.FontMicro).SingleLine()
			})
		}
	})
}

// content 根据侧栏选中项渲染右侧主内容区。
func content(c *ui.Context, db *store.Store, r nav.Navigator, activeMenu string, st *wsContentState) {
	if activeMenu != "Workspaces" {
		hubPlaceholder(c, activeMenu)
		return
	}
	if st.creating {
		createWorkspaceView(c, db, r, st)
		return
	}
	workspacesListView(c, db, r, st)
}

// 表格列宽约定（对齐设计稿 ws-manage.png，表头与行共用，保证像素级垂直对齐）。
const (
	wsColCheckW   float32 = 35
	wsColCreatorW float32 = 100
	wsColContribW float32 = 98
	wsColActiveW  float32 = 110
	wsColAccessW  float32 = 165
	wsColRoleW    float32 = 110
	wsColActionW  float32 = 64
)

// workspacesListView 按设计稿 28 / 30（Dark）绘制 Workspaces 管理页。
func workspacesListView(c *ui.Context, db *store.Store, r nav.Navigator, st *wsContentState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	items := filteredWorkspaces(db, st)

	ui.Column(c).Fill().Padding(22, m.Space6).Background(tk.BgBase).Children(func() {
		// 1. 顶部标题栏：Workspaces + Create workspace 主按钮
		ui.Row(c).FillWidth().AlignItems(ui.Center).Justify(ui.SpaceBetween).Children(func() {
			ui.Text(c, "Workspaces").FontSize(m.FontXXL).Bold().TextColor(tk.TextPrimary).SingleLine()

			createBtn := ui.ButtonBase(c.Key("ws-create-btn")).
				Height(m.ButtonLarge).PaddingX(m.Space4).Center().
				Radius(m.RadiusSM).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label("Create workspace").
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			switch {
			case createBtn.Pressed():
				createBtn.Background(tk.AccentPressed)
			case createBtn.Hovered():
				createBtn.Background(tk.AccentHover)
			default:
				createBtn.Background(tk.Accent)
			}
			if createBtn.Clicked() {
				st.resetCreateForm()
				st.creating = true
				st.createNameHandle.Focus(c)
			}
			createBtn.Children(func() {
				ui.Text(c, "Create workspace").FontSize(m.FontSM).FontWeight(600).
					TextColor(tk.OnAccent).SingleLine()
			})
		})

		ui.Box(c).Height(18)

		// 2. 筛选标签行：All / Project Workspaces / External（顺序严格对齐设计稿）
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

		// 3. 搜索框 + 筛选按钮工具条
		wsToolbar(c, st)

		ui.Box(c).Height(14)

		// 4. 工作区表格（顶部分割线 + 表头 + 可滚动数据行 + 底部计数）
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
		// 搜索输入框：宽 302、高 32、圆角 4、前置放大镜图标
		searchBox := ui.Row(c.Key("ws-search-box")).
			Width(302).Height(m.ButtonLarge).
			PaddingX(m.Space2).Gap(m.Space15).
			AlignItems(ui.Center).
			Radius(m.RadiusSM).
			Background(tk.BgSurface).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

		switch {
		case searchBox.FocusWithin():
			searchBox.Border(m.Border, tk.Focus).Shadow(0, 0, 0, 3, tk.FocusRing)
		case searchBox.Hovered():
			searchBox.Border(m.Border, tk.BorderStrong)
		default:
			searchBox.Border(m.Border, tk.BorderPopover)
		}

		searchBox.Children(func() {
			ui.Icon(c, icons.Search).FontSize(m.IconSM).TextColor(tk.TextMuted)
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

		// 筛选按钮：32×32，与搜索框同高同边框，图标 sliders-horizontal
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
				ui.Text(c, "Workspace type").FontSize(m.FontXS).FontWeight(500).
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

				ui.Text(c, "Sort by name").FontSize(m.FontXS).FontWeight(500).
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
		ui.Text(c, label).FontSize(m.FontSM).SingleLine()
		if selected {
			ui.Icon(c, icons.Check).FontSize(m.IconXS).TextColor(tk.Accent)
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

	// 表头顶部 1px 分割线 + 35px 表头 + 底部 1px 分割线
	ui.Row(c).FillWidth().Height(36).AlignItems(ui.Center).
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
			ui.Row(c).Grow(1).MinWidth(220).AlignItems(ui.Center).Children(func() {
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
						ui.Text(c, "↑").FontSize(m.FontXS).TextColor(tk.Accent).SingleLine()
					case wsSortDesc:
						ui.Text(c, "↓").FontSize(m.FontXS).TextColor(tk.Accent).SingleLine()
					}
				})
			})

			wsHeaderCell(c, "Creator", wsColCreatorW)
			wsHeaderCell(c, "Contributors", wsColContribW)
			wsHeaderCell(c, "Last activity", wsColActiveW)
			wsHeaderCell(c, "Access", wsColAccessW)
			wsHeaderCell(c, "Your role", wsColRoleW)

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
	isStarred := st.starred[item.ID]

	row := ui.Row(c.Key("ws-row-"+string(item.ID))).
		FillWidth().Height(52).AlignItems(ui.Center).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.BgSubtle).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

	switch {
	case isChecked:
		row.Background(tk.AccentSoft)
	case row.Hovered():
		row.Background(tk.BgBase.Mix(tk.TextPrimary, 0.035))
	default:
		row.Background(ui.Transparent)
	}

	openWorkspace := func() {
		if db != nil {
			_ = db.SetActiveWorkspace(item.ID)
		}
		if r != nil {
			r.Push(routes.Collections)
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

		// 2. Name 列（26×26 圆形浅底图标 + 双行名称/类型，点击进入工作台）
		ui.Row(c).Grow(1).MinWidth(220).AlignItems(ui.Center).Padding(0, m.Space2, 0, 0).Children(func() {
			nameBtn := ui.ButtonBase(c.Key("ws-open-"+string(item.ID))).
				FillWidth().Justify(ui.Start).AlignItems(ui.Center).
				Gap(10).Cursor(ui.CursorPointer).
				Role(ui.RoleButton).Label(item.Name)
			if nameBtn.Clicked() {
				openWorkspace()
			}
			nameBtn.Children(func() {
				ui.Box(c).Size(26, 26).Radius(m.RadiusFull).
					Background(tk.BgSubtle).Center().Shrink(0).
					Children(func() {
						ui.Icon(c, icons.LayoutGrid).FontSize(m.IconSM).TextColor(tk.TextSecondary)
					})

				ui.Column(c).Gap(1).Grow(1).Children(func() {
					ui.Text(c, item.Name).FontSize(m.FontSM).FontWeight(600).
						TextColor(tk.TextPrimary).SingleLine()
					ui.Text(c, item.Type).FontSize(m.FontXS).
						TextColor(tk.TextMuted).SingleLine()
				})
			})
		})

		// 3. Creator 列（暖色调圆形头像 + 名称）
		ui.Row(c).Width(wsColCreatorW).AlignItems(ui.Center).Gap(m.Space15).Children(func() {
			ui.Box(c).Size(18, 18).Radius(m.RadiusFull).Shrink(0).
				Background(tk.BgBase.Mix(tk.Accent, 0.22))
			ui.Text(c, item.Creator).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 4. Contributors 列
		ui.Row(c).Width(wsColContribW).AlignItems(ui.Center).Children(func() {
			ui.Textf(c, "%d", item.Contributors).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 5. Last activity 列
		ui.Row(c).Width(wsColActiveW).AlignItems(ui.Center).Children(func() {
			ui.Text(c, item.LastActivity).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 6. Access 列
		ui.Row(c).Width(wsColAccessW).AlignItems(ui.Center).Children(func() {
			ui.Text(c, item.Access).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 7. Your role 列
		ui.Row(c).Width(wsColRoleW).AlignItems(ui.Center).Children(func() {
			ui.Text(c, item.Role).FontSize(m.FontSM).TextColor(tk.TextPrimary).SingleLine()
		})

		// 8. 右侧操作列：收藏星标 + 更多菜单
		ui.Row(c).Width(wsColActionW).AlignItems(ui.Center).Justify(ui.End).Gap(m.Space1).Children(func() {
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
			if isStarred {
				starColor = tk.Warning
			} else if starBtn.Hovered() {
				starColor = tk.TextPrimary
			}
			if starBtn.Clicked() {
				if isStarred {
					delete(st.starred, item.ID)
				} else {
					st.starred[item.ID] = true
				}
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

	ui.Row(c).FillWidth().Padding(m.Space3, 0).AlignItems(ui.Center).Gap(m.Space2).Children(func() {
		ui.Text(c, countLabel).FontSize(m.FontXS).TextColor(tk.TextMuted).SingleLine()

		if checkedCount > 0 {
			ui.Text(c, fmt.Sprintf("· %d selected", checkedCount)).
				FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()

			clr := ui.ButtonBase(c.Key("ws-clear-sel")).
				Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Clear selection")
			if clr.Hovered() {
				clr.TextColor(tk.TextPrimary)
			} else {
				clr.TextColor(tk.Info)
			}
			if clr.Clicked() {
				clear(st.checked)
			}
			clr.Children(func() {
				ui.Text(c, "Clear selection").FontSize(m.FontXS).SingleLine()
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
					delete(st.starred, st.deleteID)
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
		b.Disabled(true).Background(tk.BgBase.Mix(tk.Accent, 0.45))
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
		wsType := st.typeOf(w.ID)
		access := accessLabelFor(wsType)

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

		// 关键字搜索
		if kw != "" {
			match := strings.Contains(strings.ToLower(w.Name), kw) ||
				strings.Contains(strings.ToLower(wsType), kw) ||
				strings.Contains(strings.ToLower(access), kw)
			if !match {
				continue
			}
		}

		out = append(out, wsRowItem{
			ID:           w.ID,
			Name:         w.Name,
			Default:      w.Default,
			Type:         wsType,
			Creator:      "You",
			Contributors: 1,
			LastActivity: "Just now",
			Access:       access,
			Role:         "Admin",
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

// hubPlaceholder 为管理端尚未出独立设计帧的侧栏页面渲染统一空态骨架。
func hubPlaceholder(c *ui.Context, activeMenu string) {
	m, tk := thV2.Metrics, thV2.Of(c)

	type meta struct {
		title string
		desc  string
		icon  *ui.SVG
	}
	info := map[string]meta{
		"Home":                 {"Home", "Recent workspaces and team activity will appear here.", icons.House},
		"ApplicationInventory": {"Application Inventory", "Discover and track applications across your team.", icons.Boxes},
		"APICatalog":           {"API Catalog", "Browse and manage your organization's API catalog.", icons.BookOpen},
		"PrivateAPINetwork":    {"Private API Network", "Share internal APIs, collections, and workspaces with your team.", icons.Lock},
		"PublicAPINetwork":     {"Public API Network", "Explore and publish public APIs for the community.", icons.Globe},
		"AgentContext":         {"Agent Context", "Configure context and tools for AI agents.", icons.Sparkles},
		"Integrations":         {"Integrations", "Connect third-party services and automate workflows.", icons.Plug},
		"Reports":              {"Reports", "Review API usage, test runs, and workspace metrics.", icons.ChartColumn},
	}[activeMenu]
	if info.title == "" {
		info = meta{title: activeMenu, desc: "This section is under construction.", icon: icons.LayoutGrid}
	}

	ui.Column(c).Fill().Padding(22, m.Space6).Background(tk.BgBase).Children(func() {
		ui.Text(c, info.title).FontSize(m.FontXXL).Bold().TextColor(tk.TextPrimary).SingleLine()

		ui.Column(c).Grow(1).FillWidth().Center().Gap(m.Space2).Children(func() {
			ui.Box(c).Size(48, 48).Radius(m.RadiusFull).Background(tk.BgSubtle).Center().Children(func() {
				ui.Icon(c, info.icon).FontSize(22).TextColor(tk.TextMuted)
			})
			ui.Text(c, info.title).FontSize(m.FontLG).FontWeight(600).TextColor(tk.TextPrimary).SingleLine()
			ui.Text(c, info.desc).FontSize(m.FontSM).TextColor(tk.TextSecondary)
		})
	})
}

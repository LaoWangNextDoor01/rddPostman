package tools

import (
	"strings"

	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"

	"github.com/egoist/mygo/ui"
)

// SidebarView 在 WorkbenchLayout 左侧栏内绘制开发者小工具分组树（完全对齐 Pixso 标准侧栏：
// 顶部 Search 过滤条 + 可折叠文件夹分组 + 26px 紧凑中性灰选中态）。
func SidebarView(c *ui.Context, activeToolID string, openTool func(toolID string)) {
	m, tk := thV2.Metrics, thV2.Of(c)

	col := ui.Column(c).Fill().Background(tk.BgApp)
	search := ui.Local(col, "tools-sb-search", func() string { return "" })
	selectedID := ui.Local(col, "tools-sb-selected", func() string { return CanonicalID(activeToolID) })
	collapsed := ui.Local(col, "tools-sb-collapsed", func() map[string]bool { return map[string]bool{} })
	moreOpen := ui.Local(col, "tools-sb-more-open", func() bool { return false })

	if activeToolID != "" {
		*selectedID = CanonicalID(activeToolID)
	}

	kw := strings.ToLower(strings.TrimSpace(*search))

	col.Children(func() {
		// 1. 顶部搜索行（对齐 collection/sidebar.go：34px 高、list-filter 图标、Search 占位、更多操作钮）
		ui.Row(c).FillWidth().Height(34).Padding(0, m.Space3).Gap(6).
			AlignItems(ui.Center).Children(func() {
			searchBox := ui.Row(c.Key("tools-sb-search-box")).Grow(1).Height(26).
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
				ui.TextInputBase(c.Key("tools-sb-search-input"), search).Grow(1).
					FontSize(m.FontSM).TextColor(tk.TextPrimary).
					Placeholder("Search").PlaceholderColor(tk.TextMuted).
					Label("Filter dev tools")
				if *search != "" {
					clr := v2.IconButton(c, v2.IconButtonOpts{
						Key:      "tools-sb-search-clear",
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

			moreBtn := v2.IconButton(c, v2.IconButtonOpts{
				Key:      "tools-sb-more",
				Label:    "More tool actions",
				Tooltip:  "更多操作",
				Icon:     icons.Ellipsis,
				Size:     24,
				IconSize: 14,
				Radius:   m.RadiusSM,
			})
			if moreBtn.Clicked() {
				*moreOpen = !*moreOpen
			}
			v2.Popover(c, moreBtn, moreOpen, v2.PopoverOpts{Width: 168}, func(panel ui.Element) {
				ui.Column(c).Padding(4).Gap(2).Children(func() {
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Layers, Key: "tools-sb-expand-all", Label: "展开全部分组",
					}).Clicked() {
						for k := range *collapsed {
							delete(*collapsed, k)
						}
						*moreOpen = false
					}
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Folder, Key: "tools-sb-collapse-all", Label: "折叠全部分组",
					}).Clicked() {
						for _, def := range registry {
							(*collapsed)[def.Category] = true
						}
						*moreOpen = false
					}
				})
			})
		})

		// 2. 分组小工具树
		type group struct {
			name  string
			items []ToolDef
		}
		var groups []group
		groupIdx := map[string]int{}
		for _, def := range registry {
			if kw != "" {
				hay := strings.ToLower(def.Name + " " + def.ShortName + " " + def.Description + " " + def.Keywords + " " + def.Category)
				if !strings.Contains(hay, kw) {
					continue
				}
			}
			idx, ok := groupIdx[def.Category]
			if !ok {
				idx = len(groups)
				groupIdx[def.Category] = idx
				groups = append(groups, group{name: def.Category})
			}
			groups[idx].items = append(groups[idx].items, def)
		}

		if len(groups) == 0 {
			ui.Column(c).Grow(1).FillWidth().Center().Padding(m.Space4).Gap(m.Space1).Children(func() {
				ui.Text(c, "未找到匹配的小工具").FontSize(m.FontXS).TextColor(tk.TextMuted)
			})
			return
		}

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(6, m.Space2, m.Space4, m.Space2).Gap(6).Children(func() {
				for _, g := range groups {
					grp := g
					isOpen := kw != "" || !(*collapsed)[grp.name]
					ui.Column(c).FillWidth().Gap(2).Children(func() {
						// 文件夹样式分组行（26px 高、⌄ 箭头 + 文件夹图标 + 分组名）
						grpBtn := ui.ButtonBase(c.Key("tools-grp-"+grp.name)).
							FillWidth().Height(m.RowHeight).Padding(0, 8).Gap(5).
							Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
							Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(grp.name).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						if grpBtn.Hovered() {
							grpBtn.Background(tk.BgSubtle)
						}
						if grpBtn.Clicked() {
							(*collapsed)[grp.name] = isOpen
						}
						grpBtn.Children(func() {
							arrow := ui.Icon(c, icons.ChevronDown).FontSize(11).TextColor(tk.TextMuted)
							if !isOpen {
								arrow.Rotate(-90)
							}
							ui.Icon(c, icons.Folder).FontSize(13).TextColor(tk.TextSecondary)
							ui.Text(c, grp.name).FontSize(m.FontSM).FontWeight(400).
								TextColor(tk.TextPrimary).SingleLine().Grow(1)
						})

						if !isOpen {
							return
						}

						for _, it := range grp.items {
							item := it
							isSel := *selectedID == item.ID
							row := ui.ButtonBase(c.Key("wb-tool-item-"+item.ID)).
								FillWidth().Height(m.RowHeight).Padding(0, 8, 0, 28).Gap(6).
								Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(item.Name).
								Description(item.Description).
								Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
							switch {
							case row.Pressed(), isSel:
								row.Background(tk.BgSelected)
							case row.Hovered():
								row.Background(tk.BgHover)
							}
							if row.Clicked() {
								*selectedID = item.ID
								if openTool != nil {
									openTool(item.ID)
								}
							}
							row.Children(func() {
								icColor := tk.TextSecondary
								weight := 400
								if isSel {
									icColor = tk.TextPrimary
									weight = 500
								}
								ui.Icon(c, item.Icon).FontSize(13).TextColor(icColor)
								ui.Text(c, item.Name).FontSize(m.FontSM).FontWeight(weight).
									TextColor(tk.TextPrimary).SingleLine().Grow(1)
							})
							if item.Description != "" {
								ui.TooltipBase(c, row, func(tip ui.Element) {
									tip.AttachTo(row, ui.AnchorRight, ui.AnchorLeft).
										MarginStart(8).
										Padding(6, 10).
										MaxWidth(360).
										Radius(m.RadiusMD).
										Background(tk.BgSurface).
										Border(m.Border, tk.BorderPopover).
										Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.06)).
										Shadow(0, 6, 16, 0, ui.RGBA(0, 0, 0, 0.10))
									tip.Children(func() {
										ui.Text(c, item.Description).
											FontSize(m.FontXS).
											TextColor(tk.TextSecondary)
									})
								})
							}
						}
					})
				}
			})
		})
	})
}

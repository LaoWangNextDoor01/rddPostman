package collection

import (
	"strings"

	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func sidebar(c *ui.Context, props pageProps) {
	m, tk := thV2.Metrics, thV2.Of(c)
	col := ui.Column(c).Fill().Background(tk.BgApp)
	search := ui.Local(col, "search", func() string { return "" })
	filterOpen := ui.Local(col, "sb-filter-open", func() bool { return false })
	moreOpen := ui.Local(col, "sb-more-open", func() bool { return false })

	col.Children(func() {
		// 1. 搜索（内嵌左侧筛选按钮下拉，对齐 Pixso 22-侧栏-搜索筛选）+ 新建「+」+ 更多「...」
		ui.Row(c).FillWidth().Height(34).Padding(0, m.Space3).Gap(6).
			AlignItems(ui.Center).Children(func() {
			searchBox := ui.Row(c.Key("sb-search-box")).Grow(1).Height(26).Padding(0, 10, 0, 6).Gap(6).
				AlignItems(ui.Center).Radius(6).Background(tk.BgSubtle)
			searchBox.Children(func() {
				// 左侧筛选图标按钮（对齐 Pixso 22-侧栏-搜索筛选：89:9987 / 89:9988，三层阶梯漏斗 ListFilter）
				filterActive := *filterOpen || !props.state.allFiltersEnabled()
				filterBtn := v2.IconButton(c, v2.IconButtonOpts{
					Key:          "sb-filter-btn",
					Label:        "Filter sidebar",
					Tooltip:      "Filter",
					Icon:         icons.ListFilter,
					Size:         18,
					IconSize:     14,
					Radius:       4,
					Color:        tk.TextMuted,
					ActiveAccent: filterActive,
				})
				if filterBtn.Clicked() {
					*filterOpen = !*filterOpen
				}

				v2.Popover(c, filterBtn, filterOpen, v2.PopoverOpts{Width: 156}, func(panel ui.Element) {
					ui.Column(c).Padding(4, 0).Gap(0).Children(func() {
						allOn := props.state.allFiltersEnabled()
						sidebarFilterCheckboxRow(c, "sb-flt-all", "Select all", allOn, true, func() {
							props.state.setAllFilters(!allOn)
						})
						for _, cat := range sidebarFilterCategories {
							catName := cat
							checked := props.state.filterEnabled(catName)
							sidebarFilterCheckboxRow(c, "sb-flt-"+catName, catName, checked, false, func() {
								if props.state.sidebarFilter == nil {
									props.state.setAllFilters(true)
								}
								props.state.sidebarFilter[catName] = !checked
							})
						}
					})
				})

				ui.TextInputBase(c.Key("sb-search-input"), search).Grow(1).
					FontSize(12).TextColor(tk.TextPrimary).
					Placeholder("Search").PlaceholderColor(tk.TextMuted)
			})

			addMenuButton(c, props)

			moreBtn := v2.IconButton(c, v2.IconButtonOpts{
				Key:      "sb-more",
				Label:    "more",
				Tooltip:  "More actions",
				Icon:     icons.Ellipsis,
				Size:     26,
				IconSize: 14,
				Radius:   4,
			})
			if moreBtn.Clicked() {
				*moreOpen = !*moreOpen
			}
			v2.Popover(c, moreBtn, moreOpen, v2.PopoverOpts{Width: 184}, func(panel ui.Element) {
				ui.Column(c).Padding(4).Gap(2).Children(func() {
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Layers, Key: "sb-more-expand", Label: "Expand all",
					}).Clicked() {
						for _, coll := range props.collections {
							*props.state.openFlag(string(coll.ID)) = true
						}
						*moreOpen = false
					}
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Folder, Key: "sb-more-collapse", Label: "Collapse all",
					}).Clicked() {
						for _, coll := range props.collections {
							*props.state.openFlag(string(coll.ID)) = false
						}
						*moreOpen = false
					}
					v2.PopoverDivider(c)
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Code, Key: "sb-more-curl", Label: "Import cURL",
					}).Clicked() {
						props.state.importCurlOpen = true
						props.state.importCurlText = ""
						props.state.importCurlErr = ""
						*moreOpen = false
					}
					if v2.MenuButton(c, v2.MenuButtonProps{
						Icon: icons.Upload, Key: "sb-more-postman", Label: "Import Postman JSON",
					}).Clicked() {
						props.state.importPostmanOpen = true
						props.state.importPostmanText = ""
						props.state.importPostmanErr = ""
						*moreOpen = false
					}
				})
			})
		})

		ui.Box(c).Height(8)

		// 2. 可滚动分区：按筛选开关动态显示 COLLECTIONS → ENVIRONMENTS → 工具区
		actions := treeActions{
			create: func(row *treeNode, kind treeKind) {
				domainKind := domain.KindFolder
				if kind == treeRequest {
					domainKind = domain.KindRequest
				}
				props.actions.createNode(domain.ID(row.id), domainKind)
			},
			remove: func(row *treeNode) {
				props.actions.deleteRow(row)
			},
			open: func(row *treeNode) {
				props.actions.openTab(row.id)
			},
			copy: func(text string) {
				c.Services().WriteClipboard(text)
			},
			duplicate: func(row *treeNode) {
				if props.actions.duplicateNode != nil {
					props.actions.duplicateNode(row.id)
				}
			},
		}

		ui.Scroll(c).Grow(1).FillWidth().Children(func() {
			ui.Column(c).FillWidth().Padding(0, 8, 16, 8).Children(func() {
				// 树形结构最顶层直接就是 Collection 级别
				if len(props.collections) == 0 {
					emptySidebar(c, props)
				} else {
					nodes := filterTree(buildTree(props.collections), *search, props.state)
					if len(nodes) == 0 && *search != "" {
						ui.Box(c).Padding(m.Space3).Center().Children(func() {
							ui.Text(c, "No matching requests").FontSize(m.FontXS).TextColor(tk.TextMuted)
						})
					} else {
						collectionTree(c, props.state, nodes, actions)
					}

					// 底部行内 + New Collection：直接新建集合并以 Tab 打开编辑（不弹框）
					newCollRow := ui.ButtonBase(c.Key("sb-inline-new-coll")).
						FillWidth().Height(26).Padding(0, 12, 0, 24).Gap(6).
						Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Start).
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("New Collection")
					switch {
					case newCollRow.Pressed():
						newCollRow.Background(tk.BgSelected)
					case newCollRow.Hovered():
						newCollRow.Background(tk.BgHover)
					}
					if newCollRow.Clicked() && props.actions.createCollectionTab != nil {
						props.actions.createCollectionTab()
					}
					newCollRow.Children(func() {
						ui.Icon(c, icons.Plus).FontSize(11).TextColor(tk.TextMuted)
						ui.Text(c, "New Collection").FontSize(m.FontXS).TextColor(tk.TextSecondary).SingleLine()
					})
				}
			})
		})
	})
}

func sidebarFilterCheckboxRow(c *ui.Context, key, label string, checked, isTitle bool, onToggle func()) {
	tk := thV2.Of(c)
	row := ui.ButtonBase(c.Key(key)).
		FillWidth().Height(28).Padding(0, 10).Gap(6).
		Radius(4).AlignItems(ui.Center).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleCheckBox).Label(label)
	switch {
	case row.Pressed():
		row.Background(tk.BgSelected)
	case row.Hovered():
		row.Background(tk.BgHover)
	}
	if row.Clicked() && onToggle != nil {
		onToggle()
	}
	row.Children(func() {
		box := ui.Box(c).Size(14, 14).Radius(3).Center()
		if checked {
			box.Background(tk.Accent).Children(func() {
				ui.Icon(c, icons.Check).FontSize(11).TextColor(tk.OnAccent)
			})
		} else {
			box.Background(tk.BgBase).Border(1, tk.BorderStrong)
		}
		weight := 400
		if isTitle {
			weight = 500
		}
		ui.Text(c, label).FontSize(12).FontWeight(weight).TextColor(tk.TextPrimary).SingleLine()
	})
}

func sidebarSectionHeader(c *ui.Context, key, title string, open bool, onClick func()) {
	tk := thV2.Of(c)
	btn := ui.ButtonBase(c.Key(key)).
		FillWidth().Height(20).Padding(0, 4).Gap(4).
		Radius(4).AlignItems(ui.Center).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(title)
	if btn.Hovered() {
		btn.Background(tk.BgHover)
	}
	if btn.Clicked() && onClick != nil {
		onClick()
	}
	btn.Children(func() {
		arrow := ui.Icon(c, icons.ChevronDown).Size(10, 10).TextColor(tk.TextMuted)
		if !open {
			arrow.Rotate(-90)
		}
		ui.Text(c, title).FontSize(10).FontWeight(600).LetterSpacing(0.4).
			TextColor(tk.TextMuted).SingleLine()
	})
}

// importCurlDialog 弹出从 cURL 导入请求的对话框。
func importCurlDialog(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	state := props.state

	ui.DialogBase(c, &state.importCurlOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(520).Padding(m.Space4).Gap(m.Space3)
		theme.DialogPanel(c, panel)
		panel.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, icons.Get("code")).FontSize(m.IconMD).TextColor(tk.Accent)
				ui.Text(c, "从 cURL 导入请求").FontSize(m.FontMD).Bold().
					TextColor(tk.TextPrimary)
			})
			ui.Text(c, "输入或粘贴标准的 cURL 命令，自动解析请求方法、URL、标头、认证与请求体并创建请求。").
				FontSize(m.FontXS).TextColor(tk.TextMuted)

			ui.TextArea(c, &state.importCurlText).FillWidth().MinHeight(120).
				MaxHeight(240).Padding(m.Space2, m.Space2).Radius(m.RadiusSM).
				FontSize(m.FontSM).FixedLineHeight(20).
				Placeholder("curl -X POST https://api.example.com/login -H \"Content-Type: application/json\" -d '{\"user\":\"admin\"}'")

			if state.importCurlErr != "" {
				ui.Text(c, state.importCurlErr).FontSize(m.FontXS).TextColor(tk.Danger)
			}

			submit := func() {
				trimmed := strings.TrimSpace(state.importCurlText)
				if trimmed == "" {
					state.importCurlErr = "请输入有效的 cURL 命令"
					return
				}
				targetID := domain.ID(state.selected)
				if err := props.actions.importCurl(targetID, trimmed); err != nil {
					state.importCurlErr = "解析 cURL 失败: " + err.Error()
					return
				}
				state.importCurlOpen = false
				state.importCurlErr = ""
			}

			ui.Row(c).Gap(m.Space2).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					state.importCurlOpen = false
				}
				if ui.PrimaryButton(c, "导入请求").Clicked() {
					submit()
				}
			})
		})
	})
}

// importPostmanDialog 弹出从 Postman 集合导入的对话框。
func importPostmanDialog(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	state := props.state

	ui.DialogBase(c, &state.importPostmanOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(560).Padding(m.Space4).Gap(m.Space3)
		theme.DialogPanel(c, panel)
		panel.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, icons.Get("file")).FontSize(m.IconMD).TextColor(tk.Accent)
				ui.Text(c, "导入 Postman 集合 (v2.1)").FontSize(m.FontMD).Bold().
					TextColor(tk.TextPrimary)
			})
			ui.Text(c, "粘贴 Postman Collection v2.1 格式的 JSON 文本，将完整导入集合、目录树、请求、环境变量与前后置脚本。").
				FontSize(m.FontXS).TextColor(tk.TextMuted)

			ui.TextArea(c, &state.importPostmanText).FillWidth().MinHeight(150).
				MaxHeight(280).Padding(m.Space2, m.Space2).Radius(m.RadiusSM).
				FontSize(m.FontSM).FixedLineHeight(20).
				Placeholder("{\n  \"info\": {\n    \"name\": \"My Postman Collection\",\n    \"schema\": \"https://schema.getpostman.com/json/collection/v2.1.0/collection.json\"\n  },\n  \"item\": [...]\n}")

			if state.importPostmanErr != "" {
				ui.Text(c, state.importPostmanErr).FontSize(m.FontXS).TextColor(tk.Danger)
			}

			submit := func() {
				trimmed := strings.TrimSpace(state.importPostmanText)
				if trimmed == "" {
					state.importPostmanErr = "请输入有效的 Postman 集合 JSON"
					return
				}
				if err := props.actions.importCollection(trimmed); err != nil {
					state.importPostmanErr = "导入失败: " + err.Error()
					return
				}
				state.importPostmanOpen = false
				state.importPostmanErr = ""
			}

			ui.Row(c).Gap(m.Space2).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					state.importPostmanOpen = false
				}
				if ui.PrimaryButton(c, "导入集合").Clicked() {
					submit()
				}
			})
		})
	})
}

// emptySidebar 对齐 Pixso 2-侧栏-集合为空：圆形弱底图标 + 标题说明 + 科技蓝主按钮。
func emptySidebar(c *ui.Context, props pageProps) {
	m, tk := thV2.Metrics, thV2.Of(c)
	ui.Column(c).FillWidth().Padding(24, 12, 16, 12).AlignItems(ui.Center).Gap(10).Children(func() {
		ui.Box(c).Size(44, 44).Radius(22).Background(tk.BgSubtle).Center().Children(func() {
			ui.Icon(c, icons.Layers).FontSize(20).TextColor(tk.TextMuted)
		})
		ui.Text(c, "You don't have any collections").FontSize(m.FontSM).FontWeight(600).
			TextColor(tk.TextPrimary).TextAlign(ui.Center)
		ui.Text(c, "Collections let you group related requests, making them easier to access and run.").
			FontSize(m.FontXS).TextColor(tk.TextMuted).TextAlign(ui.Center)

		btn := ui.ButtonBase(c.Key("empty-new-collection")).Height(28).
			PaddingX(14).AlignItems(ui.Center).Justify(ui.Center).
			Radius(m.RadiusSM).Background(tk.Accent).
			Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Create a collection").
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
		switch {
		case btn.Pressed():
			btn.Background(tk.AccentPressed)
		case btn.Hovered():
			btn.Background(tk.AccentHover)
		}
		if btn.Clicked() && props.actions.createCollectionTab != nil {
			props.actions.createCollectionTab()
		}
		btn.Children(func() {
			ui.Text(c, "Create a collection").FontSize(m.FontXS).FontWeight(500).
				TextColor(tk.OnAccent).SingleLine()
		})
	})
}

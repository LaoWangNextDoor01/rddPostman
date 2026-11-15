// Package collection 实现集合工作区：左侧集合树，右侧请求 tab。
package collection

import (
	"strconv"
	"strings"

	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/nav"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制集合工作区。
func View(c *ui.Context, n nav.Navigator) {
	m := theme.Metrics
	root := ui.Box(c).Grow(1)
	ws := *ui.Local(root, "workspace", newWorkspace)
	width := ui.Local(root, "sidebar-width", func() float32 {
		return m.SidebarWidth
	})
	root.Children(func() {
		ui.Split(c, width, func() {
			sidebar(c, ws)
		}, func() {
			main(c, ws)
		}).Fill().SplitReset(m.SidebarWidth)
	})
}

func sidebar(c *ui.Context, ws *workspace) {
	m := theme.Metrics
	tk := theme.Of(c)
	col := ui.Column(c).Grow(1).Background(tk.BgSurface)
	search := ui.Local(col, "search", func() string { return "" })
	col.Children(func() {
		// 顶部搜索 + 添加/筛选。
		ui.Row(c).Padding(m.Space2, m.Space2).Gap(m.Space1).
			AlignItems(ui.Center).Children(func() {
			ui.TextInput(c, search).Grow(1).Height(m.SizeMD).
				Padding(0, m.Space2).Placeholder(locales.SearchRequests())
			// 添加按钮
			addBtn := v1.MiniButton(c, v1.RdButtonOptions{
				Key:   "add",
				Label: "添加",
				Icon:  icons.Get("plus"),
			})

			if addBtn.Clicked() {
				ws.addOpen = !ws.addOpen
			}
			ui.PopoverBase(c, addBtn, &ws.addOpen, func(panel ui.Element) {
				panel.Margin(m.Space1, 0, 0, 0).Padding(m.Space1).Width(160).
					Radius(m.RadiusMD).Background(tk.BgSurface).
					Border(m.Border, tk.Border)
				create := func(kind nodeKind, label string) {
					if ui.Button(c, label).FillWidth().Clicked() {
						ws.create(kind)
						ws.addOpen = false
					}
				}
				create(kindCollection, locales.NewCollection())
				create(kindFolder, locales.NewFolder())
				create(kindRequest, locales.NewRequest())
			})

			// 切换环境
			v1.MiniButton(c, v1.RdButtonOptions{
				Key:   "env",
				Label: "环境切换",
				Icon:  icons.Get("globe"),
			})

			// more
			v1.MiniButton(c, v1.RdButtonOptions{
				Key:   "more",
				Label: "more",
				Icon:  icons.Get("ellipsis"),
			})
		})

		// 集合标题。
		ui.Row(c).Padding(m.Space1, m.Space2).Gap(m.Space1).
			AlignItems(ui.Center).Children(func() {

			ui.Text(c, locales.CollectionsTitle()).FontSize(m.FontMD).Bold().TextColor(tk.TextSecondary).Grow(1)

			v1.MiniButton(c, v1.RdButtonOptions{
				Key:   "add-folder",
				Label: "add folder",
				Icon:  icons.Get("folder-plus"),
			})

			v1.MiniButton(c, v1.RdButtonOptions{
				Key:   "more",
				Label: "more",
				Icon:  icons.Get("ellipsis"),
			})
		})

		ui.Scroll(c).Grow(1).PaddingX(m.Space1).Children(func() {
			collectionTree(c, ws, func(id string) { ws.openTab(id) })
		})

		ui.Divider(c)

		// 历史。
		ui.Row(c).Padding(m.Space1, m.Space2).AlignItems(ui.Center).Children(func() {
			ui.Text(c, locales.HistoryToday()).FontSize(10).TextColor(tk.TextMuted).Grow(1)
			ui.Text(c, locales.Clear()).FontSize(10).TextColor(tk.Accent)
		})
		for _, it := range [][3]string{
			{"GET", "/api/v1/users/4821/profile", "200"},
			{"POST", "/api/v1/orders", "201"},
			{"PUT", "/api/v1/inventory/8823", "200"},
		} {
			ui.Row(c).Padding(0, m.Space2).Gap(m.Space1).Height(22).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, it[0]).FontSize(10).Width(38).TextColor(methodColor(tk, it[0]))
				ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).
					Grow(1).SingleLine()
				ui.Text(c, it[2]).FontSize(10).TextColor(tk.Success)
			})
		}
	})
}

// 顶部搜索区域

func main(c *ui.Context, ws *workspace) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Background(tk.BgSurface).Children(func() {
		// 顶部请求 tab，Segment 风格：白底 36 高、内边距 4、间距 3，
		// 选中项 28 高、左右 12 内边距、圆角 6，只用底色不加描边，避免文字位移。
		ui.Row(c).Height(36).Padding(m.Space1).Gap(3).
			AlignItems(ui.End).Background(tk.BgSurface).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
			Children(func() {
				for _, id := range ws.tabs {
					tabNode := ws.find(id)
					if tabNode == nil {
						continue
					}
					tab := ui.ButtonBase(c.Key("tab-" + id)).Height(m.SizeLG).
						PaddingX(m.Space3).Gap(m.Space2).Radius(m.RadiusMD).
						AlignItems(ui.Center).
						Label(tabNode.Name).Role(ui.RoleTab)
					switch {
					case ws.active == id:
						tab.Background(tk.BgElevated).TextColor(tk.TextPrimary)
					case tab.Hovered():
						tab.Background(tk.HoverSurface).TextColor(tk.TextPrimary)
					default:
						tab.TextColor(tk.TextSecondary)
					}
					if tab.Clicked() {
						ws.active, ws.selected = id, id
					}
					tab.Children(func() {
						if tabNode.Kind == kindRequest {
							ui.Text(c, tabNode.Method).FontSize(m.FontXS).
								LetterSpacing(0.3).Bold().
								TextColor(methodColor(tk, tabNode.Method))
						}
						name := ui.Text(c, tabNode.Name).FontSize(m.FontSM).
							SingleLine()
						if ws.active == id {
							name.FontWeight(600)
						}
						if tabNode.dirty {
							// 有未保存修改时，用橙色圆点代替关闭按钮。
							ui.Box(c).Size(5, 5).Radius(m.RadiusFull).
								Background(tk.Accent)
						} else {
							ui.Icon(c, icons.Get("x")).FontSize(m.FontXS).
								TextColor(tk.TextDisabled)
						}
					})
				}
				// 新建 tab：14×14 浅灰圆角底 + 加号。
				ui.Box(c).Width(m.Space3)
				ui.Box(c).Size(m.IconMD, m.IconMD).Radius(m.RadiusSM).
					Center().Background(tk.BgElevated).Children(func() {
					ui.Icon(c, icons.Get("plus")).FontSize(m.IconMD).
						TextColor(tk.TextMuted)
				})
				ui.Spacer(c)
				// 设计稿右侧固定灰 #71717A，没有对应的主题变量。
				ui.Row(c).Gap(m.Space2).Margin(0, m.Space3, 0, 0).
					AlignItems(ui.Center).Children(func() {
					for _, ic := range []string{"square-terminal", "ellipsis"} {
						ui.Icon(c, icons.Get(ic)).FontSize(m.IconMD).
							TextColor(ui.Hex("#71717A"))
					}
				})
			})

		active := ws.find(ws.active)
		if active == nil {
			v1.EmptyState(c, locales.SelectItem())
			return
		}
		if active.Kind == kindRequest {
			requestTab(c, active)
		} else {
			infoTab(c, active)
		}
	})
}

func infoTab(c *ui.Context, n *node) {
	ui.Scroll(c).Grow(1).Children(func() {
		v1.PageHeader(c, n.Name, "")
		v1.EmptyState(c, locales.SelectItem())
	})
}

func requestTab(c *ui.Context, n *node) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Background(tk.BgSurface).Children(func() {
		// 请求编辑区。
		ui.Column(c).Padding(m.Space3, m.Space4).Gap(m.Space2).Children(func() {
			ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Get("folder")).FontSize(10).TextColor(tk.TextMuted)
				ui.Text(c, "开发者平台 API / Users / 用户资料").FontSize(10).
					TextColor(tk.TextMuted).Grow(1)
				ui.Text(c, "已保存 · 林晓").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Row(c).Height(40).Gap(m.Space2).AlignItems(ui.Center).Children(func() {
				ui.Row(c).Height(m.SizeXL).PaddingX(m.Space2).Gap(m.Space1).
					AlignItems(ui.Center).Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Text(c, n.Method).FontSize(m.FontXS).Bold().
							TextColor(methodColor(tk, n.Method))
						ui.Icon(c, icons.Get("chevron-down")).FontSize(10).
							TextColor(tk.TextMuted)
					})
				ui.Row(c).Grow(1).Height(m.SizeXL).PaddingX(m.Space2).
					AlignItems(ui.Center).Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Text(c, n.URL).FontSize(m.FontSM).
							TextColor(tk.TextSecondary).SingleLine()
					})
				ui.PrimaryButton(c, locales.Send())
				for _, ic := range []string{"ellipsis", "code"} {
					ui.Row(c).Width(m.SizeXL).Height(m.SizeXL).Justify(ui.Center).
						AlignItems(ui.Center).Radius(m.RadiusMD).
						Background(tk.BgSurface).Border(m.Border, tk.Border).
						Children(func() {
							ui.Icon(c, icons.Get(ic)).FontSize(m.IconXS).
								TextColor(tk.TextMuted)
						})
				}
			})

			tabsRow(c, []string{
				"Params 2", "Authorization", "Headers 6", "Body",
				"Scripts", "Tests", "Settings",
			}, 0, func() {
				ui.Icon(c, icons.Get("code")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
				ui.Text(c, "代码片段").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Divider(c)

			ui.Row(c).Gap(m.Space2).Height(170).Children(func() {
				ui.Column(c).Grow(1).Clip().Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						ui.Row(c).Height(26).Padding(m.Space1, m.Space2).Gap(m.Space2).
							AlignItems(ui.Center).
							BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
							Children(func() {
								ui.Text(c, "Key").FontSize(10).TextColor(tk.TextMuted).Width(160)
								ui.Text(c, "Value").FontSize(10).TextColor(tk.TextMuted).Grow(1)
								ui.Text(c, "Description").FontSize(10).TextColor(tk.TextMuted).Width(120)
							})
						for _, it := range [][3]string{
							{"include", "profile.quota", "返回字段范围"},
							{"expand", "tenant", "展开关联对象"},
						} {
							ui.Row(c).Height(32).Padding(m.Space1, m.Space2).Gap(m.Space2).
								AlignItems(ui.Center).Children(func() {
								ui.Box(c).Size(12, 12).Radius(m.RadiusSM).
									Border(m.Border, tk.Border)
								ui.Text(c, it[0]).FontSize(m.FontSM).TextColor(tk.TextPrimary).Width(140)
								ui.Text(c, it[1]).FontSize(m.FontSM).TextColor(tk.Info).Grow(1)
								ui.Text(c, it[2]).FontSize(10).TextColor(tk.TextMuted).Width(120)
							})
						}
					})

				ui.Column(c).Width(220).Padding(m.Space2).Gap(m.Space1).
					Radius(m.RadiusMD).Background(tk.BgSurface).
					Border(m.Border, tk.Border).Children(func() {
					ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.Get("triangle-alert")).FontSize(10).
							TextColor(tk.Danger)
						ui.Text(c, "解析问题").FontSize(10).Bold().TextColor(tk.Danger)
						ui.Spacer(c)
						ui.Text(c, "Sandbox").FontSize(10).TextColor(tk.TextMuted)
					})
					for _, it := range [][2]string{
						{"baseUrl", "api.sandbox.devhub.io"},
						{"userId", "4821"},
						{"accessToken", "eyJhbGciOiJIUzI1..."},
					} {
						ui.Row(c).Gap(m.Space1).Children(func() {
							ui.Text(c, it[0]).FontSize(10).TextColor(tk.Accent).Width(78)
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).
								Grow(1).SingleLine()
						})
					}
				})
			})
		})

		// 响应区。
		ui.Column(c).Grow(1).BorderWidth(m.Border, 0, 0, 0).
			BorderColor(tk.Border).Children(func() {
			ui.Row(c).Height(42).PaddingX(m.Space4).Gap(m.Space2).
				AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.Response()).FontSize(m.FontSM).Bold().
					TextColor(tk.TextPrimary)
				ui.Text(c, "200 OK").FontSize(10).TextColor(tk.Success).
					Padding(1, m.Space1).Radius(m.RadiusSM).
					Background(tk.Success.Alpha(0.12))
				ui.Text(c, "182 ms · 2.41 KB · TTFE 96 ms").FontSize(10).
					TextColor(tk.TextMuted)
				ui.Spacer(c)
				ui.Text(c, locales.TestsPassed()).FontSize(10).TextColor(tk.Success)
				ui.Icon(c, icons.Get("ellipsis")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
			})

			tabsRow(c, []string{
				"Body", "Cookies 2", "Headers 11", "Test Results 3", "Visualize",
			}, 0, func() {
				ui.Text(c, "Preview  ›  JSON").FontSize(10).TextColor(tk.TextMuted)
			})

			ui.Row(c).Grow(1).Padding(m.Space3, m.Space4).Gap(m.Space2).
				AlignItems(ui.Stretch).Children(func() {
				ui.Row(c).Grow(1).AlignItems(ui.Start).Clip().Radius(m.RadiusMD).
					Background(tk.BgSurface).Border(m.Border, tk.Border).
					Children(func() {
						lines := []string{
							"{",
							`  "id": "usr_4821",`,
							`  "username": "lin.wei",`,
							`  "displayName": "林薇",`,
							`  "email": "lin.wei@devhub.io",`,
							`  "role": "backend_engineer",`,
							`  "tenantId": "acme-apac-02",`,
							`  "verified": true,`,
							`  "createdAt": "2026-03-18T09:24:11Z",`,
							`  "scopes": ["users:read", "orders:write"],`,
							`  "quota": {`,
							`    "dailyLimit": 20000,`,
							`    "used": 8342`,
							`  },`,
							`  "resultsAs": "json",`,
							`  "lastActiveIp": "203.0.113.42"`,
							"}",
						}
						ui.Column(c).Width(28).Padding(m.Space2, m.Space1).
							Gap(1).Children(func() {
							for i := range lines {
								ui.Text(c, strconv.Itoa(i+1)).FontSize(10).
									TextColor(tk.TextMuted).TextAlign(ui.End)
							}
						})
						ui.Column(c).Grow(1).Padding(m.Space2, m.Space2).
							Gap(1).Children(func() {
							for _, line := range lines {
								jsonLine(c, tk, line)
							}
						})
					})

				ui.Column(c).Width(200).Padding(m.Space2).Gap(m.Space1).
					Radius(m.RadiusMD).Background(tk.BgSurface).
					Border(m.Border, tk.Border).Children(func() {
					ui.Text(c, locales.ResponseSummary()).FontSize(10).Bold().
						TextColor(tk.TextPrimary)
					for _, it := range [][2]string{
						{"Status", "200 OK"},
						{"Time", "182 ms"},
						{"Size", "2.41 KB"},
						{"Content-Type", "application/json"},
						{"Server", "devhub-edge/2.14"},
						{"X-Request-Id", "req_9f2c81b4"},
					} {
						ui.Row(c).Gap(m.Space1).Children(func() {
							ui.Text(c, it[0]).FontSize(10).TextColor(tk.TextMuted).Width(70)
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).
								Grow(1).SingleLine()
						})
					}
					ui.Divider(c)
					ui.Text(c, locales.SavedExamples()).FontSize(10).Bold().
						TextColor(tk.TextPrimary)
					for _, it := range [][3]string{
						{"GET", "完整资料", "12 分钟前"},
						{"GET", "字段精简", "1 小时前"},
						{"GET", "用户不存在", "昨天"},
					} {
						ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
							ui.Text(c, it[0]).FontSize(10).Width(32).
								TextColor(methodColor(tk, it[0]))
							ui.Text(c, it[1]).FontSize(10).TextColor(tk.TextSecondary).Grow(1)
							ui.Text(c, it[2]).FontSize(10).TextColor(tk.TextMuted)
							ui.Text(c, "200").FontSize(10).TextColor(tk.TextMuted)
						})
					}
				})
			})
		})
	})
}

// tabsRow 画设计稿里的下划线 tab：等高、文字居中、底部 2px 指示条。
func tabsRow(c *ui.Context, labels []string, active int, trailing func()) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Row(c).Height(34).PaddingX(m.Space4).Gap(m.Space6).
		BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
		Children(func() {
			for i, label := range labels {
				ui.Column(c).Height(34).Children(func() {
					ui.Box(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).
						Children(func() {
							txt := ui.Text(c, label).FontSize(m.FontSM)
							if i == active {
								txt.TextColor(tk.Accent).Bold()
							} else {
								txt.TextColor(tk.TextMuted)
							}
						})
					bar := ui.Box(c).Height(2)
					if i == active {
						bar.Background(tk.Accent)
					}
				})
			}
			if trailing != nil {
				ui.Spacer(c)
				trailing()
			}
		})
}

// jsonLine 把一行 JSON 拆成键和值分别着色。
func jsonLine(c *ui.Context, tk theme.Tokens, line string) {
	t := strings.TrimSpace(line)
	if t == "{" || t == "}" || t == "}," || strings.HasSuffix(t, "{") {
		ui.Text(c, line).FontSize(11).Font("monospace").
			TextColor(tk.TextMuted).SingleLine()
		return
	}
	if i := strings.Index(line, `": `); i >= 0 {
		key, val := line[:i+1]+`": `, line[i+3:]
		color := tk.Success
		if val == "{" || val == "}" || val == "}," {
			color = tk.TextMuted
		} else if val != "" && (val[0] >= '0' && val[0] <= '9' ||
			strings.HasPrefix(val, "true") || strings.HasPrefix(val, "false")) {
			color = tk.Warning
		}
		ui.Row(c).Children(func() {
			ui.Text(c, key).FontSize(11).Font("monospace").TextColor(tk.Info)
			ui.Text(c, val).FontSize(11).Font("monospace").TextColor(color)
		})
		return
	}
	ui.Text(c, line).FontSize(11).Font("monospace").TextColor(tk.TextSecondary)
}

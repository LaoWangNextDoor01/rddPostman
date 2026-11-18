package collection

import (
	"strings"

	"rddpostman/components/theme"
	v1 "rddpostman/components/v1"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func sidebar(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	col := ui.Column(c).Grow(1).Background(tk.BgSurface)
	search := ui.Local(col, "search", func() string { return "" })
	col.Children(func() {
		// 顶部工具行：搜索、新建、环境、更多。
		ui.Row(c).Padding(m.Space1, m.Space2).Gap(6).
			AlignItems(ui.Center).Children(func() {
			ui.Row(c).Fill().Height(28).PaddingX(m.Space2).Gap(m.Space1).
				AlignItems(ui.Center).Radius(m.RadiusMD).
				Background(tk.BgElevated).Children(func() {
				ui.Icon(c, icons.Get("search")).FontSize(m.IconXS).
					TextColor(tk.TextMuted)
				ui.TextInputBase(c, search).Grow(1).FontSize(m.FontXS).
					Placeholder(locales.SearchRequests()).OnChange(func() {
					// TODO: 搜索当前workspace中的结合、文件夹、请求等
				})
			})

			addMenuButton(c, props)
			environmentMenuButton(c, props)

			// TODO：待添加其他功能
			v1.MiniButton(c, v1.RdButtonOptions{
				Key:      "more",
				Label:    "more",
				Icon:     icons.Get("ellipsis"),
				Size:     m.SizeLG,
				IconSize: m.IconMD,
				Radius:   m.RadiusMD,
			})
		})

		ui.Divider(c)

		// 空提示
		if len(props.collections) == 0 {
			emptySidebar(c, props)
			return
		}

		// 请求树形结构
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
		ui.Scroll(c).Grow(1).Padding(m.Space1).Children(func() {
			nodes := filterTree(buildTree(props.collections), *search, props.state)
			if len(nodes) == 0 && *search != "" {
				ui.Box(c).Padding(m.Space3).Center().Children(func() {
					ui.Text(c, "未搜索到匹配请求").FontSize(m.FontXS).TextColor(tk.TextMuted)
				})
			} else {
				collectionTree(c, props.state, nodes, actions)
			}
		})
	})

	newCollectionDialog(c, props)
	importCurlDialog(c, props)
	importPostmanDialog(c, props)
}

// newCollectionDialog 弹出新建集合对话框。
func newCollectionDialog(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	state := props.state

	ui.DialogBase(c, &state.newCollOpen, func(backdrop, panel ui.Element) {
		backdrop.Background(tk.Overlay)
		panel.Width(420).Padding(m.Space4).Gap(m.Space3)
		theme.DialogPanel(c, panel)
		panel.Children(func() {
			ui.Row(c).FillWidth().AlignItems(ui.Center).Gap(8).Children(func() {
				ui.Icon(c, icons.Get("library")).FontSize(m.IconMD).TextColor(tk.Accent)
				ui.Text(c, locales.NewCollection()).FontSize(m.FontMD).Bold().
					TextColor(tk.TextPrimary)
			})
			ui.Text(c, locales.NewCollectionDesc(props.workspaceName)).
				FontSize(m.FontXS).TextColor(tk.TextMuted)

			ui.Text(c, locales.CollectionNameLabel()).FontSize(m.FontSM).
				FontWeight(500).TextColor(tk.TextSecondary)
			in := ui.TextInput(c, &state.newCollName).Height(m.SizeXL).
				FillWidth().Padding(0, m.Space2).Radius(m.RadiusSM).
				Placeholder(locales.CollectionNamePlaceholder())
			if state.newCollErr != "" {
				ui.Text(c, state.newCollErr).FontSize(m.FontXS).TextColor(tk.Danger)
			}

			ui.Text(c, locales.CollectionDescriptionLabel()).FontSize(m.FontSM).
				FontWeight(500).TextColor(tk.TextSecondary)
			ui.TextArea(c, &state.newCollDesc).FillWidth().MinHeight(80).
				MaxHeight(180).Padding(m.Space2, m.Space2).Radius(m.RadiusSM).
				FontSize(m.FontSM).FixedLineHeight(20).
				Placeholder(locales.CollectionDescriptionPlaceholder())

			submit := func() {
				name := strings.TrimSpace(state.newCollName)
				if name == "" {
					state.newCollErr = locales.CollectionNameRequired()
					return
				}
				coll, err := props.actions.createCollection(name, state.newCollDesc)
				if err != nil {
					state.newCollErr = err.Error()
					return
				}
				state.newCollOpen = false
				state.newCollErr = ""
				state.openTab(string(coll.ID))
			}

			if in.Submitted() {
				submit()
			}

			ui.Row(c).Gap(m.Space2).Justify(ui.End).Children(func() {
				if ui.Button(c, locales.Cancel()).Clicked() {
					state.newCollOpen = false
				}
				if ui.PrimaryButton(c, locales.CreateCollection()).Clicked() {
					submit()
				}
			})
		})
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

// emptySidebar 是设计稿里的空侧栏：图标、标题、说明与新建按钮，垂直居中。
func emptySidebar(c *ui.Context, props pageProps) {
	m := theme.Metrics
	tk := theme.Of(c)
	ui.Column(c).Grow(1).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
		ui.Column(c).Width(174).AlignItems(ui.Center).Gap(10).Children(func() {
			ui.Box(c).Size(56, 56).Radius(m.RadiusLG).
				Background(tk.BgElevated).Children(func() {
				ui.Box(c).Width(14).Height(5).Absolute().Left(21).Top(5).
					Background(tk.TextMuted)
				ui.Box(c).Width(28).Height(20).Absolute().Left(14).Top(10).
					Background(tk.TextMuted)
				ui.Box(c).Width(28).Height(15).Absolute().Left(14).Top(15).
					Background(tk.TextSecondary)
			})
			ui.Text(c, locales.EmptyCollectionsTitle()).FontSize(m.FontMD).
				Bold().TextColor(tk.TextPrimary)
			ui.Text(c, locales.EmptyCollectionsDesc()).Width(174).FontSize(m.FontXS).
				TextColor(tk.TextSecondary).TextAlign(ui.Center)

			btn := ui.ButtonBase(c.Key("empty-new-collection")).Height(34).
				PaddingX(m.Space3).Gap(m.Space1).AlignItems(ui.Center).
				Radius(m.RadiusMD).Background(tk.Accent).TextColor(tk.OnAccent).
				Label(locales.NewCollection())
			if btn.Hovered() {
				btn.Background(tk.Accent.Mix(tk.TextPrimary, 0.06))
			}
			if btn.Clicked() {
				props.state.newCollOpen = true
				props.state.newCollName = ""
				props.state.newCollDesc = ""
				props.state.newCollErr = ""
			}
			btn.Children(func() {
				ui.Icon(c, icons.Get("plus")).FontSize(m.IconXS).
					TextColor(tk.OnAccent)
				ui.Text(c, locales.NewCollection()).FontSize(m.FontSM).
					TextColor(tk.OnAccent)
			})
		})
	})
}

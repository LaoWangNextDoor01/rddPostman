package collection

import (
	"fmt"
	"strings"
	"time"

	"rddpostman/components/markdown"
	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// 概览页分段。只有概览有内容，其余分段按设计稿保留入口。
const (
	segOverview = "overview"
	segAuth     = "auth"
	segScripts  = "scripts"
	segVars     = "vars"
	segRun      = "run"

	// docWidth 是设计稿里 720 宽的正文列。
	docWidth = 720
)

// titleState 是标题的编辑态。切换 tab 时按 id 重置，避免草稿串到别的页面。
type titleState struct {
	id      string
	draft   string
	editing bool
	fresh   bool
}

// descState 是描述的编辑态。切换 tab 时按 id 重置，避免草稿串到别的页面。
type descState struct {
	id      string
	draft   string
	editing bool
	fresh   bool
	mode    string // "write" 或 "preview"
}

// overviewTab 画集合概览（node 为 nil）或文件夹概览。
func overviewTab(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node) {
	m, tk := theme.Metrics, theme.Of(c)
	id := string(coll.ID)
	if node != nil {
		id = string(node.ID)
	}
	root := ui.Column(c).Grow(1).Background(tk.BgSurface)
	title := ui.Local(root, "title-edit", func() titleState { return titleState{id: id} })
	if title.id != id {
		*title = titleState{id: id}
	}
	desc := ui.Local(root, "desc-edit", func() descState { return descState{id: id, mode: "write"} })
	if desc.id != id {
		if desc.editing && desc.id != "" && desc.draft != "" {
			props.actions.updateDescription(desc.id, desc.draft)
		}
		*desc = descState{id: id, mode: "write"}
	}
	seg := props.state.segment(id)
	root.Children(func() {
		docHeader(c, props, coll, node)
		segmentBar(c, props, node, id)

		ui.Box(c).Height(m.Space1)
		ui.Divider(c).BorderColor(tk.Border)

		if seg == segOverview {
			docBody(c, props, coll, node, id, title, desc)
		} else {
			// TODO:补充其他界面
			ui.Box(c).Grow(1)
		}
	})
}

// docHeader 是设计稿的文档头：左侧集合名或面包屑导航。
func docHeader(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node) {
	m, tk := theme.Metrics, theme.Of(c)

	id := string(coll.ID)
	if node != nil {
		id = string(node.ID)
	}
	crumbs := nodeTrail(props.collections, id)
	if len(crumbs) == 0 {
		crumbs = []CrumbItem{{ID: string(coll.ID), Name: coll.Name}}
	}

	ui.Row(c).Height(m.Space1*10).Padding(0, m.Space3).
		AlignItems(ui.Center).Children(func() {
		if len(crumbs) == 1 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Get("library")).Size(14, 14).
					TextColor(tk.TextSecondary)
				ui.Text(c, crumbs[0].Name).FontSize(m.FontMD).FontWeight(500).
					TextColor(tk.TextPrimary).SingleLine().Bold()
			})
			return
		}

		ui.Row(c).Height(20).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Get("library")).Size(14, 14).TextColor(tk.TextSecondary)

			for i, cr := range crumbs {
				cr := cr
				isLast := i == len(crumbs)-1

				if i > 0 {
					ui.Text(c, "/").FontSize(m.FontSM).
						TextColor(tk.TextMuted).Margin(0, 2)
				}

				if isLast {
					ui.Text(c, cr.Name).FontSize(m.FontSM).FontWeight(500).
						TextColor(tk.TextPrimary).SingleLine()
				} else {
					btn := ui.ButtonBase(c.Key("crumb-" + id + "-" + cr.ID)).
						Height(20).PaddingX(4).Radius(m.RadiusSM).
						AlignItems(ui.Center)
					if btn.Hovered() {
						btn.Background(tk.HoverSurface)
					}
					if btn.Clicked() {
						props.actions.openTab(cr.ID)
					}
					btn.Children(func() {
						textColor := tk.TextMuted
						if btn.Hovered() {
							textColor = tk.TextPrimary
						}
						ui.Text(c, cr.Name).FontSize(m.FontSM).
							TextColor(textColor).SingleLine()
					})
				}
			}
		})
	})
}

// segmentBar 是设计稿的分段栏：概览、授权、脚本、变量，集合页多一个运行。
func segmentBar(c *ui.Context, props pageProps, node *domain.Node, id string) {
	m := theme.Metrics
	tk := theme.Of(c)
	type segTab struct {
		key   string
		label string
		dot   bool
	}
	segs := []segTab{
		{segOverview, locales.Overview(), false},
		{segAuth, locales.AuthorizationTab(), true},
		{segScripts, locales.ScriptsTab(), false},
		{segVars, locales.VariablesTab(), false},
	}
	if node == nil {
		segs = append(segs, segTab{segRun, locales.RunTab(), false})
	}
	active := props.state.segment(id)

	ui.Row(c).Height(32).Padding(0, m.Space3).
		AlignItems(ui.Center).Children(func() {
		ui.Row(c).Gap(m.Space1).AlignItems(ui.Center).Children(func() {
			for _, s := range segs {
				s := s
				btn := ui.ButtonBase(c.Key("seg-" + id + "-" + s.key)).Height(24).
					PaddingX(10).Gap(5).AlignItems(ui.Center).Radius(m.RadiusMD).
					Label(s.label).Role(ui.RoleTab).TextColor(tk.TextSecondary)
				switch {
				case s.key == active:
					btn.Background(tk.BgElevated).TextColor(tk.TextPrimary)
				case btn.Hovered():
					btn.Background(tk.HoverSurface)
				}
				if btn.Clicked() {
					props.state.setSegment(id, s.key)
				}
				btn.Children(func() {
					text := ui.Text(c, s.label).FontSize(m.FontXS)
					if s.key == active {
						text.FontWeight(600)
					}
					if s.dot {
						ui.Box(c).Size(5, 5).Radius(m.RadiusFull).
							Background(tk.Success)
					}
				})
			}
		})
	})
}

// docBody 是 720 宽的正文列：标题、元信息、分隔线、描述、说明列表与文档链接。
func docBody(c *ui.Context, props pageProps, coll *domain.Collection,
	node *domain.Node, id string, title *titleState, desc *descState,
) {
	m, tk := theme.Metrics, theme.Of(c)
	name := coll.Name
	description := coll.Description
	updated := coll.UpdatedAt
	requests := requestTotal(coll.Items)
	folders := folderTotal(coll.Items)

	if node != nil {
		name = node.Name
		description = node.Description
		updated = node.UpdatedAt
		requests = requestTotal(node.Children)
		folders = folderTotal(node.Children)
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(34, 0, 24, 0).Children(func() {
				// 标题
				editableTitle(c, props, id, name, title)
				// meta信息，例如请求数量、更新时间
				metaRow(c, tk, m, []string{
					locales.RequestCount(requests),
					locales.FolderCount(folders),
				}, locales.UpdatedAt(relativeTime(updated)))

				ui.Box(c).Background(tk.Border).Margin(18, 0, 0, 0)

				editableDescription(c, props, id, description, desc)

				// if node == nil {
				// 	envSection(c, props, tk, m)
				// } else {
				// 	tocSection(c, node, tk, m)
				// }

				link := ui.Text(c, locales.ViewFullDocs()).FontSize(m.FontSM).
					FontWeight(500).TextColor(tk.Info).Margin(31, 0, 0, 0)
				if link.Clicked() {
					props.actions.navigate(routes.Docs)
				}
			})
		})
	})
}

// editableDescription 画支持 Markdown 渲染与编辑的描述区域。
func editableDescription(c *ui.Context, props pageProps, id, description string, desc *descState) {
	m := theme.Metrics
	tk := theme.Of(c)

	if !desc.editing {
		if strings.TrimSpace(description) == "" {
			btn := ui.ButtonBase(c.Key("empty-desc-btn")).Width(docWidth).MinHeight(58).
				Margin(20, 0, 10, 0).Padding(14, 18).Radius(m.RadiusMD).
				Background(tk.BgElevated.Alpha(0.4)).Border(m.Border, tk.Border).
				AlignItems(ui.Center).Gap(8)
			if btn.Hovered() {
				btn.Background(tk.HoverSurface).Border(m.Border, tk.Accent)
			}
			if btn.Clicked() {
				desc.editing = true
				desc.draft = ""
				desc.fresh = true
				desc.mode = "write"
			}
			btn.Children(func() {
				ui.Icon(c, icons.Get("plus")).FontSize(m.IconSM).TextColor(tk.TextMuted)
				ui.Text(c, "添加描述（支持 Markdown 语法，点击开始编辑）…").
					FontSize(m.FontSM).TextColor(tk.TextMuted)
			})
			return
		}

		ui.Column(c).Width(docWidth).Margin(18, 0, 10, 0).Gap(m.Space2).Children(func() {
			ui.Row(c).FillWidth().Height(m.Space1 * 7).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.CollectionDescriptionLabel()).
					FontSize(m.FontSM).FontWeight(600).TextColor(tk.TextSecondary)
				ui.Box(c).Margin(0, 6).Padding(1, 6).Radius(m.RadiusSM).
					Background(tk.BgElevated).Children(func() {
					ui.Text(c, "Markdown").FontSize(10).TextColor(tk.TextMuted)
				})
				ui.Spacer(c)
				editBtn := ui.ButtonBase(c.Key("edit-desc-btn")).Height(24).PaddingX(8).
					Radius(m.RadiusSM).AlignItems(ui.Center).Gap(4)
				if editBtn.Hovered() {
					editBtn.Background(tk.HoverSurface)
				}
				if editBtn.Clicked() {
					desc.editing = true
					desc.draft = description
					desc.fresh = true
					desc.mode = "write"
				}
				editBtn.Children(func() {
					ui.Icon(c, icons.Get("file-pen")).FontSize(m.FontMD).TextColor(tk.TextSecondary)
					ui.Text(c, "编辑").FontSize(m.FontSM).TextColor(tk.TextSecondary)
				})
			})

			markdown.ViewWithWidth(c, description, docWidth)
		})
		return
	}

	// 编辑模式
	done := func() {
		desc.editing = false
		if desc.draft != description {
			props.actions.updateDescription(id, desc.draft)
		}
	}

	editorBox := ui.Column(c).Width(docWidth).Margin(m.Space4, 0).Radius(m.RadiusMD).
		Background(tk.BgSurface)

	if !desc.fresh && editorBox.FocusLeft() {
		done()
		return
	}

	editorBox.Children(func() {
		// 1. 顶部工具栏：Tab（编辑 / 预览）+ 快速插入工具按钮
		ui.Row(c).FillWidth().Height(34).PaddingX(6).AlignItems(ui.Center).
			BorderWidth(1).BorderColor(tk.Border).
			Radius(m.RadiusMD).
			Children(func() {
				if desc.mode == "write" {
					ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
						tool := func(key, iconName, insert string) {
							tBtn := ui.ButtonBase(c.Key("tool-"+key)).KeepFocus().Size(24, 24).
								Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center)
							if tBtn.Hovered() {
								tBtn.Background(tk.HoverSurface)
							}
							if tBtn.Clicked() {
								desc.draft += insert
							}
							tBtn.Children(func() {
								iconColor := tk.TextSecondary
								if tBtn.Hovered() {
									iconColor = tk.TextPrimary
								}
								ui.Icon(c, icons.Get(iconName)).Size(14, 14).TextColor(iconColor)
							})
						}
						sep := func() {
							ui.Box(c).Width(1).Height(14).Background(tk.Border).Margin(0, 3)
						}

						tool("bold", "bold", "**粗体**")
						tool("italic", "italic", "*斜体*")
						tool("strikethrough", "strikethrough", "~~删除线~~")
						sep()
						tool("heading", "heading", "\n### 标题\n")
						sep()
						tool("quote", "quote", "\n> 引用\n")
						tool("code", "code", "`代码`")
						sep()
						tool("list", "list", "\n- 列表项\n")
						tool("list-ordered", "list-ordered", "\n1. 列表项\n")
						sep()
						tool("link", "link", "[链接文本](url)")
						tool("table", "table", "\n| 列 1 | 列 2 |\n| --- | --- |\n| 内容 | 内容 |\n")
					})
				}

				ui.Spacer(c)

				ui.Row(c).Height(26).Padding(2).Gap(2).
					Radius(m.RadiusSM).Background(tk.HoverSurface.Alpha(0.6)).
					AlignItems(ui.Center).Children(func() {
					writeBtn := ui.ButtonBase(c.Key("desc-tab-write")).Height(22).PaddingX(8).
						Radius(m.RadiusSM).AlignItems(ui.Center)
					if desc.mode == "write" {
						writeBtn.Background(tk.FocusRing).TextColor(tk.BgSurface)
					} else if writeBtn.Hovered() {
						writeBtn.Background(tk.HoverSurface)
					}
					if writeBtn.Clicked() {
						desc.mode = "write"
						desc.fresh = true
					}

					writeBtn.Children(func() {
						ui.Text(c, "编辑").FontSize(m.FontXS)
					})

					prevBtn := ui.ButtonBase(c.Key("desc-tab-prev")).Height(22).PaddingX(8).
						Radius(m.RadiusSM).AlignItems(ui.Center)
					if desc.mode == "preview" {
						prevBtn.Background(tk.FocusRing).TextColor(tk.BgSurface)
					} else if prevBtn.Hovered() {
						prevBtn.Background(tk.HoverSurface)
					}
					if prevBtn.Clicked() {
						desc.mode = "preview"
						if desc.draft != description {
							props.actions.updateDescription(id, desc.draft)
						}
					}
					prevBtn.Children(func() {
						ui.Text(c, "预览").FontSize(m.FontXS)
					})
				})

				doneBtn := ui.ButtonBase(c.Key("desc-tab-done")).Height(26).PaddingX(8).
					Margin(0, 0, 0, 4).Radius(m.RadiusSM).AlignItems(ui.Center)
				if doneBtn.Hovered() {
					doneBtn.Background(tk.HoverSurface)
				}
				if doneBtn.Clicked() {
					done()
					return
				}
				doneBtn.Children(func() {
					ui.Text(c, "完成").FontSize(m.FontXS).TextColor(tk.TextSecondary)
				})
			})

		// 2. 主体编辑或预览区
		if desc.mode == "write" {
			in := ui.TextAreaBase(c, &desc.draft).FillWidth().MinHeight(180).
				MaxHeight(480).Padding(12, 14).FontSize(m.FontSM).
				FixedLineHeight(22).TextColor(tk.TextPrimary)
			if desc.fresh {
				desc.fresh = false
				in.AutoFocus()
				return
			}
			if in.Shortcut(ui.Cmd, ui.KeyEnter) || in.Shortcut(0, ui.KeyEscape) {
				done()
				return
			}
			if in.FocusLeft() && !editorBox.FocusWithin() {
				done()
				return
			}
		} else {
			ui.Scroll(c).FillWidth().MinHeight(180).MaxHeight(480).
				Padding(14, 16).Children(func() {
				if strings.TrimSpace(desc.draft) == "" {
					ui.Text(c, "（无预览内容）").FontSize(m.FontSM).TextColor(tk.TextMuted)
				} else {
					markdown.View(c, desc.draft)
				}
			})
		}
	})
}

// editableTitle 画可编辑标题：单击进入编辑，Enter 或失焦保存，Esc 取消。
func editableTitle(c *ui.Context, props pageProps, id, name string, title *titleState) {
	m, tk := theme.Metrics, theme.Of(c)
	if !title.editing {
		btn := ui.ButtonBase(c.Key("doc-title")).FillWidth().Height(32).
			Justify(ui.Start).AlignItems(ui.Center).Label(name)
		if btn.Clicked() {
			title.editing, title.fresh, title.draft = true, true, name
		}
		btn.Children(func() {
			ui.Text(c, name).FontSize(m.FontXXL).FontWeight(600).
				TextColor(tk.TextPrimary).SingleLine()
		})
		return
	}

	in := ui.TextInputBase(c, &title.draft).FillWidth().Height(32).
		Padding(0, 6).Radius(m.RadiusSM).Border(m.Border, tk.Accent).
		FontSize(m.FontXXL).FontWeight(600).TextColor(tk.TextPrimary)
	if title.fresh {
		title.fresh = false
		in.AutoFocus()
		return
	}

	done := func(keep bool) {
		title.editing = false
		if !keep {
			return
		}
		next := strings.TrimSpace(title.draft)
		if next == "" || next == name {
			return
		}
		props.actions.rename(id, next)
	}
	switch {
	case in.Submitted():
		done(true)
	case in.Shortcut(0, ui.KeyEscape):
		done(false)
	case !in.Focused():
		done(true)
	}
}

// metaRow 是 11 号元信息行：请求数、文件夹数与更新时间，用圆点分隔。
func metaRow(c *ui.Context, tk theme.Tokens, m theme.Metric, counts []string, updated string) {
	ui.Row(c).Height(14).Gap(10).AlignItems(ui.Center).
		Margin(6, 0, 0, 0).Children(func() {
		for i, count := range counts {
			if i > 0 {
				ui.Text(c, "·").FontSize(m.FontSM).TextColor(tk.TextMuted)
			}
			ui.Text(c, count).FontSize(m.FontSM).
				TextColor(tk.TextSecondary).SingleLine()
		}
		ui.Text(c, "·").FontSize(m.FontSM).TextColor(tk.TextMuted)
		timeLabel := ui.Text(c, updated).FontSize(m.FontSM).TextColor(tk.TextMuted).SingleLine()
		if timeLabel.Hovered() {
			timeLabel.TextColor(tk.TextPrimary)
		}
	})
}

// envSection 是集合概览的「环境说明」：每个环境的名称与 baseUrl。
func envSection(c *ui.Context, props pageProps, tk theme.Tokens, m theme.Metric) {
	if len(props.environments) == 0 {
		return
	}
	ui.Text(c, locales.EnvironmentNotes()).FontSize(m.FontSM).
		FontWeight(500).TextColor(tk.TextPrimary).Margin(18, 0, 0, 0)
	ui.Column(c).Gap(9).Margin(7, 0, 0, 0).Children(func() {
		for _, env := range props.environments {
			ui.Row(c).Height(18).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(3.5, 3.5).Radius(m.RadiusFull).
					Background(tk.TextMuted)
				ui.Text(c, env.Name).Width(84).FontSize(m.FontSM).
					Font("monospace").TextColor(tk.TextSecondary).SingleLine()
				ui.Text(c, envBaseURL(env.Vars)).FontSize(m.FontSM).
					Font("monospace").TextColor(tk.TextSecondary).SingleLine()
			})
		}
	})
}

// tocSection 是文件夹概览的「目录」：直接子节点列表，请求带上方法。
func tocSection(c *ui.Context, node *domain.Node, tk theme.Tokens, m theme.Metric) {
	if len(node.Children) == 0 {
		return
	}
	ui.Text(c, locales.Contents()).FontSize(m.FontSM).FontWeight(500).
		TextColor(tk.TextPrimary).Margin(18, 0, 0, 0)
	ui.Column(c).Gap(9).Margin(7, 0, 0, 0).Children(func() {
		for _, child := range node.Children {
			label := child.Name
			if child.Kind == domain.KindRequest && child.Request != nil {
				label = fmt.Sprintf("%s（%s）", child.Name, child.Request.Method)
			}
			ui.Row(c).Height(18).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Box(c).Size(3.5, 3.5).Radius(m.RadiusFull).
					Background(tk.TextMuted)
				ui.Text(c, label).FontSize(m.FontSM).
					TextColor(tk.TextSecondary).SingleLine()
			})
		}
	})
}

// requestTotal 递归统计请求数。
func requestTotal(items []*domain.Node) int {
	total := 0
	for _, it := range items {
		if it.Kind == domain.KindRequest {
			total++
			continue
		}
		total += requestTotal(it.Children)
	}
	return total
}

// folderTotal 统计直接子目录数，与设计稿「N 个文件夹」一致。
func folderTotal(items []*domain.Node) int {
	total := 0
	for _, it := range items {
		if it.Kind == domain.KindFolder {
			total++
		}
	}
	return total
}

// envBaseURL 取环境里的 baseUrl 变量值。
func envBaseURL(vars []domain.KV) string {
	for _, v := range vars {
		if strings.EqualFold(v.Key, "baseUrl") {
			return v.Value
		}
	}
	return ""
}

// relativeTime 把更新时间转成设计稿里的相对时间文案。
func relativeTime(t time.Time) string {
	if t.IsZero() {
		return locales.RelativeJustNow()
	}
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return locales.RelativeJustNow()
	case d < time.Hour:
		return locales.RelativeMinutes(int(d.Minutes()))
	case d < 24*time.Hour:
		return locales.RelativeHours(int(d.Hours()))
	case d < 48*time.Hour:
		return locales.RelativeYesterday()
	default:
		return locales.RelativeDays(int(d.Hours() / 24))
	}
}

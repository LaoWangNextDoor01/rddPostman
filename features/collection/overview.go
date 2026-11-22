package collection

import (
	"fmt"
	"strings"
	"time"

	"rddpostman/components/markdown"
	"rddpostman/components/theme"
	thV2 "rddpostman/components/theme/v2"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
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
	m := theme.Metrics
	tk2 := thV2.Of(c)
	id := string(coll.ID)
	if node != nil {
		id = string(node.ID)
	}
	root := ui.Column(c).Grow(1).MinHeight(0).Background(tk2.BgSurface)
	title := ui.Local(root, "title-edit", func() titleState { return titleState{id: id} })
	if title.id != id {
		*title = titleState{id: id}
	}
	// 当外部新建集合打开新 Tab 时，自动进入行内编辑并全选输入框（对齐 Postman 新建集合交互）
	if props.state.focusNewCollID == id {
		title.editing = true
		title.fresh = true
		title.draft = coll.Name
		props.state.focusNewCollID = ""
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
		segmentBar(c, props, coll, node, id)

		ui.Box(c).Height(m.Space1)
		ui.Divider(c).BorderColor(tk2.Border)

		switch seg {
		case segOverview:
			docBody(c, props, coll, node, id, title, desc)
		case segAuth:
			docAuth(c, props, coll, node, id)
		case segVars:
			docVars(c, props, coll, node, id)
		case segScripts:
			docScripts(c, props, coll, node, id)
		case segRun:
			docRun(c, props, coll, node, id)
		default:
			docBody(c, props, coll, node, id, title, desc)
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
func segmentBar(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	m := theme.Metrics
	tk2 := thV2.Of(c)

	hasAuth := (node != nil && node.Auth.Type != "" && node.Auth.Type != domain.AuthNone) ||
		(coll != nil && coll.Auth.Type != "" && coll.Auth.Type != domain.AuthNone)
	hasVars := (node != nil && len(node.Variables) > 0) || (coll != nil && len(coll.Variables) > 0)

	type segTab struct {
		key   string
		label string
		dot   bool
	}
	segs := []segTab{
		{segOverview, locales.Overview(), false},
		{segAuth, locales.AuthorizationTab(), hasAuth},
		{segScripts, locales.ScriptsTab(), false},
		{segVars, locales.VariablesTab(), hasVars},
	}
	if node == nil {
		segs = append(segs, segTab{segRun, locales.RunTab(), false})
	}
	active := props.state.segment(id)

	ui.Row(c).Height(32).Padding(0, m.Space3).
		AlignItems(ui.Center).Children(func() {
		ui.Row(c).Height(28).Padding(2).Gap(2).Radius(6).
			Background(tk2.BgSubtle).AlignItems(ui.Center).Children(func() {
			for _, s := range segs {
				s := s
				isActive := s.key == active
				btn := ui.ButtonBase(c.Key("seg-" + id + "-" + s.key)).Height(24).MinWidth(50).
					PaddingX(10).Gap(5).AlignItems(ui.Center).Radius(4).
					Label(s.label).Role(ui.RoleTab).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case isActive:
					btn.Background(tk2.BgSurface).
						Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
				case btn.Pressed():
					btn.Background(tk2.BgSelected)
				case btn.Hovered():
					btn.Background(tk2.BgHover)
				default:
					btn.Background(ui.Transparent)
				}
				if btn.Clicked() {
					props.state.setSegment(id, s.key)
				}
				btn.Children(func() {
					color := tk2.TextSecondary
					if isActive {
						color = tk2.TextPrimary
					}
					// 统一使用 Medium (500) 字重，避免在选中与未选切换时排版跳动（Layout Shift）
					ui.Text(c, s.label).FontSize(12).FontWeight(500).TextColor(color).SingleLine()
					if s.dot {
						ui.Box(c).Size(5, 5).Radius(m.RadiusFull).
							Background(tk2.Accent)
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
					props.actions.navigate(routes.WorkbenchDocsOf(string(props.workspaceID)))
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

// editableTitle 画可编辑标题：单击或点击编辑笔图标进入编辑，Enter 或失焦保存，Esc 取消。
func editableTitle(c *ui.Context, props pageProps, id, name string, title *titleState) {
	m := theme.Metrics
	tk2 := thV2.Of(c)
	if !title.editing {
		btn := ui.ButtonBase(c.Key("doc-title")).FillWidth().Height(36).
			Justify(ui.Start).AlignItems(ui.Center).Gap(8).Label(name).
			Cursor(ui.CursorPointer).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
		if btn.Hovered() {
			btn.Background(tk2.BgSubtle)
		}
		if btn.Clicked() {
			title.editing, title.fresh, title.draft = true, true, name
		}
		btn.Children(func() {
			ui.Text(c, name).FontSize(22).FontWeight(600).
				TextColor(tk2.TextPrimary).SingleLine()
			penColor := tk2.TextMuted
			if btn.Hovered() {
				penColor = tk2.Accent
			}
			ui.Icon(c, icons.FilePen).FontSize(14).TextColor(penColor)
		})
		return
	}

	in := ui.TextInputBase(c.Key("doc-title-input"), &title.draft).
		FillWidth().Height(36).Padding(0, 8).Radius(m.RadiusSM).
		Border(1, tk2.Accent).FontSize(22).FontWeight(600).
		TextColor(tk2.TextPrimary)
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

// -----------------------------------------------------------------------------
// 集合 Tab 分段界面（对齐 Postman 集合内联编辑：认证 / 变量 / 脚本 / 运行）
// -----------------------------------------------------------------------------

type collAuthState struct {
	id    string
	token string
	user  string
	pass  string
	key   string
	val   string
}

// docAuth 渲染集合 / 目录级认证配置界面。
func docAuth(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	var currAuth domain.Auth
	if node != nil {
		currAuth = node.Auth
	} else if coll != nil {
		currAuth = coll.Auth
	}
	authType := string(currAuth.Type)
	if authType == "" {
		authType = "none"
	}

	root := ui.Scroll(c).Grow(1)
	state := ui.Local(root, "coll-auth-state-"+id, func() collAuthState {
		st := collAuthState{id: id}
		for _, kv := range currAuth.Params {
			switch strings.ToLower(kv.Key) {
			case "token":
				st.token = kv.Value
			case "username":
				st.user = kv.Value
			case "password":
				st.pass = kv.Value
			case "key":
				st.key = kv.Value
			case "value":
				st.val = kv.Value
			}
		}
		return st
	})
	if state.id != id {
		*state = collAuthState{id: id}
		for _, kv := range currAuth.Params {
			switch strings.ToLower(kv.Key) {
			case "token":
				state.token = kv.Value
			case "username":
				state.user = kv.Value
			case "password":
				state.pass = kv.Value
			case "key":
				state.key = kv.Value
			case "value":
				state.val = kv.Value
			}
		}
	}

	persistAuth := func() {
		updated := currAuth
		switch authType {
		case "bearer":
			updated.Params = []domain.KV{{Key: "token", Value: state.token, Enabled: true}}
		case "basic":
			updated.Params = []domain.KV{
				{Key: "username", Value: state.user, Enabled: true},
				{Key: "password", Value: state.pass, Enabled: true},
			}
		case "apikey":
			updated.Params = []domain.KV{
				{Key: "key", Value: state.key, Enabled: true},
				{Key: "value", Value: state.val, Enabled: true},
			}
		default:
			updated.Params = nil
		}
		if props.actions.updateAuth != nil {
			props.actions.updateAuth(id, updated)
		}
	}

	root.Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(32, 0, 40, 0).Gap(16).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "集合认证 (Authorization)").FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, "配置集合级的默认认证凭据。集合下所有设为「继承父级」的子目录与请求将自动继承此配置。").
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				ui.Row(c).Height(32).AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Text(c, "Type:").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)

					v2.Select(c, v2.SelectOpts{
						Key:       "coll-auth-select-" + id,
						Label:     "Auth Type",
						Width:     240,
						Height:    30,
						MenuWidth: 240,
						Value:     authType,
						Options: []v2.SelectOption{
							{Value: "none", Label: "No Auth (无认证)"},
							{Value: "bearer", Label: "Bearer Token"},
							{Value: "basic", Label: "Basic Auth"},
							{Value: "apikey", Label: "API Key"},
						},
						OnSelect: func(val string) {
							updated := currAuth
							updated.Type = domain.AuthType(val)
							if props.actions.updateAuth != nil {
								props.actions.updateAuth(id, updated)
							}
						},
					})
				})

				switch authType {
				case "none":
					ui.Row(c).Padding(12, 14).Radius(m.RadiusSM).Background(tk.BgSubtle).
						AlignItems(ui.Center).Gap(8).Children(func() {
						ui.Icon(c, icons.Info).FontSize(14).TextColor(tk.TextMuted)
						ui.Text(c, "该集合未配置默认认证。所有设置为继承的子请求在发送时不会附带额外的授权头。").
							FontSize(12).TextColor(tk.TextMuted)
					})
				case "bearer":
					ui.Column(c).Gap(6).Children(func() {
						ui.Text(c, "Token").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
						in := ui.TextInputBase(c.Key("coll-bearer-token"), &state.token).
							FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
							Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
							Placeholder("输入 Bearer Token 或 {{token}}")
						if in.Focused() {
							in.Border(1, tk.Accent)
						}
						if in.FocusLeft() || in.Submitted() {
							persistAuth()
						}
					})
				case "basic":
					ui.Column(c).Gap(10).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Username").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-basic-user"), &state.user).
								Width(320).Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary)
							if in.Focused() {
								in.Border(1, tk.Accent)
							}
							if in.FocusLeft() || in.Submitted() {
								persistAuth()
							}
						})
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "Password").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-basic-pass"), &state.pass).
								Width(320).Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary)
							if in.Focused() {
								in.Border(1, tk.Accent)
							}
							if in.FocusLeft() || in.Submitted() {
								persistAuth()
							}
						})
					})
				case "apikey":
					ui.Row(c).Gap(12).Children(func() {
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, "Key").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-apikey-key"), &state.key).
								FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
								Placeholder("例如 X-API-Key")
							if in.Focused() {
								in.Border(1, tk.Accent)
							}
							if in.FocusLeft() || in.Submitted() {
								persistAuth()
							}
						})
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, "Value").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-apikey-val"), &state.val).
								FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
								Placeholder("Key 值或 {{api_key}}")
							if in.Focused() {
								in.Border(1, tk.Accent)
							}
							if in.FocusLeft() || in.Submitted() {
								persistAuth()
							}
						})
					})
				}
			})
		})
	})
}

type collVarsState struct {
	id   string
	vars []domain.KV
}

// docVars 渲染集合级环境变量配置界面。
func docVars(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	var origVars []domain.KV
	if node != nil {
		origVars = node.Variables
	} else if coll != nil {
		origVars = coll.Variables
	}

	root := ui.Scroll(c).Grow(1)
	state := ui.Local(root, "coll-vars-state-"+id, func() collVarsState {
		cp := make([]domain.KV, len(origVars))
		copy(cp, origVars)
		if len(cp) == 0 {
			cp = append(cp, domain.KV{ID: domain.NewID(), Enabled: true})
		}
		return collVarsState{id: id, vars: cp}
	})
	if state.id != id {
		cp := make([]domain.KV, len(origVars))
		copy(cp, origVars)
		if len(cp) == 0 {
			cp = append(cp, domain.KV{ID: domain.NewID(), Enabled: true})
		}
		*state = collVarsState{id: id, vars: cp}
	}

	persist := func() {
		if props.actions.updateVariables != nil {
			props.actions.updateVariables(id, cleanKVList(state.vars))
		}
	}

	root.Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(32, 0, 40, 0).Gap(16).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "集合变量 (Variables)").FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, "集合变量仅在此集合及其子请求内生效。使用 {{variable_name}} 在 URL、标头、请求体中直接引用。").
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				// 变量表格
				ui.Column(c).FillWidth().Radius(m.RadiusSM).
					Border(1, tk.Border).Clip().Children(func() {
					// 表头
					ui.Row(c).FillWidth().Height(28).Background(tk.BgSubtle).
						BorderWidth(0, 0, 1, 0).BorderColor(tk.Border).
						AlignItems(ui.Center).Children(func() {
						ui.Box(c).Width(32)
						ui.Row(c).Width(180).Padding(0, 8).Children(func() {
							ui.Text(c, "Variable").FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
						})
						ui.Row(c).Grow(1).Padding(0, 8).Children(func() {
							ui.Text(c, "Current Value").FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
						})
						ui.Row(c).Width(160).Padding(0, 8).Children(func() {
							ui.Text(c, "Description").FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
						})
						ui.Box(c).Width(32)
					})

					// 各行
					for i := range state.vars {
						idx := i
						kv := &state.vars[idx]
						isLast := idx == len(state.vars)-1

						ui.Row(c).FillWidth().Height(30).AlignItems(ui.Center).
							BorderWidth(0, 0, 1, 0).BorderColor(tk.Border).Children(func() {
							// Checkbox
							ui.Row(c).Width(32).FillHeight().Center().Children(func() {
								chk := ui.ButtonBase(c.Key(fmt.Sprintf("cv-chk-%s-%d", id, idx))).
									Size(15, 15).Radius(3).Center().Cursor(ui.CursorPointer).
									Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
								if kv.Enabled {
									switch {
									case chk.Pressed():
										chk.Background(tk.AccentPressed)
									case chk.Hovered():
										chk.Background(tk.AccentHover)
									default:
										chk.Background(tk.Accent)
									}
									chk.Children(func() {
										ui.Icon(c, icons.Check).Size(10, 10).TextColor(tk.OnAccent)
									})
								} else {
									switch {
									case chk.Pressed():
										chk.Background(tk.BgSubtle).Border(1, tk.Accent)
									case chk.Hovered():
										chk.Border(1, tk.Accent)
									default:
										chk.Border(1, tk.BorderStrong).Background(tk.BgSurface)
									}
								}
								if chk.Clicked() {
									kv.Enabled = !kv.Enabled
									persist()
								}
							})

							// Key
							ui.Row(c).Width(180).FillHeight().AlignItems(ui.Center).
								BorderWidth(0, 1, 0, 0).BorderColor(tk.Border).Children(func() {
								kIn := ui.TextInputBase(c.Key(fmt.Sprintf("cv-k-%s-%d", id, idx)), &kv.Key).
									Fill().Padding(0, 8).FontSize(12).TextColor(tk.TextPrimary).
									Placeholder("Variable")
								if kIn.Changed() && isLast && (kv.Key != "" || kv.Value != "") {
									state.vars = append(state.vars, domain.KV{ID: domain.NewID(), Enabled: true})
								}
								if kIn.FocusLeft() || kIn.Submitted() {
									persist()
								}
							})

							// Value
							ui.Row(c).Grow(1).FillHeight().AlignItems(ui.Center).
								BorderWidth(0, 1, 0, 0).BorderColor(tk.Border).Children(func() {
								vIn := ui.TextInputBase(c.Key(fmt.Sprintf("cv-v-%s-%d", id, idx)), &kv.Value).
									Fill().Padding(0, 8).FontSize(12).TextColor(tk.TextPrimary).
									Placeholder("Value")
								if vIn.Changed() && isLast && (kv.Key != "" || kv.Value != "") {
									state.vars = append(state.vars, domain.KV{ID: domain.NewID(), Enabled: true})
								}
								if vIn.FocusLeft() || vIn.Submitted() {
									persist()
								}
							})

							// Description
							ui.Row(c).Width(160).FillHeight().AlignItems(ui.Center).Children(func() {
								dIn := ui.TextInputBase(c.Key(fmt.Sprintf("cv-d-%s-%d", id, idx)), &kv.Desc).
									Fill().Padding(0, 8).FontSize(12).TextColor(tk.TextSecondary).
									Placeholder("Description")
								if dIn.FocusLeft() || dIn.Submitted() {
									persist()
								}
							})

							// Delete button
							ui.Row(c).Width(32).FillHeight().Center().Children(func() {
								if !isLast || kv.Key != "" || kv.Value != "" {
									delBtn := ui.ButtonBase(c.Key(fmt.Sprintf("cv-del-%s-%d", id, idx))).
										Size(18, 18).Radius(3).Center().Cursor(ui.CursorPointer).
										Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
									switch {
									case delBtn.Pressed():
										delBtn.Background(tk.BgSelected)
									case delBtn.Hovered():
										delBtn.Background(tk.BgHover)
									}
									if delBtn.Clicked() {
										state.vars = append(state.vars[:idx], state.vars[idx+1:]...)
										if len(state.vars) == 0 {
											state.vars = append(state.vars, domain.KV{ID: domain.NewID(), Enabled: true})
										}
										persist()
									}
									delBtn.Children(func() {
										ui.Icon(c, icons.X).Size(10, 10).TextColor(tk.TextMuted)
									})
								}
							})
						})
					}
				})
			})
		})
	})
}

// docScripts 渲染集合级前置脚本配置界面。
func docScripts(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(32, 0, 40, 0).Gap(16).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "前置脚本 (Pre-request Scripts)").FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, "在集合中的每个请求发出前执行的脚本，用于动态签名与参数准备。").
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				ui.Row(c).Padding(12, 14).Radius(m.RadiusSM).Background(tk.BgSubtle).
					AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons.Info).FontSize(14).TextColor(tk.Accent)
					ui.Text(c, "脚本执行支持 pm.environment.get / set 以及 pm.variables.get / set 常用 API。").
						FontSize(12).TextColor(tk.TextSecondary)
				})
			})
		})
	})
}

// docRun 渲染集合批量运行启动界面。
func docRun(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	tk := thV2.Of(c)
	m := thV2.Metrics

	reqTotal := 0
	folderTot := 0
	if coll != nil {
		reqTotal = requestTotal(coll.Items)
		folderTot = folderTotal(coll.Items)
	}

	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).FillWidth().Justify(ui.Center).Children(func() {
			ui.Column(c).Width(docWidth).Padding(32, 0, 40, 0).Gap(20).Children(func() {
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "批量运行 (Collection Runner)").FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, "自动化按序执行该集合下的所有请求，验证 API 契约与断言测试。").
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				ui.Row(c).Gap(16).Children(func() {
					ui.Box(c).Padding(16, 20).Radius(m.RadiusSM).Background(tk.BgSubtle).
						Border(1, tk.Border).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "包含请求数").FontSize(12).TextColor(tk.TextMuted)
							ui.Text(c, fmt.Sprintf("%d", reqTotal)).FontSize(22).FontWeight(600).
								TextColor(tk.TextPrimary)
						})
					})

					ui.Box(c).Padding(16, 20).Radius(m.RadiusSM).Background(tk.BgSubtle).
						Border(1, tk.Border).Children(func() {
						ui.Column(c).Gap(4).Children(func() {
							ui.Text(c, "包含子目录").FontSize(12).TextColor(tk.TextMuted)
							ui.Text(c, fmt.Sprintf("%d", folderTot)).FontSize(22).FontWeight(600).
								TextColor(tk.TextPrimary)
						})
					})
				})

				runBtn := ui.ButtonBase(c.Key("coll-run-start")).
					Height(34).Padding(0, 20).Radius(6).Background(tk.Accent).
					AlignItems(ui.Center).Justify(ui.Center).Cursor(ui.CursorPointer).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case runBtn.Pressed():
					runBtn.Background(tk.AccentPressed)
				case runBtn.Hovered():
					runBtn.Background(tk.AccentHover)
				}
				runBtn.Children(func() {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.Play).FontSize(13).TextColor(tk.OnAccent)
						ui.Text(c, "Run Collection").FontSize(13).FontWeight(500).
							TextColor(tk.OnAccent)
					})
				})
			})
		})
	})
}

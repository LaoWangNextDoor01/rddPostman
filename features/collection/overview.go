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
	m, tk := thV2.Metrics, thV2.Of(c)

	id := string(coll.ID)
	if node != nil {
		id = string(node.ID)
	}
	crumbs := nodeTrail(props.collections, id)
	if len(crumbs) == 0 {
		crumbs = []CrumbItem{{ID: string(coll.ID), Name: coll.Name}}
	}

	ui.Row(c).Height(40).Padding(0, m.Space4).
		AlignItems(ui.Center).Children(func() {
		if len(crumbs) == 1 {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Icon(c, icons.Library).Size(14, 14).
					TextColor(tk.TextSecondary)
				ui.Text(c, crumbs[0].Name).FontSize(m.FontMD).FontWeight(600).
					TextColor(tk.TextPrimary).SingleLine()
			})
			return
		}

		ui.Row(c).Height(20).Gap(4).AlignItems(ui.Center).Children(func() {
			ui.Icon(c, icons.Library).Size(14, 14).TextColor(tk.TextSecondary)

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
						AlignItems(ui.Center).Cursor(ui.CursorPointer)
					if btn.Hovered() {
						btn.Background(tk.BgHover)
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

	ui.Row(c).Height(32).Padding(0, m.Space4).
		AlignItems(ui.Center).Children(func() {
		ui.Row(c).Height(28).Gap(4).AlignItems(ui.Center).Children(func() {
			for _, s := range segs {
				s := s
				isActive := s.key == active
				btn := ui.ButtonBase(c.Key("seg-" + id + "-" + s.key)).Height(26).
					PaddingX(9).Gap(5).AlignItems(ui.Center).Radius(5).
					Cursor(ui.CursorPointer).Label(s.label).Role(ui.RoleTab).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case isActive:
					btn.Background(tk2.BgHover)
				case btn.Pressed():
					btn.Background(tk2.BgSelected)
				case btn.Hovered():
					btn.Background(tk2.BgSubtle)
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
	m, tk := thV2.Metrics, thV2.Of(c)
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
			ui.Column(c).Width(docWidth).Padding(32, 0, 28, 0).Children(func() {
				// 标题
				editableTitle(c, props, id, name, title)
				// meta信息，例如请求数量、更新时间
				metaRow(c, tk, m, []string{
					locales.RequestCount(requests),
					locales.FolderCount(folders),
				}, locales.UpdatedAt(relativeTime(updated)))

				ui.Box(c).FillWidth().Height(m.Border).Background(tk.Border).Margin(16, 0, 0, 0)

				editableDescription(c, props, id, description, desc)
			})
		})
	})
}

// editableDescription 画支持 Markdown 渲染与编辑的描述区域（参考 Postman 极简文档流风格，适配 UI v2 浅/深色主题）。
func editableDescription(c *ui.Context, props pageProps, id, description string, desc *descState) {
	m, tk := thV2.Metrics, thV2.Of(c)

	if !desc.editing {
		if strings.TrimSpace(description) == "" {
			// 默认空态：参考 Postman，无外框无底色，纯文档流占位文案，点击整行区域直接进入编辑态
			btn := ui.ButtonBase(c.Key("empty-desc-btn")).
				Width(docWidth).MinHeight(48).
				Padding(14, 0, 12, 0).
				AlignItems(ui.Start).Justify(ui.Start).
				Cursor(ui.CursorText).Role(ui.RoleButton).Label(locales.OverviewAddDescPlaceholder()).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

			if btn.Clicked() {
				desc.editing = true
				desc.draft = ""
				desc.fresh = true
				desc.mode = "write"
			}

			hovered := btn.Hovered()
			btn.Children(func() {
				textColor := tk.TextMuted
				if hovered {
					textColor = tk.TextSecondary
				}
				ui.Text(c, locales.OverviewAddDescPlaceholder()).
					FontSize(m.FontSM).TextColor(textColor).SingleLine()
			})
			return
		}

		// 非空查看态：简洁标题条 + Markdown 正文渲染
		ui.Column(c).Width(docWidth).Padding(14, 0, 6, 0).Gap(m.Space2).Children(func() {
			ui.Row(c).FillWidth().Height(26).AlignItems(ui.Center).Children(func() {
				ui.Text(c, locales.CollectionDescriptionLabel()).
					FontSize(m.FontXS).FontWeight(500).TextColor(tk.TextMuted).SingleLine()
				ui.Spacer(c)
				editBtn := ui.ButtonBase(c.Key("edit-desc-btn")).Height(24).PaddingX(8).
					Radius(m.RadiusSM).AlignItems(ui.Center).Gap(5).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.Edit()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				switch {
				case editBtn.Pressed():
					editBtn.Background(tk.BgSelected)
				case editBtn.Hovered():
					editBtn.Background(tk.BgHover)
				}
				if editBtn.Clicked() {
					desc.editing = true
					desc.draft = description
					desc.fresh = true
					desc.mode = "write"
				}
				editBtn.Children(func() {
					fg := tk.TextMuted
					if editBtn.Hovered() {
						fg = tk.TextPrimary
					}
					ui.Icon(c, icons.FilePen).FontSize(12).TextColor(fg)
					ui.Text(c, locales.Edit()).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
				})
			})

			markdown.ViewWithWidth(c, description, docWidth)
		})
		return
	}

	// 编辑 / 预览交互态：参考 Postman，顶部无底色工具栏（紧贴上方分割线，仅保留底部 1px 细线）+ 下方无边框开放式编辑区
	done := func() {
		desc.editing = false
		if desc.draft != description {
			props.actions.updateDescription(id, desc.draft)
		}
	}

	editorBox := ui.Column(c.Key("desc-editor-card-"+id)).
		Width(docWidth).Background(tk.BgBase)

	if !desc.fresh && editorBox.FocusLeft() {
		done()
		return
	}

	editorBox.Children(func() {
		// 1. 顶部极简工具栏（38px，与页面同底色，仅底部 1px 细分割线，自然衔接上方标题分割线）
		ui.Row(c).FillWidth().Height(38).AlignItems(ui.Center).
			Background(tk.BgBase).
			BorderWidth(0, 0, m.Border, 0).BorderColor(tk.Border).
			Children(func() {
				if desc.mode == "write" {
					ui.Row(c).Gap(2).AlignItems(ui.Center).Children(func() {
						tool := func(key string, ic *ui.SVG, insert string) {
							tBtn := ui.ButtonBase(c.Key("tool-"+key)).KeepFocus().Size(26, 26).
								Radius(m.RadiusSM).Justify(ui.Center).AlignItems(ui.Center).
								Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(key).
								Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
							switch {
							case tBtn.Pressed():
								tBtn.Background(tk.BgSelected)
							case tBtn.Hovered():
								tBtn.Background(tk.BgHover)
							}
							if tBtn.Clicked() {
								desc.draft += insert
							}
							tBtn.Children(func() {
								iconColor := tk.TextSecondary
								if tBtn.Pressed() {
									iconColor = tk.Accent
								} else if tBtn.Hovered() {
									iconColor = tk.TextPrimary
								}
								ui.Icon(c, ic).Size(14, 14).TextColor(iconColor)
							})
						}
						sep := func() {
							ui.Box(c).Width(1).Height(14).Background(tk.Border).Margin(0, 3)
						}

						tool("bold", icons.Bold, locales.MdBoldSample())
						tool("italic", icons.Italic, locales.MdItalicSample())
						tool("strikethrough", icons.Strikethrough, locales.MdStrikeSample())
						sep()
						tool("heading", icons.Heading, locales.MdHeadingSample())
						sep()
						tool("list", icons.List, locales.MdListSample())
						tool("list-ordered", icons.ListOrdered, locales.MdOrderedListSample())
						tool("table", icons.Table, locales.MdTableSample())
						sep()
						tool("link", icons.Link, locales.MdLinkSample())
						tool("quote", icons.Quote, locales.MdQuoteSample())
						tool("code", icons.Code, locales.MdCodeSample())
					})
				} else {
					ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
						ui.Icon(c, icons.Eye).FontSize(13).TextColor(tk.TextMuted)
						ui.Text(c, locales.Preview()).FontSize(m.FontXS).FontWeight(500).
							TextColor(tk.TextSecondary).SingleLine()
					})
				}

				ui.Spacer(c)

				// 轻量模式切换（编辑 / 预览）
				ui.Row(c).Height(26).Padding(2).Gap(2).
					Radius(m.RadiusSM).Background(tk.BgSubtle).
					AlignItems(ui.Center).Children(func() {
					segBtn := func(key, label string, active bool, onClick func()) {
						b := ui.ButtonBase(c.Key(key)).KeepFocus().Height(22).PaddingX(8).
							Radius(4).AlignItems(ui.Center).Justify(ui.Center).
							Cursor(ui.CursorPointer).Role(ui.RoleTab).Label(label).
							Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
						switch {
						case active:
							b.Background(tk.BgSurface).Border(m.Border, tk.Border)
						case b.Pressed():
							b.Background(tk.BgSelected).Border(m.Border, ui.Transparent)
						case b.Hovered():
							b.Background(tk.BgHover).Border(m.Border, ui.Transparent)
						default:
							b.Background(ui.Transparent).Border(m.Border, ui.Transparent)
						}
						if b.Clicked() && onClick != nil {
							onClick()
						}
						b.Children(func() {
							fg := tk.TextSecondary
							if active || b.Hovered() {
								fg = tk.TextPrimary
							}
							ui.Text(c, label).FontSize(m.FontXS).FontWeight(500).TextColor(fg).SingleLine()
						})
					}

					segBtn("desc-tab-write", locales.Edit(), desc.mode == "write", func() {
						desc.mode = "write"
						desc.fresh = true
					})
					segBtn("desc-tab-prev", locales.Preview(), desc.mode == "preview", func() {
						desc.mode = "preview"
						if desc.draft != description {
							props.actions.updateDescription(id, desc.draft)
						}
					})
				})

				ui.Box(c).Width(1).Height(14).Background(tk.Border).Margin(0, 6)

				// 轻盈克制的「完成」按钮（柔和蓝底，悬停转实色，避免过重抢戏）
				doneBtn := ui.ButtonBase(c.Key("desc-tab-done")).Height(24).PaddingX(10).
					Radius(m.RadiusSM).AlignItems(ui.Center).Justify(ui.Center).
					Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(locales.Done()).
					Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
				doneFg := tk.Accent
				switch {
				case doneBtn.Pressed():
					doneBtn.Background(tk.AccentPressed)
					doneFg = tk.OnAccent
				case doneBtn.Hovered():
					doneBtn.Background(tk.Accent)
					doneFg = tk.OnAccent
				default:
					doneBtn.Background(tk.AccentSoft)
				}
				if doneBtn.Clicked() {
					done()
					return
				}
				doneBtn.Children(func() {
					ui.Text(c, locales.Done()).FontSize(m.FontXS).FontWeight(500).
						TextColor(doneFg).SingleLine()
				})
			})

		// 2. 无边框开放式正文编辑 / 预览区（左齐平，与文档流融为一体）
		if desc.mode == "write" {
			in := ui.TextAreaBase(c.Key("desc-md-input-"+id), &desc.draft).
				FillWidth().MinHeight(120).MaxHeight(460).Padding(14, 0, 8, 0).
				FontSize(m.FontMD).FixedLineHeight(22).
				TextColor(tk.TextPrimary).
				Placeholder(locales.OverviewAddDescPlaceholder()).
				PlaceholderColor(tk.TextMuted).
				Label(locales.CollectionDescriptionLabel())
			if desc.fresh {
				desc.fresh = false
				in.AutoFocus()
			} else if in.Shortcut(ui.Cmd, ui.KeyEnter) || in.Shortcut(0, ui.KeyEscape) {
				done()
				return
			} else if in.FocusLeft() && !editorBox.FocusWithin() {
				done()
				return
			}
		} else {
			ui.Scroll(c).FillWidth().MinHeight(120).MaxHeight(460).
				Padding(14, 0, 8, 0).Children(func() {
				if strings.TrimSpace(desc.draft) == "" {
					ui.Text(c, locales.OverviewEmptyPreview()).FontSize(m.FontSM).TextColor(tk.TextMuted)
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
func metaRow(c *ui.Context, tk thV2.Tokens, m thV2.Metric, counts []string, updated string) {
	ui.Row(c).Height(16).Gap(10).AlignItems(ui.Center).
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
					ui.Text(c, locales.OverviewAuthTitle()).FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, locales.OverviewAuthDesc()).
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				ui.Row(c).Height(32).AlignItems(ui.Center).Gap(12).Children(func() {
					ui.Text(c, locales.AuthTypeLabel()).FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)

					v2.Select(c, v2.SelectOpts{
						Key:       "coll-auth-select-" + id,
						Label:     locales.AuthTypeSelectLabel(),
						Width:     240,
						Height:    30,
						MenuWidth: 240,
						Value:     authType,
						Options: []v2.SelectOption{
							{Value: "none", Label: locales.AuthNone()},
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
						ui.Text(c, locales.OverviewAuthNoneHint()).
							FontSize(12).TextColor(tk.TextMuted)
					})
				case "bearer":
					ui.Column(c).Gap(6).Children(func() {
						ui.Text(c, "Token").FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
						in := ui.TextInputBase(c.Key("coll-bearer-token"), &state.token).
							FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
							Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
							Placeholder(locales.AuthTokenPlaceholder())
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
							ui.Text(c, locales.AuthUsername()).FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
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
							ui.Text(c, locales.AuthPassword()).FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
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
							ui.Text(c, locales.KeyLabel()).FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-apikey-key"), &state.key).
								FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
								Placeholder(locales.AuthAPIKeyKeyPlaceholder())
							if in.Focused() {
								in.Border(1, tk.Accent)
							}
							if in.FocusLeft() || in.Submitted() {
								persistAuth()
							}
						})
						ui.Column(c).Grow(1).Gap(4).Children(func() {
							ui.Text(c, locales.ValueLabel()).FontSize(12).FontWeight(500).TextColor(tk.TextSecondary)
							in := ui.TextInputBase(c.Key("coll-apikey-val"), &state.val).
								FillWidth().Height(32).Padding(0, 10).Radius(m.RadiusSM).
								Border(1, tk.Border).FontSize(13).TextColor(tk.TextPrimary).
								Placeholder(locales.AuthAPIKeyValPlaceholder())
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
					ui.Text(c, locales.OverviewVarsTitle()).FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, locales.OverviewVarsDesc()).
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
							ui.Text(c, locales.VariableName()).FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
						})
						ui.Row(c).Grow(1).Padding(0, 8).Children(func() {
							ui.Text(c, locales.CurrentValue()).FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
						})
						ui.Row(c).Width(160).Padding(0, 8).Children(func() {
							ui.Text(c, locales.DescriptionLabel()).FontSize(11).FontWeight(600).TextColor(tk.TextMuted)
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
									Placeholder(locales.VariableName())
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
									Placeholder(locales.ValueLabel())
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
									Placeholder(locales.DescriptionLabel())
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
					ui.Text(c, locales.OverviewScriptsTitle()).FontSize(18).FontWeight(600).
						TextColor(tk.TextPrimary)
					ui.Text(c, locales.OverviewScriptsDesc()).
						FontSize(13).TextColor(tk.TextMuted)
				})

				ui.Box(c).FillWidth().Height(1).Background(tk.Border)

				ui.Row(c).Padding(12, 14).Radius(m.RadiusSM).Background(tk.BgSubtle).
					AlignItems(ui.Center).Gap(8).Children(func() {
					ui.Icon(c, icons.Info).FontSize(14).TextColor(tk.Accent)
					ui.Text(c, locales.OverviewScriptsHint()).
						FontSize(12).TextColor(tk.TextSecondary)
				})
			})
		})
	})
}

// docRun 渲染集合批量运行启动界面（支持顺序执行请求与实时结果展示）。
func docRun(c *ui.Context, props pageProps, coll *domain.Collection, node *domain.Node, id string) {
	renderCollectionRunner(c, props, coll, node, id)
}

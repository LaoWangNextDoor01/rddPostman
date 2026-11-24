package collection

import (
	"strings"

	thV2 "rddpostman/components/theme/v2"

	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	v2 "rddpostman/components/v2"
	"rddpostman/internal/domain"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// treeKind 是集合树的行类型，只服务 UI 渲染，与 domain.Kind 解耦。
type treeKind int

const (
	treeCollection treeKind = iota
	treeFolder
	treeRequest
)

// treeNode 是集合树的 UI 行模型。数据层结构变化时只需要改 buildTree，
// 渲染代码不受影响。
type treeNode struct {
	id       string
	name     string
	kind     treeKind
	method   string
	url      string
	children []*treeNode
}

// treeActions 是树行操作的外置回调，由页面负责接到 store。
type treeActions struct {
	create    func(row *treeNode, kind treeKind)
	remove    func(row *treeNode)
	open      func(row *treeNode)
	copy      func(text string)
	duplicate func(row *treeNode)
}

// buildTree 把数据层的集合树转换成纯 UI 行模型。
func buildTree(collections []*domain.Collection) []*treeNode {
	out := make([]*treeNode, 0, len(collections))
	for _, coll := range collections {
		out = append(out, &treeNode{
			id:       string(coll.ID),
			name:     coll.Name,
			kind:     treeCollection,
			children: buildNodes(coll.Items),
		})
	}
	return out
}

func buildNodes(nodes []*domain.Node) []*treeNode {
	out := make([]*treeNode, 0, len(nodes))
	for _, n := range nodes {
		row := &treeNode{id: string(n.ID), name: n.Name, kind: treeFolder}
		if n.Kind == domain.KindRequest {
			row.kind = treeRequest
			if n.Request != nil {
				row.method = n.Request.Method
				row.url = n.Request.URL
			}
		} else {
			row.children = buildNodes(n.Children)
		}
		out = append(out, row)
	}
	return out
}

// collectionTree 用 TreeBase 画工作区里的集合树，点击行选中并打开对应内容。
// 全量对齐 theme/v2 统一侧栏规范：26px 行高、中性灰选中态、12px 字号与平滑过渡。
func collectionTree(c *ui.Context, state *workspace, rows []*treeNode, actions treeActions) {
	m, tk := thV2.Metrics, thV2.Of(c)
	tree := ui.TreeBase(c)
	tree.Root.Grow(1).Gap(2).Children(func() {
		var add func(n *treeNode, depth int)
		add = func(n *treeNode, depth int) {
			c.Key("tree-" + n.id)
			var open *bool
			expandable := n.kind != treeRequest
			if expandable {
				open = state.openFlag(n.id)
			}
			item := tree.Item(open)
			row := item.Row.Height(m.RowHeight).Gap(6).
				Padding(0, 8).
				PaddingStart(8 + 16*float32(depth)).
				Radius(m.RadiusSM).
				Cursor(ui.CursorPointer).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			selected := state.selected == n.id
			menuOpen := ui.Local(row, "tree-menu-open", func() bool { return false })
			rowHovered := row.Hovered() || *menuOpen
			switch {
			case row.Pressed(), selected:
				row.Background(tk.BgSelected)
			case rowHovered:
				row.Background(tk.BgHover)
			}
			if row.Clicked() {
				actions.open(n)
			}
			row.Children(func() {
				if expandable {
					arrow := ui.Icon(c, icons.ChevronDown).FontSize(11).
						TextColor(tk.TextMuted)
					if open != nil && !*open {
						arrow.Rotate(-90)
					}
					if arrow.Clicked() && open != nil {
						*open = !*open
					}
				} else {
					ui.Box(c).Width(11)
				}

				switch n.kind {
				case treeCollection:
					icColor := tk.TextSecondary
					if selected {
						icColor = tk.TextPrimary
					}
					ui.Icon(c, icons.Library).FontSize(13).TextColor(icColor)
				case treeRequest:
					ui.Text(c, n.method).FontSize(10).FontWeight(700).
						TextColor(methodColorV2(tk, n.method)).SingleLine()
				default:
					icColor := tk.TextSecondary
					if selected {
						icColor = tk.TextPrimary
					}
					ui.Icon(c, icons.Folder).FontSize(13).TextColor(icColor)
				}

				weight := 400
				if selected {
					weight = 500
				}
				ui.Text(c, n.name).FontSize(m.FontSM).FontWeight(weight).
					Grow(1).SingleLine().TextColor(tk.TextPrimary)

				// 始终构建按钮，保证 popover 打开时锚点还在；
				// 仅在行悬停或菜单展开时可见。
				more := v2.IconButton(c, v2.IconButtonOpts{
					Key:      "tree-menu-" + n.id,
					Label:    locales.TreeMore(),
					Icon:     icons.Ellipsis,
					Size:     20,
					IconSize: 12,
					Radius:   m.RadiusSM,
					Selected: *menuOpen,
				})
				if !rowHovered {
					more.Opacity(0).IgnoreInput(true)
				}
				if more.Clicked() {
					*menuOpen = !*menuOpen
				}
				v2.Popover(c, more, menuOpen, v2.PopoverOpts{Width: 180}, func(panel ui.Element) {
					renderTreeRowMenu(c, n, state, actions, menuOpen)
				})
			})
			if expandable {
				item.Group(func() {
					if len(n.children) == 0 {
						renderTreeEmptyNode(c, n, depth+1, actions)
					} else {
						for _, ch := range n.children {
							add(ch, depth+1)
						}
					}
				})
			}
		}

		for _, n := range rows {
			add(n, 0)
		}
	})
}

// renderTreeRowMenu 使用统一的 v2.Popover + v2.MenuButton + v2.PopoverDivider 渲染节点操作菜单，
// 与侧栏顶部「+」/「...」、历史侧栏「...」及工作区列表菜单保持 100% 视觉与交互一致。
func renderTreeRowMenu(c *ui.Context, n *treeNode, state *workspace, actions treeActions, open *bool) {
	ui.Column(c).Padding(4).Gap(2).Children(func() {
		switch n.kind {
		case treeCollection, treeFolder:
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Send, Key: "tm-new-req-" + n.id, Label: locales.NewRequest(),
			}).Clicked() {
				actions.create(n, treeRequest)
				*open = false
			}
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.FolderPlus, Key: "tm-new-fld-" + n.id, Label: locales.NewFolder(),
			}).Clicked() {
				actions.create(n, treeFolder)
				*open = false
			}
			v2.PopoverDivider(c)
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Layers, Key: "tm-toggle-" + n.id, Label: locales.TreeExpandCollapse(),
			}).Clicked() {
				flag := state.openFlag(n.id)
				*flag = !*flag
				*open = false
			}
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Copy, Key: "tm-copy-name-" + n.id, Label: locales.TreeCopyName(),
			}).Clicked() {
				actions.copy(n.name)
				*open = false
			}
			v2.PopoverDivider(c)
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Trash, Key: "tm-del-" + n.id, Label: locales.TreeDelete(), Danger: true,
			}).Clicked() {
				actions.remove(n)
				*open = false
			}
		case treeRequest:
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Send, Key: "tm-open-" + n.id, Label: locales.TreeOpen(),
			}).Clicked() {
				actions.open(n)
				*open = false
			}
			if actions.duplicate != nil && v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Copy, Key: "tm-dup-" + n.id, Label: "复制请求",
			}).Clicked() {
				actions.duplicate(n)
				*open = false
			}
			v2.PopoverDivider(c)
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Link, Key: "tm-copy-url-" + n.id, Label: locales.TreeCopyURL(),
			}).Clicked() {
				actions.copy(n.url)
				*open = false
			}
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Copy, Key: "tm-copy-name-" + n.id, Label: locales.TreeCopyName(),
			}).Clicked() {
				actions.copy(n.name)
				*open = false
			}
			v2.PopoverDivider(c)
			if v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.Trash, Key: "tm-del-" + n.id, Label: locales.TreeDelete(), Danger: true,
			}).Clicked() {
				actions.remove(n)
				*open = false
			}
		}
	})
}

func methodColor(tk theme.Tokens, method string) ui.Color {
	switch strings.ToUpper(method) {
	case "GET":
		return tk.Success
	case "POST":
		return tk.Accent
	case "PUT":
		return tk.Info
	case "DELETE":
		return tk.Danger
	case "PATCH":
		// 设计稿里 PATCH 用固定紫 #7C5CE0，没有对应的主题变量。
		return ui.Hex("#7C5CE0")
	default:
		return tk.TextMuted
	}
}

// filterTree 递归根据关键词匹配树节点（支持按名称、URL、Method 过滤），
// 如果子节点命中则保留父节点并自动展开目录。
func filterTree(nodes []*treeNode, keyword string, state *workspace) []*treeNode {
	keyword = strings.ToLower(strings.TrimSpace(keyword))
	if keyword == "" {
		return nodes
	}
	var out []*treeNode
	for _, n := range nodes {
		filteredChildren := filterTree(n.children, keyword, state)
		matched := strings.Contains(strings.ToLower(n.name), keyword) ||
			strings.Contains(strings.ToLower(n.url), keyword) ||
			strings.Contains(strings.ToLower(n.method), keyword)

		if matched || len(filteredChildren) > 0 {
			clone := *n
			clone.children = filteredChildren
			out = append(out, &clone)
			if state != nil && len(filteredChildren) > 0 {
				open := state.openFlag(n.id)
				*open = true
			}
		}
	}
	return out
}

// renderTreeEmptyNode 绘制空集合 / 空目录的紧凑引导区：
// 柔和次级灰标题 + 简短说明 + 横排紧凑带图标小按钮，整块区域支持 Hover 背景高亮。
func renderTreeEmptyNode(c *ui.Context, n *treeNode, depth int, actions treeActions) {
	m, tk := thV2.Metrics, thV2.Of(c)
	title := "Collection is empty"
	desc := "Add a request or folder to structure your API workflow."
	if n.kind == treeFolder {
		title = "Folder is empty"
		desc = "Add a request, a folder, or drag items here to group them together."
	}
	// 容器左外边距留出 8px 内边距后，文字与父级图标左边缘（25 + 16*(depth-1)）精确对齐
	indent := float32(17)
	if depth > 1 {
		indent += 16 * float32(depth-1)
	}

	ui.Box(c.Key("tree-empty-wrap-" + n.id)).FillWidth().Padding(2, 4, 6, indent).Children(func() {
		box := ui.Column(c.Key("tree-empty-" + n.id)).FillWidth().
			Padding(8, 8).Gap(4).AlignItems(ui.Start).
			Radius(m.RadiusMD).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
		if box.Hovered() {
			box.Background(tk.BgHover)
		} else {
			box.Background(ui.Transparent)
		}

		box.Children(func() {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			ui.Text(c, desc).FontSize(11).FixedLineHeight(16).TextColor(tk.TextMuted).FillWidth()

			ui.Row(c).FillWidth().Padding(4, 0, 0, 0).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
				treeEmptyOutlineBtn(c, "tree-empty-add-req-"+n.id, icons.Send, "Add request", func() {
					if actions.create != nil {
						actions.create(n, treeRequest)
					}
				})
				treeEmptyOutlineBtn(c, "tree-empty-add-folder-"+n.id, icons.FolderPlus, "Add folder", func() {
					if actions.create != nil {
						actions.create(n, treeFolder)
					}
				})
			})
		})
	})
}

func treeEmptyOutlineBtn(c *ui.Context, key string, icon *ui.SVG, label string, onClick func()) {
	m, tk := thV2.Metrics, thV2.Of(c)
	btn := ui.ButtonBase(c.Key(key)).
		Height(22).PaddingX(7).Gap(4).Radius(m.RadiusSM).
		AlignItems(ui.Center).Justify(ui.Start).
		Cursor(ui.CursorPointer).Role(ui.RoleButton).Label(label).
		Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
	switch {
	case btn.Pressed():
		btn.Background(tk.BgSelected).Border(m.Border, tk.BorderStrong)
	case btn.Hovered():
		btn.Background(tk.BgSurface).Border(m.Border, tk.BorderStrong)
	default:
		btn.Background(ui.Transparent).Border(m.Border, tk.BorderPopover)
	}
	if btn.Clicked() && onClick != nil {
		onClick()
	}
	btn.Children(func() {
		icColor := tk.TextSecondary
		if btn.Hovered() || btn.Pressed() {
			icColor = tk.TextPrimary
		}
		ui.Icon(c, icon).FontSize(11).TextColor(icColor)
		ui.Text(c, label).FontSize(11).FontWeight(400).TextColor(tk.TextPrimary).SingleLine()
	})
}

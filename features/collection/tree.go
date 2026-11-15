package collection

import (
	"rddpostman/components/theme"
	"rddpostman/components/v1/icons"
	"rddpostman/internal/domain"
	"rddpostman/locales"
	"strings"

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
	create func(row *treeNode, kind treeKind)
	remove func(row *treeNode)
	open   func(row *treeNode)
	copy   func(text string)
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
func collectionTree(c *ui.Context, state *workspace, rows []*treeNode, actions treeActions) {
	m := theme.Metrics
	tk := theme.Of(c)
	tree := ui.TreeBase(c)
	tree.Root.Grow(1).Gap(1).Children(func() {
		var add func(n *treeNode, depth int)
		add = func(n *treeNode, depth int) {
			c.Key("tree-" + n.id)
			var open *bool
			expandable := n.kind != treeRequest
			if expandable {
				open = state.openFlag(n.id)
			}
			item := tree.Item(open)
			row := item.Row.Height(m.SizeLG).Gap(m.Space1).
				Padding(0, m.Space1).
				PaddingStart(m.Space1 + 14*float32(depth)).
				Radius(m.RadiusSM).MarginY(1)
			selected := state.selected == n.id
			switch {
			case selected:
				row.Background(tk.AccentSoft)
			case row.Hovered():
				row.Background(tk.HoverSurface)
			}
			if row.Clicked() {
				state.selected = n.id
				state.openTab(n.id)
			}
			row.Children(func() {
				if expandable {
					arrow := ui.Icon(c, icons.Get("chevron-down")).Size(12, 12).
						TextColor(tk.TextMuted)
					if open != nil && !*open {
						arrow.Rotate(-90)
					}
					if arrow.Clicked() && open != nil {
						*open = !*open
					}
				} else {
					ui.Box(c).Width(12)
				}

				switch n.kind {
				case treeCollection:
					ui.Icon(c, icons.Get("library")).FontSize(m.IconSM).
						TextColor(tk.TextPrimary).Bold()
				case treeRequest:
					ui.Text(c, n.method).FontSize(10).TextColor(methodColor(tk, n.method)).Bold()
				default:
					ui.Icon(c, icons.Get("folder")).FontSize(m.IconSM).
						TextColor(tk.TextSecondary)
				}

				textColor := tk.TextSecondary
				if selected {
					textColor = tk.TextPrimary
				}
				title := ui.Text(c, n.name).FontSize(m.FontSM).Grow(1).SingleLine().TextColor(textColor)
				if n.kind == treeCollection {
					title.Bold()
				}

				// 始终构建按钮，保证 popover 打开时锚点还在；
				// 只是整行没有悬停时把它透明掉。
				more := ui.ButtonBase(c.Key("tree-menu-" + n.id)).
					Width(m.SizeSM).Height(m.SizeSM).
					Justify(ui.Center).AlignItems(ui.Center).
					Radius(m.RadiusSM).Label(locales.TreeMore()).
					Role(ui.RoleButton)
				if !row.Hovered() {
					more.Opacity(0).IgnoreInput(true)
				}
				iconColor := tk.TextMuted
				if more.Hovered() {
					// 行本身是 hover-surface，按钮再深一档，避免糊在一起。
					more.Background(tk.Border)
					iconColor = tk.TextPrimary
				}
				more.Children(func() {
					ui.Icon(c, icons.Get("ellipsis")).FontSize(m.IconXS).
						TextColor(iconColor)
				})
				more.PopoverMenuButton(func(menu *ui.Menu) {
					rowMenu(menu, n, state, actions)
				}, ui.PopoverMenuButtonOptions{
					// 菜单面板采用统一的弹层外观，和下拉、对话框一致。
					Panel: func(panel ui.Element) {
						panel.MinWidth(160).Padding(m.Space1).FontSize(m.FontSM)
						theme.PopoverPanel(c, panel)
					},
				})
			})
			if expandable {
				item.Group(func() {
					for _, ch := range n.children {
						add(ch, depth+1)
					}
				})
			}
		}

		for _, n := range rows {
			add(n, 0)
		}
	})
}

// rowMenu 按节点类型构建不同的操作菜单。集合/目录可以继续创建子节点，
// 请求则提供打开、复制 URL 等操作；「其他」是一层子菜单。
func rowMenu(m *ui.Menu, n *treeNode, state *workspace, actions treeActions) {
	switch n.kind {
	case treeCollection, treeFolder:
		if m.Item(locales.NewFolder()).Chosen() {
			actions.create(n, treeFolder)
		}
		if m.Item(locales.NewRequest()).Chosen() {
			actions.create(n, treeRequest)
		}
		m.Separator()
		if m.Item(locales.TreeDelete()).Danger().Chosen() {
			actions.remove(n)
		}
		m.Separator()
		m.Submenu(locales.TreeOther(), func(sub *ui.Menu) {
			if sub.Item(locales.TreeCopyName()).Chosen() {
				actions.copy(n.name)
			}
			if sub.Item(locales.TreeCopyID()).Chosen() {
				actions.copy(n.id)
			}
			if sub.Item(locales.TreeExpandCollapse()).Chosen() {
				open := state.openFlag(n.id)
				*open = !*open
			}
		})
	case treeRequest:
		if m.Item(locales.TreeOpen()).Chosen() {
			actions.open(n)
		}
		m.Separator()
		if m.Item(locales.TreeDelete()).Danger().Chosen() {
			actions.remove(n)
		}
		m.Separator()
		m.Submenu(locales.TreeOther(), func(sub *ui.Menu) {
			if sub.Item(locales.TreeCopyName()).Chosen() {
				actions.copy(n.name)
			}
			if sub.Item(locales.TreeCopyURL()).Chosen() {
				actions.copy(n.url)
			}
			if sub.Item(locales.TreeCopyID()).Chosen() {
				actions.copy(n.id)
			}
		})
	}
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

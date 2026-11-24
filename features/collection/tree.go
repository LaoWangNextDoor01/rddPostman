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
	collID   string
	parentID string
	index    int // 在原始父容器子列表中的下标，过滤后仍保持真实顺序下标
	method   string
	url      string
	children []*treeNode
}

// treeDragItem 是拖拽树节点时携带的不可变载荷。
type treeDragItem struct {
	id       string
	collID   string
	parentID string
	index    int
	kind     treeKind
}

// dropPosition 表示拖拽放置相对目标行的位置。
type dropPosition uint8

const (
	dropNone dropPosition = iota
	dropBefore
	dropInside
	dropAfter
)

// treeActions 是树行操作的外置回调，由页面负责接到 store。
type treeActions struct {
	collections    []*domain.Collection
	create         func(row *treeNode, kind treeKind)
	remove         func(row *treeNode)
	open           func(row *treeNode)
	copy           func(text string)
	duplicate      func(row *treeNode)
	move           func(srcCollID, nodeID, dstCollID, dstParentID string, dstIndex int)
	moveCollection func(collID string, dstIndex int)
	openMoveDialog func(row *treeNode)
}

// buildTree 把数据层的集合树转换成纯 UI 行模型。
func buildTree(collections []*domain.Collection) []*treeNode {
	out := make([]*treeNode, 0, len(collections))
	for i, coll := range collections {
		collID := string(coll.ID)
		out = append(out, &treeNode{
			id:       collID,
			name:     coll.Name,
			kind:     treeCollection,
			collID:   collID,
			parentID: "",
			index:    i,
			children: buildNodes(collID, "", coll.Items),
		})
	}
	return out
}

func buildNodes(collID, parentID string, nodes []*domain.Node) []*treeNode {
	out := make([]*treeNode, 0, len(nodes))
	for i, n := range nodes {
		id := string(n.ID)
		row := &treeNode{
			id:       id,
			name:     n.Name,
			kind:     treeFolder,
			collID:   collID,
			parentID: parentID,
			index:    i,
		}
		if n.Kind == domain.KindRequest {
			row.kind = treeRequest
			if n.Request != nil {
				row.method = n.Request.Method
				row.url = n.Request.URL
			}
		} else {
			row.children = buildNodes(collID, id, n.Children)
		}
		out = append(out, row)
	}
	return out
}

// rowDropPosition 根据指针在目标行内的纵坐标判定放置区域（上方插入 / 移入内部 / 下方插入）。
func rowDropPosition(src treeDragItem, target *treeNode, py, h float32) dropPosition {
	if target == nil {
		return dropNone
	}
	if h <= 0 {
		h = thV2.Metrics.RowHeight
	}
	if src.kind == treeCollection {
		if target.kind != treeCollection {
			return dropNone
		}
		if py < h*0.5 {
			return dropBefore
		}
		return dropAfter
	}
	switch target.kind {
	case treeCollection:
		return dropInside
	case treeFolder:
		if py < h*0.25 {
			return dropBefore
		}
		if py > h*0.75 {
			return dropAfter
		}
		return dropInside
	case treeRequest:
		if py < h*0.5 {
			return dropBefore
		}
		return dropAfter
	default:
		return dropNone
	}
}

func isNodeSelfOrDescendant(collections []*domain.Collection, collID, ancestorID, targetID string) bool {
	if ancestorID == "" || targetID == "" {
		return false
	}
	if ancestorID == targetID {
		return true
	}
	for _, c := range collections {
		if string(c.ID) != collID {
			continue
		}
		anc := domain.FindIn(c.Items, domain.ID(ancestorID))
		if anc == nil {
			return false
		}
		return anc.Find(domain.ID(targetID)) != nil
	}
	return false
}

// canDropTreeItem 校验 src 节点是否允许以 pos 方式放置到 target 上，
// 自动拦截自身、目录循环嵌套以及不会产生任何位置变化的空操作。
func canDropTreeItem(src treeDragItem, target *treeNode, pos dropPosition, collections []*domain.Collection) bool {
	if pos == dropNone || target == nil || src.id == "" || src.id == target.id {
		return false
	}
	if src.kind == treeCollection {
		if target.kind != treeCollection || (pos != dropBefore && pos != dropAfter) {
			return false
		}
		dstIdx := target.index
		if pos == dropAfter {
			dstIdx++
		}
		return dstIdx != src.index && dstIdx != src.index+1
	}

	if src.kind == treeFolder && target.collID == src.collID && target.kind != treeCollection {
		if isNodeSelfOrDescendant(collections, src.collID, src.id, target.id) {
			return false
		}
	}

	switch pos {
	case dropInside:
		switch target.kind {
		case treeCollection:
			return !(src.collID == target.id && src.parentID == "")
		case treeFolder:
			return !(src.collID == target.collID && src.parentID == target.id)
		default:
			return false
		}
	case dropBefore, dropAfter:
		if target.kind == treeCollection {
			return false
		}
		if src.collID == target.collID && src.parentID == target.parentID {
			dstIdx := target.index
			if pos == dropAfter {
				dstIdx++
			}
			if dstIdx == src.index || dstIdx == src.index+1 {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// collectionTree 用 TreeBase 画工作区里的集合树，点击行选中并打开对应内容，
// 同时支持拖拽节点进行排序、跨目录/跨集合移动，以及通过菜单移动到指定容器。
// 全量对齐 theme/v2 统一侧栏规范：26px 行高、中性灰选中态、12px 字号与平滑过渡。
func collectionTree(c *ui.Context, state *workspace, rows []*treeNode, actions treeActions) {
	m, tk := thV2.Metrics, thV2.Of(c)
	tree := ui.TreeBase(c)
	var pendingDrop func()

	scheduleDrop := func(src treeDragItem, target treeNode, pos dropPosition) {
		pendingDrop = func() {
			if src.kind == treeCollection {
				if actions.moveCollection == nil {
					return
				}
				dstIdx := target.index
				if pos == dropAfter {
					dstIdx++
				}
				actions.moveCollection(src.id, dstIdx)
				return
			}
			if actions.move == nil {
				return
			}
			switch pos {
			case dropInside:
				if target.kind == treeCollection {
					actions.move(src.collID, src.id, target.id, "", -1)
				} else {
					actions.move(src.collID, src.id, target.collID, target.id, -1)
				}
			case dropBefore:
				actions.move(src.collID, src.id, target.collID, target.parentID, target.index)
			case dropAfter:
				actions.move(src.collID, src.id, target.collID, target.parentID, target.index+1)
			}
		}
	}

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
			row := item.Row.Height(m.RowHeight).Gap(4).
				Padding(0, 8).
				PaddingStart(4 + 16*float32(depth)).
				Radius(m.RadiusSM).
				Cursor(ui.CursorPointer).
				Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
			selected := state.selected == n.id
			menuOpen := ui.Local(row, "tree-menu-open", func() bool { return false })
			lastDropPos := ui.Local(row, "tree-last-drop-pos", func() dropPosition { return dropNone })

			canDrag := !*menuOpen && ((n.kind == treeCollection && actions.moveCollection != nil) ||
				(n.kind != treeCollection && actions.move != nil))
			if canDrag {
				row.Drag(treeDragItem{
					id:       n.id,
					collID:   n.collID,
					parentID: n.parentID,
					index:    n.index,
					kind:     n.kind,
				})
			}

			draggingSelf := row.Dragging()
			if draggingSelf {
				row.Opacity(0.45)
			}

			overItem, isDragOver := ui.DragOver[treeDragItem](row)
			droppedItem, isDropped := ui.Drop[treeDragItem](row)
			dropPos := dropNone
			if isDragOver || isDropped {
				src := overItem
				if isDropped {
					src = droppedItem
				}
				_, py, pointerOver := row.PointerPosition()
				h := row.Bounds().H
				if h <= 0 {
					h = m.RowHeight
				}
				if isDropped && !pointerOver && *lastDropPos != dropNone {
					if canDropTreeItem(src, n, *lastDropPos, actions.collections) {
						dropPos = *lastDropPos
					}
				} else {
					cand := rowDropPosition(src, n, py, h)
					if canDropTreeItem(src, n, cand, actions.collections) {
						dropPos = cand
					}
				}
			}
			if isDragOver {
				*lastDropPos = dropPos
			} else if isDropped {
				if dropPos != dropNone {
					scheduleDrop(droppedItem, *n, dropPos)
				}
				*lastDropPos = dropNone
			} else if *lastDropPos != dropNone {
				*lastDropPos = dropNone
			}

			rowHovered := row.Hovered() || *menuOpen
			switch {
			case isDragOver && dropPos == dropInside:
				row.Background(tk.AccentSoft)
			case row.Pressed(), selected:
				row.Background(tk.BgSelected)
			case rowHovered:
				row.Background(tk.BgHover)
			}

			if isDragOver && dropPos != dropNone {
				pos := dropPos
				indentX := float32(8) + 16*float32(depth)
				accent := tk.Accent
				radius := m.RadiusSM
				row.DrawOver(func(p *ui.Painter, r ui.Rect) {
					switch pos {
					case dropInside:
						p.Stroke(r, accent, radius, 1.5)
					case dropBefore:
						y := r.Y
						x0 := r.X + indentX
						if x0 > r.X+r.W-12 {
							x0 = r.X + 8
						}
						p.Fill(ui.Rect{X: x0 + 3, Y: y, W: r.X + r.W - 4 - (x0 + 3), H: 2}, accent, 1)
						p.Fill(ui.Rect{X: x0, Y: y - 1.5, W: 5, H: 5}, accent, 2.5)
					case dropAfter:
						y := r.Y + r.H - 2
						x0 := r.X + indentX
						if x0 > r.X+r.W-12 {
							x0 = r.X + 8
						}
						p.Fill(ui.Rect{X: x0 + 3, Y: y, W: r.X + r.W - 4 - (x0 + 3), H: 2}, accent, 1)
						p.Fill(ui.Rect{X: x0, Y: y - 1.5, W: 5, H: 5}, accent, 2.5)
					}
				})
			}

			var toggleClicked, moreClicked bool
			row.Children(func() {
				if expandable {
					toggleBtn := ui.ButtonBase(c.Key("tree-toggle-"+n.id)).
						Size(22, 24).Radius(m.RadiusSM).Center().
						Cursor(ui.CursorPointer).Role(ui.RoleButton).Label("Toggle " + n.name).
						Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})
					toggleHovered := toggleBtn.Hovered()
					switch {
					case toggleBtn.Pressed():
						toggleBtn.Background(tk.BgSelected)
					case toggleHovered:
						if selected || rowHovered {
							toggleBtn.Background(tk.BgSelected)
						} else {
							toggleBtn.Background(tk.BgHover)
						}
					}
					if toggleBtn.Clicked() {
						toggleClicked = true
						if open != nil {
							*open = !*open
						}
					}
					toggleBtn.Children(func() {
						arrowColor := tk.TextMuted
						if toggleHovered || toggleBtn.Pressed() {
							arrowColor = tk.TextPrimary
						}
						arrow := ui.Icon(c, icons.ChevronDown).FontSize(12).
							TextColor(arrowColor)
						if open != nil && !*open {
							arrow.Rotate(-90)
						}
					})
				} else {
					ui.Box(c).Width(22)
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
				if !rowHovered || draggingSelf {
					more.Opacity(0).IgnoreInput(true)
				}
				if more.Clicked() {
					moreClicked = true
					*menuOpen = !*menuOpen
				}
				v2.Popover(c, more, menuOpen, v2.PopoverOpts{Width: 180}, func(panel ui.Element) {
					renderTreeRowMenu(c, n, state, actions, menuOpen)
				})
			})
			if row.Clicked() && !toggleClicked && !moreClicked {
				actions.open(n)
			}
			if expandable {
				item.Group(func() {
					if len(n.children) == 0 {
						renderTreeEmptyNode(c, n, depth+1, actions, func(src treeDragItem, target treeNode) {
							scheduleDrop(src, target, dropInside)
						})
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

	if pendingDrop != nil {
		pendingDrop()
	}
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
			if n.kind == treeFolder && actions.openMoveDialog != nil && v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.ArrowRight, Key: "tm-move-" + n.id, Label: locales.TreeMoveTo(),
			}).Clicked() {
				actions.openMoveDialog(n)
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
				Icon: icons.Copy, Key: "tm-dup-" + n.id, Label: locales.CollDuplicateRequest(),
			}).Clicked() {
				actions.duplicate(n)
				*open = false
			}
			if actions.openMoveDialog != nil && v2.MenuButton(c, v2.MenuButtonProps{
				Icon: icons.ArrowRight, Key: "tm-move-" + n.id, Label: locales.TreeMoveTo(),
			}).Clicked() {
				actions.openMoveDialog(n)
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
// 柔和次级灰标题 + 简短说明 + 横排紧凑带图标小按钮，整块区域支持 Hover 背景高亮以及拖拽放入。
func renderTreeEmptyNode(
	c *ui.Context,
	n *treeNode,
	depth int,
	actions treeActions,
	onDrop func(src treeDragItem, target treeNode),
) {
	m, tk := thV2.Metrics, thV2.Of(c)
	title := locales.CollEmptyNodeTitle()
	desc := locales.CollEmptyNodeDesc()
	if n.kind == treeFolder {
		title = locales.FolderEmptyNodeTitle()
		desc = locales.FolderEmptyNodeDesc()
	}
	// 容器左外边距留出 8px 内边距后，文字与父级图标左边缘（30 + 16*(depth-1)）精确对齐
	indent := float32(22)
	if depth > 1 {
		indent += 16 * float32(depth-1)
	}

	ui.Box(c.Key("tree-empty-wrap-" + n.id)).FillWidth().Padding(2, 4, 6, indent).Children(func() {
		box := ui.Column(c.Key("tree-empty-" + n.id)).FillWidth().
			Padding(8, 8).Gap(4).AlignItems(ui.Start).
			Radius(m.RadiusMD).
			Transition(ui.ElementTransition{Colors: true, Duration: thV2.Motion.Fast})

		overItem, isDragOver := ui.DragOver[treeDragItem](box)
		droppedItem, isDropped := ui.Drop[treeDragItem](box)
		canAcceptOver := isDragOver && actions.move != nil && canDropTreeItem(overItem, n, dropInside, actions.collections)
		if isDropped && actions.move != nil && onDrop != nil && canDropTreeItem(droppedItem, n, dropInside, actions.collections) {
			onDrop(droppedItem, *n)
		}

		switch {
		case canAcceptOver:
			box.Background(tk.AccentSoft)
			accent, radius := tk.Accent, m.RadiusMD
			box.DrawOver(func(p *ui.Painter, r ui.Rect) {
				p.Stroke(r, accent, radius, 1.5)
			})
		case box.Hovered():
			box.Background(tk.BgHover)
		default:
			box.Background(ui.Transparent)
		}

		box.Children(func() {
			ui.Text(c, title).FontSize(12).FontWeight(600).TextColor(tk.TextSecondary)
			ui.Text(c, desc).FontSize(11).FixedLineHeight(16).TextColor(tk.TextMuted).FillWidth()

			ui.Row(c).FillWidth().Padding(4, 0, 0, 0).Gap(6).AlignItems(ui.Center).Wrap().Children(func() {
				treeEmptyOutlineBtn(c, "tree-empty-add-req-"+n.id, icons.Send, locales.AddRequest(), func() {
					if actions.create != nil {
						actions.create(n, treeRequest)
					}
				})
				treeEmptyOutlineBtn(c, "tree-empty-add-folder-"+n.id, icons.FolderPlus, locales.AddFolder(), func() {
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

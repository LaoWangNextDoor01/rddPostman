package collection

import "fmt"

type nodeKind int

const (
	kindCollection nodeKind = iota
	kindFolder
	kindRequest
)

type node struct {
	ID       string
	Kind     nodeKind
	Name     string
	Method   string
	URL      string
	open     bool
	dirty    bool // 有未保存修改，tab 上用圆点代替关闭按钮
	Children []*node
}

func (n *node) expandable() bool { return n.Kind != kindRequest }

// workspace 是集合页的状态：树、当前选中项和右侧打开的 tab。
type workspace struct {
	roots    []*node
	selected string
	tabs     []string
	active   string
	addOpen  bool
	seq      int
}

func newWorkspace() *workspace {
	req := func(id, method, name string) *node {
		return &node{ID: id, Kind: kindRequest, Name: name, Method: method,
			URL: "https://api.example.com/users/me"}
	}
	w := &workspace{
		roots: []*node{
			{ID: "c1", Kind: kindCollection, Name: "开发者平台 API", open: true, Children: []*node{
				{ID: "f1", Kind: kindFolder, Name: "Users", open: true, Children: []*node{
					req("r1", "POST", "登录获取令牌"),
					req("r2", "POST", "刷新 AccessToken"),
					req("r3", "DELETE", "删除会话"),
					{ID: "f1a", Kind: kindFolder, Name: "用户资料", open: true, Children: []*node{
						req("r4", "GET", "获取用户资料"),
						req("r5", "PATCH", "更新用户资料"),
						req("r6", "POST", "上传头像"),
					}},
				}},
				{ID: "f2", Kind: kindFolder, Name: "Orders", Children: []*node{
					req("r7", "GET", "获取订单"),
					req("r8", "POST", "创建订单"),
					req("r9", "DELETE", "取消订单"),
				}},
				{ID: "f3", Kind: kindFolder, Name: "Inventory", Children: []*node{
					req("r10", "GET", "库存列表"),
					req("r11", "PUT", "更新库存"),
				}},
			}},
			{ID: "c2", Kind: kindCollection, Name: "Webhooks 沙箱", Children: []*node{
				{ID: "f4", Kind: kindFolder, Name: "Events", Children: []*node{
					req("r12", "GET", "事件列表"),
					req("r13", "PATCH", "Webhook 回执"),
					req("r14", "PUT", "重放事件"),
				}},
			}},
			{ID: "c3", Kind: kindCollection, Name: "支付网关", Children: []*node{
				{ID: "f5", Kind: kindFolder, Name: "Payments", Children: []*node{
					req("r15", "GET", "支付列表"),
				}},
				{ID: "f6", Kind: kindFolder, Name: "Refunds", Children: []*node{
					req("r16", "POST", "创建退款"),
				}},
			}},
		},
		selected: "r4",
		tabs:     []string{"r4", "r8", "r11", "r3", "r13"},
		active:   "r4",
	}
	// 与设计稿一致：第二个 tab 有未保存修改。
	w.find("r8").dirty = true
	return w
}

func (w *workspace) find(id string) *node {
	var walk func(n *node) *node
	walk = func(n *node) *node {
		if n.ID == id {
			return n
		}
		for _, ch := range n.Children {
			if got := walk(ch); got != nil {
				return got
			}
		}
		return nil
	}
	for _, r := range w.roots {
		if got := walk(r); got != nil {
			return got
		}
	}
	return nil
}

// openTab 打开或激活一个节点的 tab，并同步树上的选中项。
func (w *workspace) openTab(id string) {
	exists := false
	for _, t := range w.tabs {
		if t == id {
			exists = true
			break
		}
	}
	if !exists {
		w.tabs = append(w.tabs, id)
	}
	w.active, w.selected = id, id
}

// containerFor 返回新节点应放入的父容器。
func (w *workspace) containerFor(id string) *node {
	var walk func(n *node) *node
	walk = func(n *node) *node {
		if n.ID == id {
			if n.Kind == kindRequest {
				return nil // 由上一层作为父节点
			}
			return n
		}
		for _, ch := range n.Children {
			if ch.ID == id {
				return n
			}
			if got := walk(ch); got != nil {
				return got
			}
		}
		return nil
	}
	for _, r := range w.roots {
		if got := walk(r); got != nil {
			return got
		}
	}
	return nil
}

func (w *workspace) firstCollection() *node {
	for _, r := range w.roots {
		if r.Kind == kindCollection {
			return r
		}
	}
	return nil
}

func (w *workspace) create(kind nodeKind) *node {
	w.seq++
	n := &node{ID: fmt.Sprintf("new%d", w.seq), Kind: kind}
	switch kind {
	case kindCollection:
		n.Name, n.open = "新建集合", true
		w.roots = append(w.roots, n)
	case kindFolder:
		n.Name, n.open = "新建目录", true
	case kindRequest:
		n.Name, n.Method, n.URL = "新建请求", "GET", "https://api.example.com"
	}

	if kind != kindCollection {
		parent := w.containerFor(w.selected)
		if parent == nil {
			parent = w.firstCollection()
		}
		if parent != nil {
			parent.Children = append(parent.Children, n)
			parent.open = true
		} else {
			w.roots = append(w.roots, n)
		}
	}

	w.openTab(n.ID)
	return n
}

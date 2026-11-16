package collection

import "rddpostman/internal/domain"

// workspace 是集合页的会话状态：打开的 tab、树上选中项与弹层开关。
// 集合与请求本身来自 store 快照，这里不保存业务数据。
type workspace struct {
	tabs        []string
	selected    string
	active      string
	addOpen     bool
	newCollOpen bool
	newCollName string
	newCollDesc string
	newCollErr  string
	open        map[string]*bool  // 目录展开状态，缺省展开
	segs        map[string]string // 每个 tab 当前选中的分段
}

func newWorkspace() *workspace {
	return &workspace{open: map[string]*bool{}, segs: map[string]string{}}
}

// segment 返回 tab 当前分段，缺省是概览。
func (w *workspace) segment(id string) string {
	if s, ok := w.segs[id]; ok {
		return s
	}
	return segOverview
}

func (w *workspace) setSegment(id, seg string) { w.segs[id] = seg }

// openFlag 返回一个稳定的展开开关，供 TreeBase 渲染与切换。
func (w *workspace) openFlag(id string) *bool {
	if f, ok := w.open[id]; ok {
		return f
	}
	f := new(bool)
	*f = true
	w.open[id] = f
	return f
}

// openTab 打开或激活一个请求 tab。
func (w *workspace) openTab(id string) {
	for _, t := range w.tabs {
		if t == id {
			w.active, w.selected = id, id
			return
		}
	}
	w.tabs = append(w.tabs, id)
	w.active, w.selected = id, id
}

// pruneTabs 清掉已经不存在的 tab，并把当前 tab/选中项调整到有效值。
// 删除集合、目录或请求后调用。
func (w *workspace) pruneTabs(collections []*domain.Collection) {
	exists := func(id string) bool {
		for _, coll := range collections {
			if string(coll.ID) == id {
				return true
			}
			if domain.FindIn(coll.Items, domain.ID(id)) != nil {
				return true
			}
		}
		return false
	}
	keep := w.tabs[:0]
	for _, id := range w.tabs {
		if exists(id) {
			keep = append(keep, id)
		}
	}
	w.tabs = keep
	if !exists(w.active) {
		if len(keep) == 0 {
			w.active = ""
		} else {
			w.active = keep[len(keep)-1]
		}
	}
	if !exists(w.selected) {
		w.selected = w.active
	}
}

// closeTab 关闭 tab，并把相邻的 tab 设为当前。
func (w *workspace) closeTab(id string) {
	for i, t := range w.tabs {
		if t != id {
			continue
		}
		w.tabs = append(w.tabs[:i], w.tabs[i+1:]...)
		break
	}
	if w.active != id {
		return
	}
	if len(w.tabs) == 0 {
		w.active, w.selected = "", ""
		return
	}
	w.active = w.tabs[len(w.tabs)-1]
	w.selected = w.active
}

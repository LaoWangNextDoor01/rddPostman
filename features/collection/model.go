package collection

import (
	"strings"

	"rddpostman/internal/domain"
)

const (
	envTabPrefix  = "env:"
	toolTabPrefix = "tool:"
)

// EnvTabID 返回环境变量编辑 Tab 的唯一标识。
func EnvTabID(envID domain.ID) string { return envTabPrefix + string(envID) }

func parseEnvTabID(id string) (domain.ID, bool) {
	if raw, ok := strings.CutPrefix(id, envTabPrefix); ok && raw != "" {
		return domain.ID(raw), true
	}
	return "", false
}

// ToolTabID 返回开发者小工具 Tab 的唯一标识。
func ToolTabID(toolID string) string { return toolTabPrefix + toolID }

func parseToolTabID(id string) (string, bool) {
	if raw, ok := strings.CutPrefix(id, toolTabPrefix); ok && raw != "" {
		return raw, true
	}
	return "", false
}

// draftNode 维护尚未持久化到 store 的临时草稿请求。
type draftNode struct {
	collID   domain.ID
	parentID domain.ID
	node     *domain.Node
}

// workspace 是集合页的会话状态：打开的 tab、树上选中项与弹层开关。
// 集合与请求本身来自 store 快照，这里不保存业务数据。
type workspace struct {
	tabs           []string
	selected       string
	active         string
	addOpen        bool
	focusNewCollID string // 新建集合后在 Tab 内自动进入标题行内编辑态
	focusURLReq    bool   // 快捷键 Cmd/Ctrl+L 触发聚焦当前请求 URL 输入框

	drafts  map[string]*draftNode // 未持久化的草稿节点：tabID -> draftNode
	unsaved map[string]bool       // 标记 tab 是否存在未保存变更：tabID -> bool

	importCurlOpen    bool
	importCurlText    string
	importCurlErr     string
	importPostmanOpen bool
	importPostmanText string
	importPostmanErr  string

	deleteConfirmOpen bool
	deletePending     *treeNode

	sidebarFilter map[string]bool // 侧栏搜索筛选勾选状态（缺省全选）
	open          map[string]*bool  // 目录展开状态，缺省展开
	segs          map[string]string // 每个 tab 当前选中的分段
}

var sidebarFilterCategories = []string{
	"Collections",
	"Environments",
	"Documents",
	"Specs",
	"Mocks",
	"Datasets",
	"Toolsets",
	"Flows",
}

func newWorkspace() *workspace {
	filters := make(map[string]bool, len(sidebarFilterCategories))
	for _, cat := range sidebarFilterCategories {
		filters[cat] = true
	}
	return &workspace{
		drafts:        map[string]*draftNode{},
		unsaved:       map[string]bool{},
		sidebarFilter: filters,
		open:          map[string]*bool{},
		segs:          map[string]string{},
	}
}

func (w *workspace) isUnsaved(id string) bool {
	if w == nil || w.unsaved == nil {
		return false
	}
	return w.unsaved[id]
}

func (w *workspace) setUnsaved(id string, unsaved bool) {
	if w == nil {
		return
	}
	if w.unsaved == nil {
		w.unsaved = make(map[string]bool)
	}
	if unsaved {
		w.unsaved[id] = true
	} else {
		delete(w.unsaved, id)
	}
}

func (w *workspace) isDraft(id string) bool {
	if w == nil || w.drafts == nil {
		return false
	}
	_, ok := w.drafts[id]
	return ok
}

func (w *workspace) addDraft(collID, parentID domain.ID, node *domain.Node) {
	if w == nil {
		return
	}
	if w.drafts == nil {
		w.drafts = make(map[string]*draftNode)
	}
	id := string(node.ID)
	w.drafts[id] = &draftNode{
		collID:   collID,
		parentID: parentID,
		node:     node,
	}
	w.setUnsaved(id, true)
	w.openTab(id)
}

func (w *workspace) removeDraft(id string) {
	if w == nil {
		return
	}
	if w.drafts != nil {
		delete(w.drafts, id)
	}
	if w.unsaved != nil {
		delete(w.unsaved, id)
	}
}

func (w *workspace) filterEnabled(cat string) bool {
	if len(w.sidebarFilter) == 0 {
		return true
	}
	v, ok := w.sidebarFilter[cat]
	if !ok {
		return true
	}
	return v
}

func (w *workspace) allFiltersEnabled() bool {
	for _, cat := range sidebarFilterCategories {
		if !w.filterEnabled(cat) {
			return false
		}
	}
	return true
}

func (w *workspace) setAllFilters(on bool) {
	if w.sidebarFilter == nil {
		w.sidebarFilter = make(map[string]bool, len(sidebarFilterCategories))
	}
	for _, cat := range sidebarFilterCategories {
		w.sidebarFilter[cat] = on
	}
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

// pruneTabs 清掉已经不存在的集合/请求 tab（保留环境 tab 与未保存草稿），并把当前 tab/选中项调整到有效值。
// 删除集合、目录或请求后调用。
func (w *workspace) pruneTabs(collections []*domain.Collection) {
	exists := func(id string) bool {
		if _, isEnv := parseEnvTabID(id); isEnv {
			return true
		}
		if _, isTool := parseToolTabID(id); isTool {
			return true
		}
		if _, isDraft := w.drafts[id]; isDraft {
			return true
		}
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
	w.pruneWith(exists)
}

// pruneMissingEnvs 清理已被删除的环境对应的 tab。
func (w *workspace) pruneMissingEnvs(environments []domain.Environment) {
	if len(w.tabs) == 0 {
		return
	}
	envSet := make(map[domain.ID]struct{}, len(environments))
	for _, e := range environments {
		envSet[e.ID] = struct{}{}
	}
	w.pruneWith(func(id string) bool {
		if envID, isEnv := parseEnvTabID(id); isEnv {
			_, ok := envSet[envID]
			return ok
		}
		return true
	})
}

func (w *workspace) pruneWith(exists func(id string) bool) {
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
	if w.isDraft(id) {
		w.removeDraft(id)
	}
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

// nextTab 切换到下一个打开的 Tab（循环）。
func (w *workspace) nextTab() {
	if len(w.tabs) <= 1 {
		return
	}
	for i, t := range w.tabs {
		if t == w.active {
			next := w.tabs[(i+1)%len(w.tabs)]
			w.active, w.selected = next, next
			return
		}
	}
	w.active, w.selected = w.tabs[0], w.tabs[0]
}

// prevTab 切换到上一个打开的 Tab（循环）。
func (w *workspace) prevTab() {
	if len(w.tabs) <= 1 {
		return
	}
	for i, t := range w.tabs {
		if t == w.active {
			prev := w.tabs[(i-1+len(w.tabs))%len(w.tabs)]
			w.active, w.selected = prev, prev
			return
		}
	}
	w.active, w.selected = w.tabs[len(w.tabs)-1], w.tabs[len(w.tabs)-1]
}

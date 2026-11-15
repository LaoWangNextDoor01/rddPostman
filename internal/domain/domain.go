// Package domain 定义纯数据模型与纯函数（集合、请求、环境、响应）。
// 不依赖 ui、网络与文件系统，便于直接单元测试。
package domain

import (
	"sort"
	"strings"
	"time"
)

// ID 是集合与节点的稳定标识，创建后不再变化。
type ID string

// Kind 区分集合树里的一行是目录还是请求。
type Kind string

const (
	KindFolder  Kind = "folder"
	KindRequest Kind = "request"
)

// Workspace 是集合之上的一层分组。
// 默认工作区由系统自动创建且不可删除；用户可另外创建多个。
type Workspace struct {
	ID      ID     `json:"id"`
	Name    string `json:"name"`
	Default bool   `json:"default,omitempty"`
}

// Collection 是一个集合文件的全部内容，对应磁盘上的 <名称>.collection.json。
type Collection struct {
	Version int     `json:"version"`
	ID      ID      `json:"id"`
	Name    string  `json:"name"`
	Items   []*Node `json:"items"`
}

// Node 是集合树节点：目录带 Children，请求带 Request。
type Node struct {
	ID       ID       `json:"id"`
	Kind     Kind     `json:"kind"`
	Name     string   `json:"name"`
	Children []*Node  `json:"children,omitempty"`
	Request  *Request `json:"request,omitempty"`
}

// Request 是一个请求的配置。
type Request struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Params  []KV   `json:"params,omitempty"`
	Headers []KV   `json:"headers,omitempty"`
	Body    string `json:"body,omitempty"`
	Auth    string `json:"auth,omitempty"` // none、bearer、basic
}

// KV 是参数、请求头、变量共用的键值对。
// Secret 为 true 时 Value 不写进集合文件，而是存在应用数据目录。
type KV struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Enabled bool   `json:"enabled"`
	Secret  bool   `json:"secret,omitempty"`
}

// Environment 是一组可切换的环境变量。
type Environment struct {
	ID   ID     `json:"id"`
	Name string `json:"name"`
	Vars []KV   `json:"vars"`
}

// Response 是一次请求的结果，只留在内存里，不落盘。
type Response struct {
	Status   int
	Headers  []KV
	Body     []byte
	Duration time.Duration
	At       time.Time
}

// HistoryEntry 是运行历史里的一条摘要。
type HistoryEntry struct {
	ID           ID        `json:"id"`
	CollectionID ID        `json:"collectionID"`
	RequestID    ID        `json:"requestID"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	Status       int       `json:"status"`
	DurationMS   int       `json:"durationMS"`
	At           time.Time `json:"at"`
}

// Clone 深拷贝集合，供写时复制的状态更新使用。
func (c *Collection) Clone() *Collection {
	if c == nil {
		return nil
	}
	out := *c
	out.Items = cloneNodes(c.Items)
	return &out
}

// Clone 深拷贝节点及其子树。
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	out := *n
	out.Children = cloneNodes(n.Children)
	out.Request = n.Request.Clone()
	return &out
}

// Clone 深拷贝请求。
func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}
	out := *r
	out.Params = append([]KV(nil), r.Params...)
	out.Headers = append([]KV(nil), r.Headers...)
	return &out
}

func cloneNodes(ns []*Node) []*Node {
	if ns == nil {
		return nil
	}
	out := make([]*Node, len(ns))
	for i, n := range ns {
		out[i] = n.Clone()
	}
	return out
}

// Find 在子树里按 ID 找节点，包含 n 自己。
func (n *Node) Find(id ID) *Node {
	if n == nil {
		return nil
	}
	if n.ID == id {
		return n
	}
	for _, ch := range n.Children {
		if got := ch.Find(id); got != nil {
			return got
		}
	}
	return nil
}

// FindIn 在整棵树里找节点。
func FindIn(items []*Node, id ID) *Node {
	for _, n := range items {
		if got := n.Find(id); got != nil {
			return got
		}
	}
	return nil
}

// RemoveChild 从子树里删除 id 节点，返回新切片和是否删掉了。
// 不修改传入的节点，沿途父节点会复制一份。
func RemoveChild(items []*Node, id ID) ([]*Node, bool) {
	for i, n := range items {
		if n.ID == id {
			out := make([]*Node, 0, len(items)-1)
			out = append(out, items[:i]...)
			out = append(out, items[i+1:]...)
			return out, true
		}
		if len(n.Children) > 0 {
			if rest, ok := RemoveChild(n.Children, id); ok {
				cp := *n
				cp.Children = rest
				out := append([]*Node(nil), items...)
				out[i] = &cp
				return out, true
			}
		}
	}
	return items, false
}

// ParentOf 返回 id 的父节点；id 在根层时返回 nil 和 true。
func ParentOf(items []*Node, id ID) (*Node, bool) {
	for _, n := range items {
		for _, ch := range n.Children {
			if ch.ID == id {
				return n, true
			}
		}
		if len(n.Children) > 0 {
			if p, ok := ParentOf(n.Children, id); ok {
				return p, true
			}
		}
	}
	for _, n := range items {
		if n.ID == id {
			return nil, true
		}
	}
	return nil, false
}

// CountRequests 统计子树里的请求数量。
func CountRequests(items []*Node) int {
	total := 0
	for _, n := range items {
		if n.Kind == KindRequest {
			total++
			continue
		}
		total += CountRequests(n.Children)
	}
	return total
}

// Prepare 把 URL、参数、请求头、Body 里的 {{变量}} 替换成环境值，
// 返回替换后的请求与缺失的变量名（去重并按名字排序）。
func Prepare(r Request, env Environment) (Request, []string) {
	vals := make(map[string]string, len(env.Vars))
	for _, v := range env.Vars {
		if v.Enabled {
			vals[v.Key] = v.Value
		}
	}
	missing := map[string]bool{}
	sub := func(s string) string { return substitute(s, vals, missing) }

	out := *r.Clone()
	out.URL = sub(r.URL)
	out.Body = sub(r.Body)
	for i := range out.Params {
		out.Params[i].Value = sub(out.Params[i].Value)
	}
	for i := range out.Headers {
		out.Headers[i].Value = sub(out.Headers[i].Value)
	}

	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return out, names
}

// substitute 替换 s 中所有 {{name}}；未知变量记进 missing 并原样保留。
func substitute(s string, vals map[string]string, missing map[string]bool) string {
	var b strings.Builder
	for {
		start := strings.Index(s, "{{")
		if start < 0 {
			b.WriteString(s)
			break
		}
		end := strings.Index(s[start+2:], "}}")
		if end < 0 {
			b.WriteString(s)
			break
		}
		b.WriteString(s[:start])
		name := strings.TrimSpace(s[start+2 : start+2+end])
		if v, ok := vals[name]; ok {
			b.WriteString(v)
		} else {
			missing[name] = true
			b.WriteString(s[start : start+2+end+2])
		}
		s = s[start+2+end+2:]
	}
	return b.String()
}

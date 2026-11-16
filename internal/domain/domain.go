// Package domain 定义纯数据模型与纯函数（集合、请求、环境、变量合并）。
// 不依赖 ui、网络与文件系统，便于直接单元测试与后续替换存储实现。
package domain

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"time"
)

// ID 是工作区、集合与节点的稳定标识，创建后不再变化。
// 节点 ID 与文件名解耦：改名只改文件，ID 仍然稳定。
type ID string

// Kind 区分集合树里的一行是目录还是请求。
type Kind string

const (
	KindFolder  Kind = "folder"
	KindRequest Kind = "request"
)

// 文件格式标识与版本。每个持久化文件都自带 schema + version，
// 这样迁移可以按文件独立进行，也便于将来接入 SQLite / 远程存储。
const (
	SchemaRegistry    = "rddpostman.registry"
	SchemaWorkspace   = "rddpostman.workspace"
	SchemaCollection  = "rddpostman.collection"
	SchemaFolder      = "rddpostman.folder"
	SchemaRequest     = "rddpostman.request"
	SchemaEnvironment = "rddpostman.environment"
	SchemaLocal       = "rddpostman.local"
	SchemaSecrets     = "rddpostman.secrets"
	SchemaGlobals     = "rddpostman.globals"

	// DefaultEnvironmentName 是工作区初始默认环境的名称。
	DefaultEnvironmentName = "默认环境"

	// Version 是当前写入的格式版本。
	Version = 3
)

// Workspace 是一个工作区的元数据，对应磁盘上的 workspace.json。
// Default 来自注册表，不写进 workspace.json，避免复制目录后出现两个默认工作区。
type Workspace struct {
	Schema               string    `json:"schema"`
	Version              int       `json:"version"`
	ID                   ID        `json:"id"`
	Name                 string    `json:"name"`
	Description          string    `json:"description,omitempty"`
	Variables            []KV      `json:"variables,omitempty"`
	DefaultEnvironmentID ID        `json:"defaultEnvironmentID,omitempty"`
	Default              bool      `json:"-"`
	CreatedAt            time.Time `json:"createdAt,omitzero"`
	UpdatedAt            time.Time `json:"updatedAt,omitzero"`
}

// Collection 是一个集合的元数据，对应 collection.json。
// 子节点不写进这个文件，由 storage 扫描目录得到。
type Collection struct {
	Schema      string    `json:"schema"`
	Version     int       `json:"version"`
	ID          ID        `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Variables   []KV      `json:"variables,omitempty"`
	Auth        Auth      `json:"auth,omitzero"`
	Order       int64     `json:"order,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitzero"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero"`
	Items       []*Node   `json:"-"`
}

// Node 是集合树节点。目录对应一个真实目录 + folder.json，
// 请求对应 <名称>.request.json，Kind 由磁盘上的形态决定。
type Node struct {
	Schema      string    `json:"schema,omitempty"`
	Version     int       `json:"version,omitempty"`
	ID          ID        `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Variables   []KV      `json:"variables,omitempty"`
	Auth        Auth      `json:"auth,omitzero"`
	Order       int64     `json:"order,omitempty"`
	Request     *Request  `json:"request,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitzero"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero"`

	Kind     Kind    `json:"-"`
	Children []*Node `json:"-"`
}

// Request 是一个请求的传输配置。认证放在 Node.Auth 上，
// 以便目录级继承（request → folder → collection）。
type Request struct {
	Protocol      string    `json:"protocol,omitempty"`
	Method        string    `json:"method,omitempty"`
	URL           string    `json:"url,omitempty"`
	PathVariables []KV      `json:"pathVariables,omitempty"`
	Query         []KV      `json:"query,omitempty"`
	Headers       []KV      `json:"headers,omitempty"`
	Cookies       []KV      `json:"cookies,omitempty"`
	Body          Body      `json:"body,omitzero"`
	Options       Options   `json:"options,omitzero"`
	CreatedAt     time.Time `json:"createdAt,omitzero"`
	UpdatedAt     time.Time `json:"updatedAt,omitzero"`
}

// BodyMode 是请求体的类型。
type BodyMode string

const (
	BodyNone       BodyMode = "none"
	BodyRaw        BodyMode = "raw"
	BodyURLEncoded BodyMode = "urlencoded"
	BodyFormData   BodyMode = "form-data"
	BodyBinary     BodyMode = "binary"
	BodyGraphQL    BodyMode = "graphql"
)

// Body 是请求体配置。Raw 用 Language 区分 json / xml / text。
type Body struct {
	Mode     BodyMode `json:"mode,omitempty"`
	Raw      string   `json:"raw,omitempty"`
	Language string   `json:"language,omitempty"`
	Fields   []KV     `json:"fields,omitempty"`
	File     string   `json:"file,omitempty"`
	GraphQL  *GraphQL `json:"graphql,omitempty"`
}

// GraphQL 是 GraphQL 请求的查询与变量。
type GraphQL struct {
	Query     string `json:"query,omitempty"`
	Variables string `json:"variables,omitempty"`
}

// Options 是单请求级传输选项。指针字段用于区分「继承工作区默认」和「显式关闭」。
type Options struct {
	TimeoutMS       int   `json:"timeoutMS,omitempty"`
	FollowRedirects *bool `json:"followRedirects,omitempty"`
	VerifyTLS       *bool `json:"verifyTLS,omitempty"`
}

// AuthType 是认证方式。
type AuthType string

const (
	AuthInherit AuthType = "inherit"
	AuthNone    AuthType = "none"
	AuthBearer  AuthType = "bearer"
	AuthBasic   AuthType = "basic"
	AuthAPIKey  AuthType = "apikey"
	AuthOAuth2  AuthType = "oauth2"
)

// Auth 是认证配置。Type 为空表示继承父级。
type Auth struct {
	Type   AuthType `json:"type,omitempty"`
	Params []KV     `json:"params,omitempty"`
}

// KV 是参数、请求头、变量共用的键值对。
// ID 用于列表行状态与 secret 索引，Secret 为 true 时真实值不写进业务文件。
// 环境变量的本机覆盖值存在 Local.CurrentValues，按 ID 索引。
type KV struct {
	ID        ID        `json:"id,omitempty"`
	Key       string    `json:"key"`
	Value     string    `json:"value,omitempty"`
	Desc      string    `json:"desc,omitempty"`
	Enabled   bool      `json:"enabled"`
	Secret    bool      `json:"secret,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
}

// Environment 是一组可切换的环境变量，对应 environments/<名称>.environment.json。
type Environment struct {
	Schema      string    `json:"schema"`
	Version     int       `json:"version"`
	ID          ID        `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Vars        []KV      `json:"vars,omitempty"`
	CreatedAt   time.Time `json:"createdAt,omitzero"`
	UpdatedAt   time.Time `json:"updatedAt,omitzero"`
}

// Local 是工作区的本机状态，对应 local.json，不参与 Git 同步。
type Local struct {
	Schema                 string        `json:"schema"`
	Version                int           `json:"version"`
	SelectedEnvironmentID  ID            `json:"selectedEnvironmentID,omitempty"`
	CollectionEnvironments map[ID]ID     `json:"collectionEnvironments,omitempty"`
	CurrentValues          map[ID]string `json:"currentValues,omitempty"`
	CreatedAt              time.Time     `json:"createdAt,omitzero"`
	UpdatedAt              time.Time     `json:"updatedAt,omitzero"`
}

// Secrets 保存 secret 变量的真实值，按 KV.ID 索引，对应 secrets.json（0600）。
type Secrets struct {
	Schema    string        `json:"schema"`
	Version   int           `json:"version"`
	Values    map[ID]string `json:"values,omitempty"`
	CreatedAt time.Time     `json:"createdAt,omitzero"`
	UpdatedAt time.Time     `json:"updatedAt,omitzero"`
}

// Globals 是跨工作区的全局变量，对应数据根目录下的 globals.json。
type Globals struct {
	Schema    string    `json:"schema"`
	Version   int       `json:"version"`
	Variables []KV      `json:"variables,omitempty"`
	CreatedAt time.Time `json:"createdAt,omitzero"`
	UpdatedAt time.Time `json:"updatedAt,omitzero"`
}

// Response 是一次请求的结果，只留在内存里，不落盘。
type Response struct {
	Status   int
	Headers  []KV
	Body     []byte
	Duration time.Duration
	At       time.Time
}

// HistoryEntry 是运行历史里的一条摘要，供历史页、统计与文档导出派生使用。
type HistoryEntry struct {
	ID           ID        `json:"id"`
	WorkspaceID  ID        `json:"workspaceID"`
	CollectionID ID        `json:"collectionID"`
	RequestID    ID        `json:"requestID"`
	Method       string    `json:"method"`
	URL          string    `json:"url"`
	Status       int       `json:"status"`
	DurationMS   int       `json:"durationMS"`
	Size         int64     `json:"size"`
	BodyRef      string    `json:"bodyRef,omitempty"`
	Error        string    `json:"error,omitempty"`
	At           time.Time `json:"at"`
	CreatedAt    time.Time `json:"createdAt,omitzero"`
}

// NewID 生成稳定的随机 ID。
func NewID() ID {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("domain: 生成 ID 失败: %v", err))
	}
	return ID(hex.EncodeToString(b))
}

// Clone 深拷贝工作区。
func (w *Workspace) Clone() *Workspace {
	if w == nil {
		return nil
	}
	out := *w
	out.Variables = append([]KV(nil), w.Variables...)
	return &out
}

// Clone 深拷贝集合。
func (c *Collection) Clone() *Collection {
	if c == nil {
		return nil
	}
	out := *c
	out.Variables = append([]KV(nil), c.Variables...)
	out.Auth = c.Auth.Clone()
	out.Items = cloneNodes(c.Items)
	return &out
}

// Clone 深拷贝节点及其子树。
func (n *Node) Clone() *Node {
	if n == nil {
		return nil
	}
	out := *n
	out.Variables = append([]KV(nil), n.Variables...)
	out.Auth = n.Auth.Clone()
	out.Request = n.Request.Clone()
	out.Children = cloneNodes(n.Children)
	return &out
}

// Clone 深拷贝请求。
func (r *Request) Clone() *Request {
	if r == nil {
		return nil
	}
	out := *r
	out.PathVariables = append([]KV(nil), r.PathVariables...)
	out.Query = append([]KV(nil), r.Query...)
	out.Headers = append([]KV(nil), r.Headers...)
	out.Cookies = append([]KV(nil), r.Cookies...)
	out.Body.Fields = append([]KV(nil), r.Body.Fields...)
	if r.Body.GraphQL != nil {
		g := *r.Body.GraphQL
		out.Body.GraphQL = &g
	}
	if r.Options.FollowRedirects != nil {
		v := *r.Options.FollowRedirects
		out.Options.FollowRedirects = &v
	}
	if r.Options.VerifyTLS != nil {
		v := *r.Options.VerifyTLS
		out.Options.VerifyTLS = &v
	}
	return &out
}

// Clone 深拷贝认证配置。
func (a Auth) Clone() Auth {
	out := a
	out.Params = append([]KV(nil), a.Params...)
	return out
}

// Clone 深拷贝环境。
func (e Environment) Clone() Environment {
	out := e
	out.Vars = append([]KV(nil), e.Vars...)
	return out
}

// Clone 深拷贝本机状态。
func (l Local) Clone() Local {
	out := l
	if l.CollectionEnvironments != nil {
		out.CollectionEnvironments = make(map[ID]ID, len(l.CollectionEnvironments))
		for k, v := range l.CollectionEnvironments {
			out.CollectionEnvironments[k] = v
		}
	}
	if l.CurrentValues != nil {
		out.CurrentValues = make(map[ID]string, len(l.CurrentValues))
		for k, v := range l.CurrentValues {
			out.CurrentValues[k] = v
		}
	}
	return out
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

// FindPath 返回从根到 id 的节点路径（包含 id 自己）。
func FindPath(items []*Node, id ID) ([]*Node, bool) {
	for _, n := range items {
		if n.ID == id {
			return []*Node{n}, true
		}
		if p, ok := FindPath(n.Children, id); ok {
			return append([]*Node{n}, p...), true
		}
	}
	return nil, false
}

// AncestorFolders 返回 id 的祖先目录，从外到内，不包含自身。
// 用于构造目录级变量与认证的继承链。
func AncestorFolders(items []*Node, id ID) []*Node {
	path, ok := FindPath(items, id)
	if !ok || len(path) <= 1 {
		return nil
	}
	out := make([]*Node, 0, len(path)-1)
	for _, n := range path[:len(path)-1] {
		if n.Kind == KindFolder {
			out = append(out, n)
		}
	}
	return out
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

// NextOrder 返回同一容器里追加节点的顺序值。
// 用固定间隔留出插入空间，重排时取中点即可，避免批量重写。
func NextOrder(items []*Node) int64 {
	var max int64
	for _, n := range items {
		if n.Order > max {
			max = n.Order
		}
	}
	if max == 0 {
		return 1024
	}
	return max + 1024
}

// ResolveAuth 沿继承链（从外到内）返回最终生效的认证配置。
func ResolveAuth(chain []Auth) Auth {
	var out Auth
	for _, a := range chain {
		switch a.Type {
		case "", AuthInherit:
			continue
		default:
			out = a
		}
	}
	if out.Type == "" {
		return Auth{Type: AuthNone}
	}
	return out
}

// VarSource 标记变量来自哪一层，便于 UI 展示取值来源。
type VarSource string

const (
	VarGlobal      VarSource = "global"
	VarWorkspace   VarSource = "workspace"
	VarEnvironment VarSource = "environment"
	VarCollection  VarSource = "collection"
	VarFolder      VarSource = "folder"
	VarRequest     VarSource = "request"
	VarRuntime     VarSource = "runtime"
)

// Variable 是合并后的变量。
type Variable struct {
	Key    string
	Value  string
	Source VarSource
	Secret bool
}

// Layer 是一层变量。Current 存本机覆盖值，Secrets 存 secret 真实值，均按 KV.ID 索引。
type Layer struct {
	Source  VarSource
	Vars    []KV
	Current map[ID]string
	Secrets map[ID]string
}

// Merge 按从低到高的顺序合并变量层，同名后者覆盖前者。
// 禁用项与空名字直接跳过；空值算已定义，会覆盖低层的值。
func Merge(layers []Layer) map[string]Variable {
	out := make(map[string]Variable)
	for _, layer := range layers {
		for _, kv := range layer.Vars {
			if !kv.Enabled {
				continue
			}
			key := strings.TrimSpace(kv.Key)
			if key == "" {
				continue
			}
			value := kv.Value
			if kv.ID != "" {
				if v, ok := layer.Current[kv.ID]; ok {
					value = v
				}
				if v, ok := layer.Secrets[kv.ID]; ok {
					value = v
				}
			}
			out[key] = Variable{Key: key, Value: value, Source: layer.Source, Secret: kv.Secret}
		}
	}
	return out
}

// Prepare 把请求里的 {{变量}} 替换成合并后的值，
// 返回替换后的请求与缺失的变量名（去重并按名字排序）。
func Prepare(r Request, vars map[string]Variable) (Request, []string) {
	missing := map[string]bool{}
	sub := func(s string) string { return substitute(s, vars, missing) }

	out := *r.Clone()
	out.URL = sub(out.URL)
	out.Body.Raw = sub(out.Body.Raw)
	out.Body.File = sub(out.Body.File)
	if out.Body.GraphQL != nil {
		out.Body.GraphQL.Query = sub(out.Body.GraphQL.Query)
		out.Body.GraphQL.Variables = sub(out.Body.GraphQL.Variables)
	}
	subKVs := func(kvs []KV) {
		for i := range kvs {
			kvs[i].Value = sub(kvs[i].Value)
		}
	}
	subKVs(out.PathVariables)
	subKVs(out.Query)
	subKVs(out.Headers)
	subKVs(out.Cookies)
	subKVs(out.Body.Fields)

	return out, missingNames(missing)
}

// PrepareAuth 替换认证参数里的变量。
func PrepareAuth(a Auth, vars map[string]Variable) (Auth, []string) {
	missing := map[string]bool{}
	out := a.Clone()
	for i := range out.Params {
		out.Params[i].Value = substitute(out.Params[i].Value, vars, missing)
	}
	return out, missingNames(missing)
}

func missingNames(missing map[string]bool) []string {
	names := make([]string, 0, len(missing))
	for name := range missing {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// substitute 替换 s 中所有 {{name}}；未知变量记进 missing 并原样保留。
// {{$guid}}、{{$timestamp}} 等以 $ 开头的动态变量在发送时求值。
func substitute(s string, vars map[string]Variable, missing map[string]bool) string {
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
		if v, ok := vars[name]; ok {
			b.WriteString(v.Value)
		} else if v, ok := dynamicValue(name); ok {
			b.WriteString(v)
		} else {
			missing[name] = true
			b.WriteString(s[start : start+2+end+2])
		}
		s = s[start+2+end+2:]
	}
	return b.String()
}

// dynamicValue 返回内置动态变量的当前值。
func dynamicValue(name string) (string, bool) {
	switch name {
	case "$guid", "$uuid":
		return string(NewID()) + string(NewID()), true
	case "$timestamp":
		return fmt.Sprintf("%d", time.Now().Unix()), true
	case "$isoTimestamp":
		return time.Now().UTC().Format(time.RFC3339), true
	case "$randomInt":
		b := make([]byte, 2)
		if _, err := rand.Read(b); err != nil {
			return "0", true
		}
		return fmt.Sprintf("%d", int(b[0])<<8|int(b[1])), true
	default:
		return "", false
	}
}

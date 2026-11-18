package domain

import (
	"reflect"
	"strings"
	"testing"
)

func TestPrepare(t *testing.T) {
	vars := Merge([]Layer{{Source: VarEnvironment, Vars: []KV{
		{ID: "v1", Key: "baseUrl", Value: "https://api.example.com", Enabled: true},
		{ID: "v2", Key: "tenant", Value: "acme", Enabled: false},
	}}})
	req := Request{
		Method:  "GET",
		URL:     "{{baseUrl}}/users/{{userId}}",
		Headers: []KV{{Key: "X-Tenant", Value: "{{tenant}}", Enabled: true}},
	}

	got, missing := Prepare(req, vars)
	if want := "https://api.example.com/users/{{userId}}"; got.URL != want {
		t.Errorf("URL = %q，想要 %q", got.URL, want)
	}
	// tenant 被禁用，等于没定义。
	if want := []string{"tenant", "userId"}; !reflect.DeepEqual(missing, want) {
		t.Errorf("missing = %v，想要 %v", missing, want)
	}
	if got.Headers[0].Value != "{{tenant}}" {
		t.Errorf("未知变量应原样保留，得到 %q", got.Headers[0].Value)
	}
	if req.URL != "{{baseUrl}}/users/{{userId}}" {
		t.Errorf("Prepare 不应改动入参，得到 %q", req.URL)
	}
}

func TestPrepareNoVars(t *testing.T) {
	req := Request{URL: "https://example.com/a?b=c{"}
	got, missing := Prepare(req, nil)
	if got.URL != req.URL || len(missing) != 0 {
		t.Errorf("无变量时应原样返回，得到 %q %v", got.URL, missing)
	}
}

func TestPrepareBodyAndQuery(t *testing.T) {
	vars := Merge([]Layer{{Source: VarEnvironment, Vars: []KV{
		{Key: "host", Value: "example.com", Enabled: true},
		{Key: "token", Value: "t-1", Enabled: true},
	}}})
	req := Request{
		URL:   "https://{{host}}/v1",
		Query: []KV{{Key: "q", Value: "{{token}}"}},
		Body: Body{Mode: BodyRaw, Raw: `{"token":"{{token}}"}`,
			Fields: []KV{{Key: "f", Value: "{{host}}"}}},
	}
	got, missing := Prepare(req, vars)
	if len(missing) != 0 {
		t.Fatalf("不该有缺失变量: %v", missing)
	}
	if got.Query[0].Value != "t-1" || got.Body.Fields[0].Value != "example.com" {
		t.Errorf("query/body 字段没替换: %+v %+v", got.Query, got.Body.Fields)
	}
	if !strings.Contains(got.Body.Raw, `"t-1"`) {
		t.Errorf("raw body 没替换: %q", got.Body.Raw)
	}
}

func TestMergePriority(t *testing.T) {
	layers := []Layer{
		{Source: VarGlobal, Vars: []KV{{ID: "g", Key: "a", Value: "global", Enabled: true}}},
		{Source: VarWorkspace, Vars: []KV{
			{ID: "w", Key: "a", Value: "workspace", Enabled: true},
			{ID: "w2", Key: "b", Value: "workspace", Enabled: true},
		}},
		{Source: VarEnvironment, Vars: []KV{
			{ID: "e", Key: "b", Value: "initial", Enabled: true},
			{ID: "e2", Key: "c", Value: "env", Enabled: true},
			{ID: "e3", Key: "off", Value: "env", Enabled: false},
		}, Current: map[ID]string{"e": "current"}},
		{Source: VarCollection, Vars: []KV{{Key: "c", Value: "collection", Enabled: true}}},
		{Source: VarFolder, Vars: []KV{{Key: "d", Value: "folder", Enabled: true}}},
		{Source: VarRequest, Vars: []KV{{Key: "d", Value: "request", Enabled: true}}},
	}
	got := Merge(layers)

	if got["a"].Value != "workspace" || got["a"].Source != VarWorkspace {
		t.Errorf("a = %+v，想要 workspace 层", got["a"])
	}
	// 环境变量本机覆盖值优先于初始值。
	if got["b"].Value != "current" || got["b"].Source != VarEnvironment {
		t.Errorf("b = %+v，想要 current", got["b"])
	}
	if got["c"].Value != "collection" {
		t.Errorf("c = %+v，想要 collection", got["c"])
	}
	if got["d"].Value != "request" {
		t.Errorf("d = %+v，想要 request", got["d"])
	}
	if _, ok := got["off"]; ok {
		t.Errorf("禁用变量不应进入合并结果")
	}
}

func TestMergeSecrets(t *testing.T) {
	got := Merge([]Layer{{
		Source:  VarEnvironment,
		Vars:    []KV{{ID: "s1", Key: "token", Value: "", Enabled: true, Secret: true}},
		Secrets: map[ID]string{"s1": "real-secret"},
	}})
	if got["token"].Value != "real-secret" || !got["token"].Secret {
		t.Errorf("secret 应从 secrets 取真实值，得到 %+v", got["token"])
	}
}

func TestDynamicVariables(t *testing.T) {
	got, missing := Prepare(Request{URL: "{{$timestamp}}/{{$guid}}/{{$randomUUID}}/{{$randomInt}}/{{$randomEmail}}/{{$randomFirstName}}/{{$randomPrice}}"}, nil)
	if len(missing) != 0 {
		t.Fatalf("动态变量不该算缺失: %v", missing)
	}
	if strings.Contains(got.URL, "{{") {
		t.Errorf("动态变量没求值: %q", got.URL)
	}
	if !strings.Contains(got.URL, "@example.com") {
		t.Errorf("expected $randomEmail in URL, got: %q", got.URL)
	}
}

func TestResolveAuth(t *testing.T) {
	chain := []Auth{
		{Type: AuthBearer, Params: []KV{{Key: "token", Value: "t"}}},
		{Type: AuthInherit},
	}
	if got := ResolveAuth(chain); got.Type != AuthBearer {
		t.Errorf("继承后应得到 bearer，得到 %+v", got)
	}
	chain = append(chain, Auth{Type: AuthNone})
	if got := ResolveAuth(chain); got.Type != AuthNone {
		t.Errorf("显式 none 应终止继承，得到 %+v", got)
	}
	if got := ResolveAuth(nil); got.Type != AuthNone {
		t.Errorf("空链应得到 none，得到 %+v", got)
	}
}

func TestTreeOps(t *testing.T) {
	tree := []*Node{
		{ID: "f1", Kind: KindFolder, Name: "Users", Children: []*Node{
			{ID: "f2", Kind: KindFolder, Name: "Admin", Children: []*Node{
				{ID: "r1", Kind: KindRequest, Name: "获取用户"},
			}},
			{ID: "r2", Kind: KindRequest, Name: "创建用户"},
		}},
		{ID: "r3", Kind: KindRequest, Name: "根请求"},
	}

	if n := FindIn(tree, "r2"); n == nil || n.Name != "创建用户" {
		t.Fatalf("FindIn 没找到 r2: %v", n)
	}
	if p, ok := ParentOf(tree, "r2"); !ok || p.ID != "f1" {
		t.Errorf("ParentOf(r2) = %v %v，想要 f1", p, ok)
	}
	if p, ok := ParentOf(tree, "r3"); !ok || p != nil {
		t.Errorf("ParentOf(r3) 应该是根层，得到 %v %v", p, ok)
	}
	if got := CountRequests(tree); got != 3 {
		t.Errorf("CountRequests = %d，想要 3", got)
	}
	path, ok := FindPath(tree, "r1")
	if !ok || len(path) != 3 || path[0].ID != "f1" || path[2].ID != "r1" {
		t.Errorf("FindPath(r1) = %v %v", path, ok)
	}
	folders := AncestorFolders(tree, "r1")
	if len(folders) != 2 || folders[0].ID != "f1" || folders[1].ID != "f2" {
		t.Errorf("AncestorFolders(r1) = %v", folders)
	}
	if got := AncestorFolders(tree, "f2"); len(got) != 1 || got[0].ID != "f1" {
		t.Errorf("AncestorFolders(f2) = %v", got)
	}
	rest, ok := RemoveChild(tree, "r1")
	if !ok || FindIn(rest, "r1") != nil {
		t.Errorf("RemoveChild 没删掉 r1")
	}
	if FindIn(tree, "r1") == nil {
		t.Errorf("RemoveChild 把原树的节点也删了")
	}
}

func TestCloneIsDeep(t *testing.T) {
	follow := true
	c := &Collection{ID: "c1", Name: "集合", Variables: []KV{{Key: "a", Value: "1"}},
		Auth: Auth{Type: AuthBearer, Params: []KV{{Key: "token", Value: "t"}}},
		Items: []*Node{
			{ID: "f1", Kind: KindFolder, Children: []*Node{
				{ID: "r1", Kind: KindRequest, Variables: []KV{{Key: "b", Value: "2"}},
					Request: &Request{Method: "GET", Query: []KV{{Key: "q", Value: "1"}},
						Options: Options{FollowRedirects: &follow}}},
			}},
		}}
	next := c.Clone()
	next.Items[0].Children[0].Request.Method = "POST"
	next.Items[0].Children[0].Request.Query[0].Value = "2"
	next.Items[0].Children[0].Variables[0].Value = "3"
	next.Auth.Params[0].Value = "x"
	next.Name = "新名字"

	if got := c.Items[0].Children[0].Request.Method; got != "GET" {
		t.Errorf("克隆后改动影响了原集合，Method = %q", got)
	}
	if c.Items[0].Children[0].Request.Query[0].Value != "1" ||
		c.Items[0].Children[0].Variables[0].Value != "2" ||
		c.Auth.Params[0].Value != "t" {
		t.Errorf("克隆不够深")
	}
	if c.Name != "集合" {
		t.Errorf("克隆后改动影响了原集合名: %q", c.Name)
	}
}

func TestNextOrder(t *testing.T) {
	items := []*Node{{Order: 1024}, {Order: 3072}}
	if got := NextOrder(items); got != 4096 {
		t.Errorf("NextOrder = %d，想要 4096", got)
	}
	if got := NextOrder(nil); got != 1024 {
		t.Errorf("空列表 NextOrder = %d，想要 1024", got)
	}
}

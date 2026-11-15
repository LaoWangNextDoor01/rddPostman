package domain

import (
	"reflect"
	"testing"
)

func TestPrepare(t *testing.T) {
	env := Environment{Vars: []KV{
		{Key: "baseUrl", Value: "https://api.example.com", Enabled: true},
		{Key: "tenant", Value: "acme", Enabled: false},
	}}
	req := Request{
		Method:  "GET",
		URL:     "{{baseUrl}}/users/{{userId}}",
		Headers: []KV{{Key: "X-Tenant", Value: "{{tenant}}", Enabled: true}},
	}

	got, missing := Prepare(req, env)
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
	got, missing := Prepare(req, Environment{})
	if got.URL != req.URL || len(missing) != 0 {
		t.Errorf("无变量时应原样返回，得到 %q %v", got.URL, missing)
	}
}

func TestTreeOps(t *testing.T) {
	tree := []*Node{
		{ID: "f1", Kind: KindFolder, Name: "Users", Children: []*Node{
			{ID: "r1", Kind: KindRequest, Name: "获取用户"},
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
	rest, ok := RemoveChild(tree, "r1")
	if !ok || FindIn(rest, "r1") != nil {
		t.Errorf("RemoveChild 没删掉 r1")
	}
	if FindIn(tree, "r1") == nil {
		t.Errorf("RemoveChild 把原树的节点也删了")
	}
}

func TestCloneIsDeep(t *testing.T) {
	c := &Collection{ID: "c1", Name: "集合", Items: []*Node{
		{ID: "f1", Kind: KindFolder, Children: []*Node{
			{ID: "r1", Kind: KindRequest, Request: &Request{Method: "GET"}},
		}},
	}}
	next := c.Clone()
	next.Items[0].Children[0].Request.Method = "POST"
	next.Name = "新名字"

	if got := c.Items[0].Children[0].Request.Method; got != "GET" {
		t.Errorf("克隆后改动影响了原集合，Method = %q", got)
	}
	if c.Name != "集合" {
		t.Errorf("克隆后改动影响了原集合名: %q", c.Name)
	}
}

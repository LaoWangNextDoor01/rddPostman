package collection

import (
	"testing"
	"time"

	"rddpostman/components/theme"
	"rddpostman/internal/domain"

	"github.com/egoist/mygo/ui"
)

func TestRequestTabRender(t *testing.T) {
	ws := newWorkspace()
	verifyTLS := true
	followRedirects := true

	reqNode := &domain.Node{
		ID:        "req-1",
		Kind:      domain.KindRequest,
		Name:      "获取用户信息",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Auth: domain.Auth{
			Type: domain.AuthBearer,
			Params: []domain.KV{
				{Key: "token", Value: "test-token-123", Enabled: true},
			},
		},
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://api.example.com/v1/users/me?include=profile,settings",
			Headers: []domain.KV{
				{Key: "Accept", Value: "application/json", Enabled: true, Desc: "期望返回格式"},
			},
			Query: []domain.KV{
				{Key: "include", Value: "profile,settings", Enabled: true, Desc: "附加字段"},
			},
			Body: domain.Body{
				Mode: domain.BodyRaw,
				Raw:  `{"client": "test"}`,
			},
			Options: domain.Options{
				TimeoutMS:       10000,
				FollowRedirects: &followRedirects,
				VerifyTLS:       &verifyTLS,
			},
		},
	}

	coll := &domain.Collection{
		ID:        "coll-1",
		Name:      "用户中心",
		UpdatedAt: time.Now(),
		Items:     []*domain.Node{reqNode},
	}

	renamed := ""
	nodeUpdated := false

	props := pageProps{
		workspaceID:   "ws-1",
		workspaceName: "默认空间",
		collections:   []*domain.Collection{coll},
		state:         ws,
		actions: pageActions{
			rename: func(id, name string) {
				renamed = name
				reqNode.Name = name
			},
			updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
				nodeUpdated = true
				fn(reqNode)
			},
			navigate: func(path string) {},
		},
	}

	// 1. 测试常规渲染
	view := func(c *ui.Context) {
		requestTab(c, props, coll, reqNode, string(reqNode.ID))
	}

	tt := ui.NewTester(view, 960, 700)
	tt.Frame()
	img := tt.Image()
	if img == nil {
		t.Fatal("RequestTab 常规模式渲染失败：图像为空")
	}

	// 2. 测试所有请求配置 Tab 切换
	tabs := []int{0, 1, 2, 3, 4}
	for _, tabIndex := range tabs {
		tabView := func(c *ui.Context) {
			root := ui.Column(c).Grow(1)
			state := ui.Local(root, "req-session-"+string(reqNode.ID), func() reqSessionState {
				s := defaultSessionState(string(reqNode.ID), reqNode.Request, &reqNode.Auth)
				s.reqTab = tabIndex
				return s
			})
			state.reqTab = tabIndex
			reqConfigTabs(c, props, coll, reqNode, string(reqNode.ID), state, theme.Of(c), theme.Metrics)
		}
		ttTab := ui.NewTester(tabView, 960, 400)
		ttTab.Frame()
		if ttTab.Image() == nil {
			t.Fatalf("请求配置 Tab %d 渲染失败：图像为空", tabIndex)
		}
	}

	// 3. 测试响应区各个 Tab 切换
	respTabs := []int{0, 1, 2}
	for _, rTab := range respTabs {
		rView := func(c *ui.Context) {
			root := ui.Column(c).Grow(1)
			state := ui.Local(root, "req-session-"+string(reqNode.ID), func() reqSessionState {
				s := defaultSessionState(string(reqNode.ID), reqNode.Request, &reqNode.Auth)
				s.respTab = rTab
				return s
			})
			state.respTab = rTab
			responsePane(c, props, reqNode, state, theme.Of(c), theme.Metrics)
		}
		ttResp := ui.NewTester(rView, 960, 400)
		ttResp.Frame()
		if ttResp.Image() == nil {
			t.Fatalf("响应区 Tab %d 渲染失败：图像为空", rTab)
		}
	}

	// 4. 测试面包屑在编辑状态下的渲染
	bView := func(c *ui.Context) {
		root := ui.Column(c).Grow(1)
		title := ui.Local(root, "req-title-"+string(reqNode.ID), func() reqTitleState {
			return reqTitleState{
				id:      string(reqNode.ID),
				editing: true,
				draft:   "修改后的请求名",
			}
		})
		reqBreadcrumb(c, props, reqNode, string(reqNode.ID), title, theme.Of(c), theme.Metrics)
	}
	ttBreadcrumb := ui.NewTester(bView, 960, 100)
	ttBreadcrumb.Frame()
	if ttBreadcrumb.Image() == nil {
		t.Fatal("面包屑编辑态渲染失败：图像为空")
	}

	// 5. 测试请求行（包含下拉打开态）
	rLineView := func(c *ui.Context) {
		root := ui.Column(c).Grow(1)
		state := ui.Local(root, "req-session-"+string(reqNode.ID), func() reqSessionState {
			s := defaultSessionState(string(reqNode.ID), reqNode.Request, &reqNode.Auth)
			s.methodOpen = true
			return s
		})
		reqLine(c, props, coll, reqNode, string(reqNode.ID), state, theme.Of(c), theme.Metrics)
	}
	ttRLine := ui.NewTester(rLineView, 960, 200)
	ttRLine.Frame()
	if ttRLine.Image() == nil {
		t.Fatal("请求行下拉打开态渲染失败：图像为空")
	}

	// 6. 测试 URL 与 Query 参数的互相同步逻辑
	parsed := parseQueryFromURL("https://api.example.com/items?page=1&limit=20")
	if len(parsed) != 2 || parsed[0].Key != "page" || parsed[0].Value != "1" {
		t.Fatalf("parseQueryFromURL 失败，得到: %+v", parsed)
	}

	syncedURL := syncQueryToURL("https://api.example.com/items", []domain.KV{
		{Key: "sort", Value: "desc", Enabled: true},
		{Key: "archived", Value: "false", Enabled: false}, // 禁用不拼接到 URL
		{Key: "", Value: "ignore", Enabled: true},         // 空 Key 忽略
	})
	if syncedURL != "https://api.example.com/items?sort=desc" {
		t.Fatalf("syncQueryToURL 失败，得到: %s", syncedURL)
	}

	_ = renamed
	_ = nodeUpdated
}

func TestParamsEditingAndPersistence(t *testing.T) {
	node := &domain.Node{
		ID:   "req-test",
		Name: "测试请求",
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://api.example.com/test",
		},
	}
	coll := &domain.Collection{
		ID:    "coll-test",
		Items: []*domain.Node{node},
	}

	persisted := false
	props := pageProps{
		collections: []*domain.Collection{coll},
		actions: pageActions{
			updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
				persisted = true
				fn(node)
			},
		},
	}

	state := defaultSessionState("req-test", node.Request, nil)
	if len(state.queryDraft) == 0 {
		t.Fatal("queryDraft 应当至少有一行可编辑项")
	}

	// 模拟在 queryDraft 中输入参数
	state.queryDraft[0].Key = "filter"
	state.queryDraft[0].Value = "active"

	// 触发更新持久化
	state.urlDraft = syncQueryToURL(state.urlDraft, state.queryDraft)
	props.actions.updateNode(coll.ID, node.ID, func(n *domain.Node) {
		if n.Request != nil {
			n.Request.Query = cleanKVList(state.queryDraft)
			n.Request.URL = state.urlDraft
		}
	})

	if !persisted {
		t.Fatal("updateNode 未被调用")
	}
	if node.Request.URL != "https://api.example.com/test?filter=active" {
		t.Fatalf("URL 未被同步更新，当前: %s", node.Request.URL)
	}
	if len(node.Request.Query) != 1 || node.Request.Query[0].Key != "filter" {
		t.Fatalf("Query 未正确持久化，当前: %+v", node.Request.Query)
	}
}

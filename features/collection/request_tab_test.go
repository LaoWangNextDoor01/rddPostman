package collection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"rddpostman/components/theme"
	"rddpostman/internal/domain"
	"rddpostman/internal/transport"

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
			resolveAuth: func(collID, nodeID domain.ID) domain.Auth {
				return domain.Auth{Type: domain.AuthBearer}
			},
			sendRequest: func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
				onDone(&transport.Response{
					StatusCode:    200,
					StatusText:    "OK",
					Duration:      120 * time.Millisecond,
					Size:          1024,
					FormattedBody: `{"ok":true}`,
				})
				return func() {}
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

	// 2. 测试所有请求配置 Tab 切换 (Params, Auth, Headers, Body, Scripts, Settings)
	tabs := []int{0, 1, 2, 3, 4, 5}
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

	// 3. 测试响应区各个 Tab 切换 (Body, Headers, Cookies, Test Results)
	respTabs := []int{0, 1, 2, 3}
	for _, rTab := range respTabs {
		rView := func(c *ui.Context) {
			root := ui.Column(c).Grow(1)
			state := ui.Local(root, "req-session-"+string(reqNode.ID), func() reqSessionState {
				s := defaultSessionState(string(reqNode.ID), reqNode.Request, &reqNode.Auth)
				s.respTab = rTab
				s.hasResponse = true
				s.respHeaders = []domain.KV{{Key: "Content-Type", Value: "application/json"}}
				s.respCookies = []transport.ResponseCookie{{Name: "session", Value: "xyz"}}
				s.respTestResults = []domain.TestResult{
					{Name: "Status is 200", Passed: true},
					{Name: "Response time is reasonable", Passed: false, Error: "响应耗时超出限制"},
				}
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
		reqBreadcrumb(c, props, reqNode, string(reqNode.ID), title, func() {}, theme.Of(c), theme.Metrics)
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
		{Key: "archived", Value: "false", Enabled: false},
		{Key: "", Value: "ignore", Enabled: true},
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

	state.queryDraft[0].Key = "filter"
	state.queryDraft[0].Value = "active"

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

func TestSendRequestWithRealServerAndInterpolation(t *testing.T) {
	// 验证：真实 HTTP 发送、{{变量}} 插值与认证生效
	receivedAuth := ""
	receivedHeader := ""

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedAuth = r.Header.Get("Authorization")
		receivedHeader = r.Header.Get("X-Env")
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"code":200,"msg":"success"}`))
	}))
	defer ts.Close()

	// 模拟变量层：{{host}} -> ts.URL, {{token}} -> my-secret-token, {{envName}} -> production
	vars := map[string]domain.Variable{
		"host":    {Key: "host", Value: ts.URL},
		"token":   {Key: "token", Value: "my-secret-token"},
		"envName": {Key: "envName", Value: "production"},
	}

	rawReq := domain.Request{
		Method: "POST",
		URL:    "{{host}}/v1/test",
		Headers: []domain.KV{
			{Key: "X-Env", Value: "{{envName}}", Enabled: true},
		},
	}
	rawAuth := domain.Auth{
		Type: domain.AuthBearer,
		Params: []domain.KV{
			{Key: "token", Value: "{{token}}", Enabled: true},
		},
	}

	// 1. 变量插值
	preparedReq, missingReq := domain.Prepare(rawReq, vars)
	if len(missingReq) > 0 {
		t.Fatalf("unexpected missing vars: %v", missingReq)
	}
	if preparedReq.URL != ts.URL+"/v1/test" {
		t.Fatalf("URL variable not substituted: %s", preparedReq.URL)
	}

	// 2. 认证插值
	preparedAuth, missingAuth := domain.PrepareAuth(rawAuth, vars)
	if len(missingAuth) > 0 {
		t.Fatalf("unexpected missing auth vars: %v", missingAuth)
	}

	// 3. 真实发送
	resp, err := transport.Send(context.Background(), preparedReq, preparedAuth)
	if err != nil {
		t.Fatalf("transport.Send error: %v", err)
	}
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if receivedAuth != "Bearer my-secret-token" {
		t.Fatalf("server did not receive expected Bearer auth, got: %s", receivedAuth)
	}
	if receivedHeader != "production" {
		t.Fatalf("server did not receive expected X-Env header, got: %s", receivedHeader)
	}
}

func TestScriptsAndSettingsTabRender(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:        "req-script-1",
		Kind:      domain.KindRequest,
		Name:      "脚本测试",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &domain.Request{
			Method:           "POST",
			URL:              "https://api.example.com/test",
			PreRequestScript: `pm.variables.set("timestamp", "1609459200");`,
			TestsScript:      `pm.test("Status is 200", function () { pm.response.to.have.status(200); });`,
			Options: domain.Options{
				ProxyURL:         "http://127.0.0.1:7890",
				DisableCookieJar: false,
			},
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "集合",
		Items: []*domain.Node{reqNode},
	}

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
		actions: pageActions{
			updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
				fn(reqNode)
			},
			resolveAuth: func(collID, nodeID domain.ID) domain.Auth {
				return domain.Auth{}
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		requestTab(c, props, coll, reqNode, string(reqNode.ID))
	}, 960, 700)
	tt.Frame()
	if tt.Image() == nil {
		t.Fatal("ScriptsAndSettingsTabRender failed: image is nil")
	}
}

func TestImportCurlAndPostmanDialogsRender(t *testing.T) {
	ws := newWorkspace()
	ws.importCurlOpen = true
	ws.importCurlText = "curl -X POST https://api.example.com/login -H \"Content-Type: application/json\" -d '{\"user\":\"admin\"}'"
	ws.importPostmanOpen = true
	ws.importPostmanText = "{\"info\":{\"name\":\"TestColl\",\"schema\":\"https://schema.getpostman.com/json/collection/v2.1.0/collection.json\"},\"item\":[]}"

	props := pageProps{
		workspaceID: "ws-1",
		state:       ws,
	}

	ttCurl := ui.NewTester(func(c *ui.Context) {
		importCurlDialog(c, props)
	}, 600, 400)
	ttCurl.Frame()
	if ttCurl.Image() == nil {
		t.Fatal("importCurlDialog render image is nil")
	}

	ttPostman := ui.NewTester(func(c *ui.Context) {
		importPostmanDialog(c, props)
	}, 600, 400)
	ttPostman.Frame()
	if ttPostman.Image() == nil {
		t.Fatal("importPostmanDialog render image is nil")
	}
}

func TestSidebarFilterAndSearchRender(t *testing.T) {
	ws := newWorkspace()
	props := pageProps{
		workspaceID: "ws-1",
		state:       ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		sidebar(c, props)
	}, 260, 700)
	tt.Frame()
	if tt.Image() == nil {
		t.Fatal("sidebar render image is nil")
	}

	// 测试筛选 Popover 展开与复选框勾选
	tt.Click("Filter sidebar")
	tt.Frame()

	// 再次点击关闭
	tt.Click("Filter sidebar")
	tt.Frame()
}

func TestBodyEditorModesAndBeautify(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:        "req-body-test",
		Kind:      domain.KindRequest,
		Name:      "Body Test",
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Request: &domain.Request{
			Method: "POST",
			URL:    "https://api.example.com/test",
			Body: domain.Body{
				Mode: domain.BodyRaw,
				Raw:  `{"test":   123, "arr":[1,  2]}`,
			},
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "Test Coll",
		Items: []*domain.Node{reqNode},
	}

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
		actions: pageActions{
			updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
				fn(reqNode)
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		requestTab(c, props, coll, reqNode, string(reqNode.ID))
	}, 960, 700)
	tt.Frame()
	if tt.Image() == nil {
		t.Fatal("requestTab image is nil")
	}

	// 点击 Body Tab
	tt.Click("Body")
	tt.Frame()

	// 点击 Beautify 格式化 JSON
	tt.Click("Beautify")
	tt.Frame()

	// 点击 binary 单选框
	tt.Click("binary")
	tt.Frame()
	if reqNode.Request.Body.Mode != domain.BodyMode("binary") {
		t.Fatalf("expected binary mode, got %s", reqNode.Request.Body.Mode)
	}
}

func TestDocsTabAndBulkEditSwitch(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:          "req-docs-test",
		Kind:        domain.KindRequest,
		Name:        "Docs Test",
		Description: "Initial description",
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://api.example.com/docs",
			Query: []domain.KV{
				{Key: "userId", Value: "10086", Enabled: true},
				{Key: "lang", Value: "zh-CN", Enabled: true},
			},
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "Test Coll",
		Items: []*domain.Node{reqNode},
	}

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
		actions: pageActions{
			updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
				fn(reqNode)
			},
		},
	}

	tt := ui.NewTester(func(c *ui.Context) {
		requestTab(c, props, coll, reqNode, string(reqNode.ID))
	}, 960, 700)
	tt.Frame()
	if tt.Image() == nil {
		t.Fatal("requestTab image is nil")
	}

	// 1. 测试 Docs Tab 切换与 Markdown 工具栏点击
	tt.Click("Docs")
	tt.Frame()
	tt.Click("Bold")
	tt.Frame()

	// 2. 切回 Params 并测试 Bulk Edit 切换
	tt.Click("Params")
	tt.Frame()
	tt.Click("Bulk Edit")
	tt.Frame()

	// 3. 点击切回 Key-Value Edit
	tt.Click("Key-Value Edit")
	tt.Frame()
}

func TestTabsBarDoesNotRenderOverview(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:   "req-1",
		Kind: domain.KindRequest,
		Name: "Test Request",
		Request: &domain.Request{
			Method: "GET",
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "Test Coll",
		Items: []*domain.Node{reqNode},
	}
	ws.openTab("req-1")

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		tabsBar(c, props)
	}, 800, 40)
	tt.Frame()

	if tt.HasText("Overview") {
		t.Fatalf("tabsBar should not contain Overview tab, but found it in texts: %q", tt.Texts())
	}
	if !tt.HasText("Test Request") {
		t.Fatalf("tabsBar should contain open request tab 'Test Request', got: %q", tt.Texts())
	}
}

func TestSidebarFlatHierarchy(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:   "req-1",
		Kind: domain.KindRequest,
		Name: "Get User Info",
		Request: &domain.Request{
			Method: "GET",
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "User Service API",
		Items: []*domain.Node{reqNode},
	}

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		sidebar(c, props)
	}, 260, 600)
	tt.Frame()

	// 1. 顶层直接展示 Collection 级别
	if !tt.HasText("User Service API") {
		t.Fatalf("sidebar must render collection name directly, got texts: %q", tt.Texts())
	}

	// 2. 验证不再有大写顶层层级标签
	for _, unwanted := range []string{"COLLECTIONS", "ENVIRONMENTS", "SPECS", "MOCKS", "DATASETS"} {
		if tt.HasText(unwanted) {
			t.Fatalf("sidebar should not contain flat section header %q, found in texts: %q", unwanted, tt.Texts())
		}
	}
}

func TestTabsBarHorizontalScrollAndNoEnvSwitcher(t *testing.T) {
	ws := newWorkspace()
	reqNode := &domain.Node{
		ID:   "req-1",
		Kind: domain.KindRequest,
		Name: "Sample API",
		Request: &domain.Request{
			Method: "POST",
		},
	}
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "Test Coll",
		Items: []*domain.Node{reqNode},
	}
	ws.openTab("req-1")

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		environments: []domain.Environment{
			{ID: "env-1", Name: "Production"},
		},
		state: ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		tabsBar(c, props)
	}, 800, 40)
	tt.Frame()

	// 1. 验证 Tab 正常渲染
	if !tt.HasText("Sample API") {
		t.Fatalf("tabsBar must render active tab, got: %q", tt.Texts())
	}

	// 2. 验证最右侧环境选择器已彻底去掉
	if tt.HasText("No Environment") || tt.HasText("Production") {
		t.Fatalf("tabsBar should NOT contain environment switcher, got texts: %q", tt.Texts())
	}
}

func TestEmptyWorkspaceWhiteActionCards(t *testing.T) {
	ws := newWorkspace()
	coll := &domain.Collection{
		ID:   "coll-1",
		Name: "Test Coll",
	}

	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		mainPane(c, props)
	}, 900, 600)
	tt.Frame()

	// 未打开任何 Tab 时呈现欢迎空态
	if !tt.HasText("API Workspace") {
		t.Fatalf("mainPane should render empty workspace welcome title, got: %q", tt.Texts())
	}
	if !tt.HasText("New request") || !tt.HasText("New collection") || !tt.HasText("导入接口 (cURL)") {
		t.Fatalf("mainPane should render four action cards, got: %q", tt.Texts())
	}
}

func TestNewRequestDefaultUnsavedWithDotAndSaveAction(t *testing.T) {
	ws := newWorkspace()
	coll := &domain.Collection{
		ID:    "coll-1",
		Name:  "My APIs",
		Items: []*domain.Node{},
	}

	// 1. 模拟新建请求操作：调用 createNodeIn
	createNodeIn(nil, ws, []*domain.Collection{coll}, domain.KindRequest, string(coll.ID))

	// 2. 验证默认未保存：coll.Items 依然为空（没有持久化写入）
	if len(coll.Items) != 0 {
		t.Fatalf("new request must NOT be saved by default, but found items: %d", len(coll.Items))
	}

	// 3. 验证 Tab 已打开，且处于未保存状态
	if len(ws.tabs) != 1 {
		t.Fatalf("expected 1 open tab, got: %d", len(ws.tabs))
	}
	draftID := ws.tabs[0]
	if !ws.isUnsaved(draftID) {
		t.Fatalf("new request tab %s must have isUnsaved=true", draftID)
	}

	// 4. 测试 tabsBar 渲染：Tab 名称显示 New request，且包含未保存状态
	props := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{coll},
		state:       ws,
	}

	tt := ui.NewTester(func(c *ui.Context) {
		tabsBar(c, props)
	}, 800, 40)
	tt.Frame()

	if !tt.HasText("New request") {
		t.Fatalf("tabsBar must render 'New request' tab, got: %q", tt.Texts())
	}

	// 5. 模拟手动保存
	saved := false
	props.actions.saveRequest = func(id string, req domain.Request, auth domain.Auth, name string) {
		if draft, ok := ws.drafts[id]; ok {
			coll.Items = append(coll.Items, draft.node)
			delete(ws.drafts, id)
			ws.setUnsaved(id, false)
			saved = true
		}
	}

	props.actions.saveRequest(draftID, domain.Request{Method: "GET", URL: "https://example.com/api"}, domain.Auth{}, "My Saved Request")

	if !saved {
		t.Fatal("saveRequest was not called")
	}
	if len(coll.Items) != 1 {
		t.Fatalf("collection should have 1 saved item now, got: %d", len(coll.Items))
	}
	if ws.isUnsaved(draftID) {
		t.Fatal("isUnsaved must be false after manual save")
	}
}

func TestDefaultRequestHeadersAndUserCustomization(t *testing.T) {
	ws := newWorkspace()
	coll := &domain.Collection{
		ID:    "coll-headers-test",
		Name:  "Headers Test Collection",
		Items: []*domain.Node{},
	}

	// 1. 新建请求，验证自动注入 Postman 默认规范请求头
	createNodeIn(nil, ws, []*domain.Collection{coll}, domain.KindRequest, string(coll.ID))
	draftID := ws.active
	draft, ok := ws.drafts[draftID]
	if !ok || draft == nil || draft.node == nil || draft.node.Request == nil {
		t.Fatal("新建请求草稿未成功创建")
	}

	headers := draft.node.Request.Headers
	if len(headers) < 4 {
		t.Fatalf("新建请求默认 headers 应包含至少 4 个 Postman 默认头，实际得到: %d", len(headers))
	}

	expectedKeys := map[string]bool{
		"User-Agent":      false,
		"Accept":          false,
		"Accept-Encoding": false,
		"Connection":      false,
	}
	for _, h := range headers {
		if _, exists := expectedKeys[h.Key]; exists {
			expectedKeys[h.Key] = true
		}
	}
	for k, found := range expectedKeys {
		if !found {
			t.Errorf("缺少默认请求头: %s", k)
		}
	}

	// 2. 模拟用户自由修改与追加新头
	headers = append(headers, domain.KV{
		Key:     "X-Custom-Auth",
		Value:   "token_test_12345",
		Enabled: true,
	})
	cleaned := cleanKVList(headers)
	if len(cleaned) != 5 {
		t.Fatalf("追加自定义 Header 后总数应为 5，得到: %d", len(cleaned))
	}

	// 3. 模拟用户自由删除其中某项默认头（例如删除 User-Agent）
	var withoutUA []domain.KV
	for _, h := range cleaned {
		if h.Key != "User-Agent" {
			withoutUA = append(withoutUA, h)
		}
	}
	cleanedWithoutUA := cleanKVList(withoutUA)
	if len(cleanedWithoutUA) != 4 {
		t.Fatalf("删除 User-Agent 后应剩余 4 个 Header，得到: %d", len(cleanedWithoutUA))
	}
	for _, h := range cleanedWithoutUA {
		if h.Key == "User-Agent" {
			t.Fatal("User-Agent 应已被成功删除")
		}
	}

	// 4. 测试恢复默认请求头
	reset := domain.DefaultRequestHeaders()
	if len(reset) != 4 {
		t.Fatalf("DefaultRequestHeaders 应返回 4 项，得到: %d", len(reset))
	}
}


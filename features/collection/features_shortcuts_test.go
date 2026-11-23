package collection

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/store"
	"rddpostman/internal/transport"

	"github.com/egoist/mygo/ui"
)

func TestRequestAndWorkbenchShortcuts(t *testing.T) {
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ws := db.ActiveWorkspace()
	coll, err := db.CreateCollection(ws.ID, "Core API", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	req1 := &domain.Node{
		ID:   domain.NewID(),
		Kind: domain.KindRequest,
		Name: "First Request",
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://example.com/first",
		},
	}
	req2 := &domain.Node{
		ID:   domain.NewID(),
		Kind: domain.KindRequest,
		Name: "Second Request",
		Request: &domain.Request{
			Method: "POST",
			URL:    "https://example.com/second",
		},
	}
	if err := db.AddNode(coll.ID, "", req1); err != nil {
		t.Fatalf("AddNode req1: %v", err)
	}
	if err := db.AddNode(coll.ID, "", req2); err != nil {
		t.Fatalf("AddNode req2: %v", err)
	}

	// 1. 验证请求级快捷键 Cmd+Enter 发送 & Cmd+S 保存 & Cmd+L 聚焦 URL
	var sent atomic.Int32
	var saved atomic.Int32
	state := newWorkspace()
	state.openTab(string(req1.ID))

	reqView := func(c *ui.Context) {
		props := pageProps{
			workspaceID: ws.ID,
			collections: db.ActiveWorkspace().Collections,
			state:       state,
			actions: pageActions{
				saveRequest: func(id string, req domain.Request, auth domain.Auth, name string) {
					saved.Add(1)
				},
				sendRequest: func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
					sent.Add(1)
					onDone(&transport.Response{
						StatusCode:    200,
						Duration:      12 * time.Millisecond,
						Size:          16,
						FormattedBody: `{"ok":true}`,
					})
					return func() {}
				},
				updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
					_ = db.UpdateNode(collID, nodeID, fn)
				},
			},
		}
		requestTab(c, props, db.ActiveWorkspace().Collections[0], req1, string(req1.ID))
	}

	ttReq := ui.NewTester(reqView, 1000, 700)
	ttReq.Key(ui.Cmd, ui.KeyS)
	ttReq.Frame()
	if saved.Load() != 1 {
		t.Fatalf("Cmd+S save count = %d, want 1", saved.Load())
	}

	ttReq.Key(ui.Cmd, ui.KeyEnter)
	ttReq.Frame()
	if sent.Load() != 1 {
		t.Fatalf("Cmd+Enter send count = %d, want 1", sent.Load())
	}
	if !ttReq.HasText("200 OK") {
		t.Fatalf("expected 200 OK after Cmd+Enter; got %q", ttReq.Texts())
	}

	ttReq.Key(ui.Cmd, ui.KeyL)
	ttReq.Frame()
	ttReq.Type("/extra")
	ttReq.Frame()

	// 2. 验证工作台级 Tab 切换 (Ctrl+Tab / Ctrl+Shift+Tab) 与关闭 (Cmd+W)
	pending := []WorkbenchCommand{
		{Kind: CmdOpenTab, TargetID: string(req1.ID)},
		{Kind: CmdOpenTab, TargetID: string(req2.ID)},
	}
	wbView := func(c *ui.Context) {
		WorkbenchView(c, db, nil, ws.ID, WorkbenchSlots{
			ConsumeCommand: func() *WorkbenchCommand {
				if len(pending) == 0 {
					return nil
				}
				cmd := pending[0]
				pending = pending[1:]
				return &cmd
			},
		})
	}
	ttWB := ui.NewTester(wbView, 1200, 760)
	ttWB.Frame() // consumes req2
	ttWB.Frame()
	if !ttWB.HasText("Second Request") || !ttWB.HasText("First Request") {
		t.Fatalf("expected both request tabs open; got %q", ttWB.Texts())
	}

	// Ctrl+Tab 切回 First Request（面包屑变为 First Request，Method 为 GET）
	ttWB.Key(ui.Ctrl, ui.KeyTab)
	ttWB.Frame()
	if !hasBreadcrumb(ttWB.Texts(), "First Request") {
		t.Fatalf("Ctrl+Tab should switch to First Request; got %q", ttWB.Texts())
	}

	// Cmd+W 关闭 First Request，剩余 Second Request（面包屑变为 Second Request）
	ttWB.Key(ui.Cmd, ui.KeyW)
	ttWB.Frame()
	if !hasBreadcrumb(ttWB.Texts(), "Second Request") {
		t.Fatalf("Cmd+W should close First Request and activate Second Request; got %q", ttWB.Texts())
	}
}

func hasBreadcrumb(texts []string, name string) bool {
	for i := 0; i+1 < len(texts); i++ {
		if texts[i] == "›" && texts[i+1] == name {
			return true
		}
	}
	return false
}

func TestBinaryBodyFileSelection(t *testing.T) {
	origPicker := openBinaryFilePicker
	defer func() { openBinaryFilePicker = origPicker }()

	wantPath := filepath.Join(t.TempDir(), "payload.bin")
	done := make(chan struct{}, 1)
	openBinaryFilePicker = func() (string, error) {
		done <- struct{}{}
		return wantPath, nil
	}

	coll := &domain.Collection{ID: "c1", Name: "Uploads"}
	node := &domain.Node{
		ID:   "r1",
		Kind: domain.KindRequest,
		Name: "Upload File",
		Request: &domain.Request{
			Method: "POST",
			URL:    "https://example.com/upload",
			Body:   domain.Body{Mode: domain.BodyBinary},
		},
	}
	coll.Items = []*domain.Node{node}

	state := newWorkspace()
	state.openTab("r1")

	view := func(c *ui.Context) {
		props := pageProps{
			workspaceID: "ws1",
			collections: []*domain.Collection{coll},
			state:       state,
			actions: pageActions{
				updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
					fn(node)
				},
			},
		}
		requestTab(c, props, coll, node, "r1")
	}

	tt := ui.NewTester(view, 1000, 700)
	if err := tt.Click("Body"); err != nil {
		t.Fatalf("Click Body tab: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Select File"); err != nil {
		t.Fatalf("Click Select File: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for binary file picker")
	}
	tt.Frame()

	if node.Request.Body.File != wantPath {
		t.Fatalf("node.Request.Body.File = %q, want %q", node.Request.Body.File, wantPath)
	}
	if !tt.HasText("payload.bin") {
		t.Fatalf("expected binary file badge 'payload.bin'; got %q", tt.Texts())
	}

	if err := tt.Click("Clear file"); err != nil {
		t.Fatalf("Click Clear file: %v", err)
	}
	tt.Frame()
	if node.Request.Body.File != "" {
		t.Fatalf("expected cleared binary file path, got %q", node.Request.Body.File)
	}
}

func TestCollectionRunnerExecutesAllRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()

	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ws := db.ActiveWorkspace()
	coll, err := db.CreateCollection(ws.ID, "Runner Suite", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	for _, name := range []string{"Health Check", "Status Check"} {
		n := &domain.Node{
			ID:   domain.NewID(),
			Kind: domain.KindRequest,
			Name: name,
			Request: &domain.Request{
				Method:      "GET",
				URL:         srv.URL,
				TestsScript: `pm.test("Status is 200", function() { pm.response.to.have.status(200); });`,
			},
		}
		if err := db.AddNode(coll.ID, "", n); err != nil {
			t.Fatalf("AddNode: %v", err)
		}
	}

	state := newWorkspace()
	state.openTab(string(coll.ID))
	state.setSegment(string(coll.ID), segRun)

	view := func(c *ui.Context) {
		snap := db.Snapshot().ActiveWorkspace()
		props := pageProps{
			workspaceID: snap.ID,
			collections: snap.Collections,
			state:       state,
			actions: pageActions{
				runRequestSync: func(ctx context.Context, collID, nodeID domain.ID, req domain.Request, auth domain.Auth) *transport.Response {
					return executeRequestSync(ctx, db, snap.ID, snap.SelectedEnvironmentID, collID, nodeID, req, auth)
				},
			},
		}
		overviewTab(c, props, snap.Collections[0], nil)
	}

	tt := ui.NewTester(view, 1000, 760)
	if !tt.HasText("Run Collection") {
		t.Fatalf("expected Run Collection button; got %q", tt.Texts())
	}
	if err := tt.Click("Run Collection"); err != nil {
		t.Fatalf("Click Run Collection: %v", err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("执行完成：共 2 个请求") && tt.HasText("Tests: 1/1 passed") {
			break
		}
		time.Sleep(25 * time.Millisecond)
	}
	if !tt.HasText("执行完成：共 2 个请求") || !tt.HasText("Health Check") || !tt.HasText("Status Check") {
		t.Fatalf("Collection Runner did not finish with expected results; got %q", tt.Texts())
	}

	hist, err := db.History(ws.ID, 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(hist) != 2 {
		t.Fatalf("history entries = %d, want 2", len(hist))
	}
}

func TestResponseHeadersAndCookiesCopy(t *testing.T) {
	headers := []domain.KV{
		{Key: "Content-Type", Value: "application/json", Enabled: true},
		{Key: "X-Trace-Id", Value: "trace-123", Enabled: true},
	}
	cookies := []transport.ResponseCookie{
		{Name: "sid", Value: "abc123", Domain: "example.com", Path: "/", HttpOnly: true},
		{Name: "theme", Value: "dark", Domain: "example.com", Path: "/"},
	}

	if got, want := formatResponseHeaders(headers), "Content-Type: application/json\nX-Trace-Id: trace-123"; got != want {
		t.Fatalf("formatResponseHeaders = %q, want %q", got, want)
	}
	if got, want := formatResponseCookies(cookies), "sid=abc123; theme=dark"; got != want {
		t.Fatalf("formatResponseCookies = %q, want %q", got, want)
	}

	coll := &domain.Collection{ID: "c1", Name: "Copy Suite"}
	node := &domain.Node{
		ID:   "r1",
		Kind: domain.KindRequest,
		Name: "Inspect Headers & Cookies",
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://example.com/test",
		},
	}
	coll.Items = []*domain.Node{node}
	state := newWorkspace()
	state.openTab("r1")

	view := func(c *ui.Context) {
		props := pageProps{
			workspaceID: "ws1",
			collections: []*domain.Collection{coll},
			state:       state,
			actions: pageActions{
				sendRequest: func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
					onDone(&transport.Response{
						StatusCode:    200,
						Duration:      10 * time.Millisecond,
						Size:          12,
						FormattedBody: `{"ok":true}`,
						Headers:       headers,
						Cookies:       cookies,
					})
					return func() {}
				},
			},
		}
		requestTab(c, props, coll, node, "r1")
	}

	tt := ui.NewTester(view, 1100, 760)
	if err := tt.Click("Send"); err != nil {
		t.Fatalf("Click Send: %v", err)
	}
	tt.Frame()

	// 1. 切到 Headers 响应页，验证单行复制与 Copy All 反馈
	if err := tt.Click("Headers 2"); err != nil {
		t.Fatalf("Click Headers 2: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Copy All") || !tt.HasText("X-Trace-Id") {
		t.Fatalf("expected response headers with Copy All; got %q", tt.Texts())
	}
	if err := tt.Click("Copy header X-Trace-Id"); err != nil {
		t.Fatalf("Click Copy header X-Trace-Id: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Copied") {
		t.Fatalf("expected Copied feedback after copying header; got %q", tt.Texts())
	}
	if err := tt.Click("Copy All Headers"); err != nil {
		t.Fatalf("Click Copy All Headers: %v", err)
	}
	tt.Frame()

	// 2. 切到 Cookies 响应页，验证单行 Name=Value / Value 复制与 Copy All Cookies
	if err := tt.Click("Cookies 2"); err != nil {
		t.Fatalf("Click Cookies 2: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Copy All") || !tt.HasText("sid") {
		t.Fatalf("expected response cookies with Copy All; got %q", tt.Texts())
	}
	if err := tt.Click("Cookie name sid"); err != nil {
		t.Fatalf("Click Cookie name sid: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Copied") {
		t.Fatalf("expected Copied feedback after copying cookie; got %q", tt.Texts())
	}
	if err := tt.Click("Copy All Cookies"); err != nil {
		t.Fatalf("Click Copy All Cookies: %v", err)
	}
	tt.Frame()
}

func TestDeleteTreeRowRequiresConfirmation(t *testing.T) {
	cases := []struct {
		name      string
		kind      treeKind
		wantTitle string
	}{
		{name: "Payments Collection", kind: treeCollection, wantTitle: "删除集合"},
		{name: "Auth Folder", kind: treeFolder, wantTitle: "删除目录"},
		{name: "Get Token", kind: treeRequest, wantTitle: "删除请求"},
	}

	for _, tc := range cases {
		t.Run(tc.wantTitle, func(t *testing.T) {
			state := newWorkspace()
			var deleted *treeNode
			props := pageProps{
				state: state,
				actions: pageActions{
					deleteRow: func(row *treeNode) {
						deleted = row
					},
				},
			}
			// 触发待删除二次确认态
			state.deletePending = &treeNode{id: "id-1", name: tc.name, kind: tc.kind}
			state.deleteConfirmOpen = true

			tt := ui.NewTester(func(c *ui.Context) {
				ui.Box(c).Fill()
				deleteTreeRowDialog(c, props)
			}, 800, 600)

			if !tt.HasText(tc.wantTitle) || !tt.HasText(tc.name) {
				t.Fatalf("expected confirm dialog %q mentioning %q; got %q", tc.wantTitle, tc.name, tt.Texts())
			}
			if deleted != nil {
				t.Fatalf("row should not be deleted before clicking confirm")
			}

			// 1. 点击取消：不删除且弹窗关闭
			if err := tt.Click("取消"); err != nil {
				t.Fatalf("Click 取消: %v", err)
			}
			tt.Frame()
			if deleted != nil || state.deleteConfirmOpen {
				t.Fatalf("cancel should abort deletion and close dialog")
			}

			// 2. 再次打开并点击确认删除：执行回调并关闭
			state.deletePending = &treeNode{id: "id-1", name: tc.name, kind: tc.kind}
			state.deleteConfirmOpen = true
			tt.Frame()
			if err := tt.Click("确认删除"); err != nil {
				t.Fatalf("Click 确认删除: %v", err)
			}
			tt.Frame()
			if deleted == nil || deleted.id != "id-1" {
				t.Fatalf("expected deleteRow called for id-1, got %+v", deleted)
			}
		})
	}
}

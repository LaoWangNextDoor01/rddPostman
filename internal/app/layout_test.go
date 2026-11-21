package app

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func saveLayoutPNG(t *testing.T, dir, name string, tt *ui.Tester) {
	t.Helper()
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	f, err := os.Create(filepath.Join(dir, name))
	if err != nil {
		t.Fatalf("Create %s: %v", name, err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
}

func newTestApp(t *testing.T) (*App, *store.Store) {
	t.Helper()
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return NewWithStore(db), db
}

func TestDualLayoutHubAndWorkbench(t *testing.T) {
	a, db := newTestApp(t)
	ws, err := db.CreateWorkspace("Payments API")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	coll, err := db.CreateCollection(ws.ID, "Demo Collection", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	reqNode := &domain.Node{
		ID:   domain.NewID(),
		Kind: domain.KindRequest,
		Name: "Get User Profile",
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://api.example.com/v1/profile",
		},
	}
	if err := db.AddNode(coll.ID, "", reqNode); err != nil {
		t.Fatalf("AddNode: %v", err)
	}
	if _, err := db.CreateEnvironment(ws.ID, "Staging"); err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
	_ = db.RecordHistory(ws.ID, domain.HistoryEntry{
		WorkspaceID:  ws.ID,
		CollectionID: coll.ID,
		RequestID:    reqNode.ID,
		Method:       "GET",
		URL:          "https://api.example.com/v1/profile",
		Status:       200,
		DurationMS:   42,
		At:           time.Now(),
	})

	tt := ui.NewTester(a.View, 1280, 800)
	snapDir := os.Getenv("LAYOUT_SNAPSHOT_DIR")

	// 1. 启动默认进入 (hub) Workspaces 列表页，带 Hub 侧栏 + 顶部 TitleBar + 底部 StatusBar
	if got := a.Router().Path(); got != routes.HubWorkspaces {
		t.Fatalf("initial path = %q, want %q", got, routes.HubWorkspaces)
	}
	if tt.HasText("Ctrl 20") {
		t.Fatalf("shortcutLabel regression: still showing Ctrl 20 in %q", tt.Texts())
	}
	if !tt.HasText("Ctrl K") && !tt.HasText("⌘K") {
		t.Fatalf("expected Ctrl K or ⌘K shortcut badge; got %q", tt.Texts())
	}
	saveLayoutPNG(t, snapDir, "hub-list.png", tt)
	for _, want := range []string{"wang's Team", "Workspaces", "API Catalog", "Payments API", "Create workspace"} {
		if !tt.HasText(want) {
			t.Fatalf("hub list missing %q; got %q", want, tt.Texts())
		}
	}

	// 2. 点击 Create workspace 跳转到独立子路由 /hub/workspaces/new，Hub 侧栏依然常驻
	if err := tt.Click("Create workspace"); err != nil {
		t.Fatalf("Click Create workspace: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubWorkspaceNew {
		t.Fatalf("after Create workspace path = %q, want %q", got, routes.HubWorkspaceNew)
	}
	if !tt.HasText("Create your workspace") || !tt.HasText("wang's Team") {
		t.Fatalf("expected create page inside HubLayout; got %q", tt.Texts())
	}

	// 点击 Cancel 返回 /hub/workspaces
	if err := tt.Click("Cancel"); err != nil {
		t.Fatalf("Click Cancel: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubWorkspaces {
		t.Fatalf("after Cancel path = %q, want %q", got, routes.HubWorkspaces)
	}

	// 3. 在 Hub 侧栏切换到 API Catalog，并用顶部 Back 按钮回退
	if err := tt.Click("API Catalog"); err != nil {
		t.Fatalf("Click API Catalog: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubAPICatalog {
		t.Fatalf("after API Catalog path = %q, want %q", got, routes.HubAPICatalog)
	}
	if !tt.HasText("Browse and manage your organization's API catalog.") {
		t.Fatalf("expected API Catalog placeholder; got %q", tt.Texts())
	}
	if err := tt.Click("Back"); err != nil {
		t.Fatalf("Click Back: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubWorkspaces {
		t.Fatalf("after Back path = %q, want %q", got, routes.HubWorkspaces)
	}

	// 4. 在 Hub 布局下点击顶部设置按钮：弹出全局设置 Dialog，不改变路由
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("Click Settings in HubLayout: %v", err)
	}
	if got := a.Router().Path(); got != routes.HubWorkspaces {
		t.Fatalf("settings dialog changed path to %q", got)
	}
	if !tt.HasText("General") || !tt.HasText("Themes") {
		t.Fatalf("expected global Settings dialog; got %q", tt.Texts())
	}
	saveLayoutPNG(t, snapDir, "hub-settings-dialog.png", tt)
	if err := tt.Click("Themes"); err != nil {
		t.Fatalf("Click Themes tab: %v", err)
	}
	if !tt.HasText("#336FFF") {
		t.Fatalf("expected Themes tab content; got %q", tt.Texts())
	}
	if err := tt.Click("Close settings"); err != nil {
		t.Fatalf("Click Close settings: %v", err)
	}
	if tt.HasText("#336FFF") {
		t.Fatalf("settings dialog did not close")
	}

	// 5. 点击工作区列表行进入 (workbench) 布局：Hub 侧栏消失，切换为工作台侧栏
	if err := tt.Click("Payments API"); err != nil {
		t.Fatalf("Click workspace row Payments API: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	wantWB := routes.WorkbenchCollectionsOf(string(ws.ID))
	if got := a.Router().Path(); got != wantWB {
		t.Fatalf("workbench path = %q, want %q", got, wantWB)
	}
	if tt.HasText("wang's Team") {
		t.Fatalf("Hub sidebar should not be shown inside WorkbenchLayout")
	}
	if !tt.HasText("Demo Collection") {
		t.Fatalf("expected Collections sidebar in WorkbenchLayout; got %q", tt.Texts())
	}

	// 展开集合并打开一个请求 Tab
	if err := tt.Click("Demo Collection"); err != nil {
		t.Fatalf("Click Demo Collection: %v", err)
	}
	if !tt.HasText("Get User Profile") {
		t.Fatalf("expected expanded request in tree; got %q", tt.Texts())
	}
	if err := tt.Click("Get User Profile"); err != nil {
		t.Fatalf("Click Get User Profile: %v", err)
	}
	if !tt.HasText("Query Params") || !tt.HasText("Send") {
		t.Fatalf("expected opened request tab in right mainPane; got %q", tt.Texts())
	}
	saveLayoutPNG(t, snapDir, "workbench-collections.png", tt)
	if snapDir != "" {
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		saveLayoutPNG(t, snapDir, "workbench-collections-dark.png", tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}

	// 6. 切换工作台左侧图标到 History：左侧变为 REQUEST HISTORY，右侧已打开的请求 Tab 保持不动！
	if err := tt.Click("History"); err != nil {
		t.Fatalf("Click History tab: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchHistoryOf(string(ws.ID)) {
		t.Fatalf("history path = %q, want %q", got, routes.WorkbenchHistoryOf(string(ws.ID)))
	}
	if !tt.HasText("REQUEST HISTORY") || !tt.HasText("Today") {
		t.Fatalf("expected History sidebar panel; got %q", tt.Texts())
	}
	if !tt.HasText("Query Params") || !tt.HasText("Get User Profile") {
		t.Fatalf("right mainPane lost open request tab when switching to History; got %q", tt.Texts())
	}
	saveLayoutPNG(t, snapDir, "workbench-history.png", tt)

	// 7. 再切到 Environments：左侧变为 ENVIRONMENTS，右侧已打开的请求 Tab 依然保持不动！
	if err := tt.Click("Environments"); err != nil {
		t.Fatalf("Click Environments tab: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchEnvironmentsOf(string(ws.ID)) {
		t.Fatalf("environments path = %q, want %q", got, routes.WorkbenchEnvironmentsOf(string(ws.ID)))
	}
	if !tt.HasText("ENVIRONMENTS") || !tt.HasText("Staging") {
		t.Fatalf("expected Environments sidebar panel with Staging; got %q", tt.Texts())
	}
	if !tt.HasText("Query Params") || !tt.HasText("Get User Profile") {
		t.Fatalf("right mainPane lost open request tab when switching to Environments; got %q", tt.Texts())
	}

	// 7b. 点击侧栏环境项「Staging」：在右侧多 Tab 区打开环境变量编辑 Tab，编辑并保存变量
	if err := tt.Click("Staging"); err != nil {
		t.Fatalf("Click Staging environment: %v", err)
	}
	if !tt.HasText("VARIABLE") || !tt.HasText("VALUE") || !tt.HasText("Save") {
		t.Fatalf("expected Environment Editor tab in right mainPane; got %q", tt.Texts())
	}
	if err := tt.Click("Variable key"); err != nil {
		t.Fatalf("Click Variable key: %v", err)
	}
	tt.Type("baseUrl")
	if err := tt.Click("Variable value"); err != nil {
		t.Fatalf("Click Variable value: %v", err)
	}
	tt.Type("https://staging.example.com")
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("Click Save env: %v", err)
	}
	if !tt.HasText("Saved") {
		t.Fatalf("expected Saved feedback after saving environment; got %q", tt.Texts())
	}
	saveLayoutPNG(t, snapDir, "workbench-env-tab.png", tt)

	// 校验变量已持久化到 store
	gotWS := db.Snapshot().Workspace(ws.ID)
	foundVar := false
	for _, env := range gotWS.Environments {
		if env.Name == "Staging" {
			for _, kv := range env.Vars {
				if kv.Key == "baseUrl" && kv.Value == "https://staging.example.com" {
					foundVar = true
				}
			}
		}
	}
	if !foundVar {
		t.Fatalf("saved environment variable baseUrl not found in store: %+v", gotWS.Environments)
	}

	// 从顶部多 Tab 栏点回原请求 Tab，确认两个 Tab 并存且可自由切换
	if err := tt.Click("Get User Profile"); err != nil {
		t.Fatalf("Switch back to Get User Profile tab: %v", err)
	}
	if !tt.HasText("Query Params") {
		t.Fatalf("expected request tab restored; got %q", tt.Texts())
	}

	// 8. 在 WorkbenchLayout 下打开全局设置弹窗，路由不变且关闭后请求 Tab 仍在
	if err := tt.Click("设置"); err != nil {
		t.Fatalf("Click Settings in WorkbenchLayout: %v", err)
	}
	if !tt.HasText("General") {
		t.Fatalf("expected Settings dialog in WorkbenchLayout; got %q", tt.Texts())
	}
	if err := tt.Click("Close settings"); err != nil {
		t.Fatalf("Click Close settings in WorkbenchLayout: %v", err)
	}
	if got := a.Router().Path(); got != routes.WorkbenchEnvironmentsOf(string(ws.ID)) {
		t.Fatalf("path changed after closing settings dialog: %q", got)
	}
	if !tt.HasText("Query Params") || !tt.HasText("Get User Profile") {
		t.Fatalf("open request tab lost after closing settings dialog")
	}

	// 9. 通过顶部 Workspace 下拉搜索过滤，并点击「创建」一键直达 /hub/workspaces/new
	if err := tt.Click(locales.SwitchWorkspace()); err != nil {
		t.Fatalf("Click workspace selector: %v", err)
	}
	if err := tt.Click("Search dropdown workspaces"); err != nil {
		t.Fatalf("Click dropdown search: %v", err)
	}
	tt.Type("XYZNoMatch")
	if !tt.HasText("No matching workspaces") {
		t.Fatalf("expected filtered empty state in workspace dropdown; got %q", tt.Texts())
	}
	if err := tt.Click("创建"); err != nil {
		t.Fatalf("Click 创建 in workspace selector: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubWorkspaceNew {
		t.Fatalf("after dropdown 创建 path = %q, want %q", got, routes.HubWorkspaceNew)
	}
	if !tt.HasText("Create your workspace") || !tt.HasText("wang's Team") {
		t.Fatalf("expected HubLayout create page from dropdown; got %q", tt.Texts())
	}
}

func TestCustomWindowsAppMenu(t *testing.T) {
	a, _ := newTestApp(t)
	quitCalled := false
	a.SetQuitFunc(func() {
		quitCalled = true
	})

	tt := ui.NewTester(a.View, 1280, 800)

	// 1. 标题栏上存在 App Menu 按钮，点击展开自绘浮层
	if err := tt.Click("App Menu"); err != nil {
		t.Fatalf("Click App Menu button: %v", err)
	}

	// 2. 检查 9 项菜单内容与快捷键标注是否均正确渲染（对齐 Pixso 节点 52:20243）
	expectedItems := []string{
		"New Request", "Ctrl+N",
		"New Collection", "Ctrl+Shift+N",
		"New Workspace",
		"Import...", "Ctrl+O",
		"Export...",
		"Settings...", "Ctrl+,",
		"Help",
		"About Postman",
		"Quit", "Alt+F4",
	}
	for _, item := range expectedItems {
		if !tt.HasText(item) {
			t.Fatalf("custom Windows app menu missing item %q; got texts: %q", item, tt.Texts())
		}
	}

	// 3. 点击 New Workspace 验证路由跳转及菜单关闭
	if err := tt.Click("New Workspace"); err != nil {
		t.Fatalf("Click New Workspace in app menu: %v", err)
	}
	tt.Advance(300 * time.Millisecond)
	if got := a.Router().Path(); got != routes.HubWorkspaceNew {
		t.Fatalf("after menu New Workspace path = %q, want %q", got, routes.HubWorkspaceNew)
	}
	if tt.HasText("Alt+F4") {
		t.Fatalf("menu popover should be closed after clicking New Workspace")
	}

	// 4. 再次打开菜单并点击 Quit，验证退出回调被触发
	if err := tt.Click("App Menu"); err != nil {
		t.Fatalf("reopen App Menu: %v", err)
	}
	if err := tt.Click("Quit"); err != nil {
		t.Fatalf("Click Quit menu item: %v", err)
	}
	if !quitCalled {
		t.Fatalf("expected quit function to be called on Quit menu item click")
	}
}

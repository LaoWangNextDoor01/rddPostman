package app

import (
	"encoding/json"
	"math"
	"os"
	"testing"
	"time"

	"rddpostman/features/settings"
	"rddpostman/internal/domain"
	"rddpostman/internal/routes"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func TestGlobalShortcutsAndModals(t *testing.T) {
	a, db := newTestApp(t)
	ws := db.ActiveWorkspace()
	if ws == nil {
		t.Fatal("expected default active workspace")
	}
	coll, err := db.CreateCollection(ws.ID, "Orders API", "")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	reqNode := &domain.Node{
		ID:   domain.NewID(),
		Kind: domain.KindRequest,
		Name: "Create Order",
		Request: &domain.Request{
			Method: "POST",
			URL:    "https://api.example.com/v1/orders",
		},
	}
	if err := db.AddNode(coll.ID, "", reqNode); err != nil {
		t.Fatalf("AddNode: %v", err)
	}

	tt := ui.NewTester(a.View, 1280, 800)

	// 1. Cmd+K 打开全局搜索，支持搜索并回车打开请求
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Frame()
	if !tt.HasText("Create Order") {
		t.Fatalf("Cmd+K global search should list request 'Create Order'; got %q", tt.Texts())
	}
	if err := tt.Click("Create Order"); err != nil {
		t.Fatalf("click search result Create Order: %v", err)
	}
	tt.Advance(250 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchCollectionsOf(string(ws.ID)) {
		t.Fatalf("after clicking search result path = %q, want %q", got, routes.WorkbenchCollectionsOf(string(ws.ID)))
	}
	if !tt.HasText("Create Order") {
		t.Fatalf("expected request tab 'Create Order' opened; got %q", tt.Texts())
	}

	// 2. Cmd+Shift+P 打开命令面板（仅命令）
	tt.Key(ui.Cmd|ui.Shift, ui.KeyP)
	tt.Frame()
	if !tt.HasText("Toggle Dark / Light Theme") {
		t.Fatalf("Cmd+Shift+P should open command palette; got %q", tt.Texts())
	}
	// Esc 关闭搜索弹窗
	tt.Key(0, ui.KeyEscape)
	tt.Frame()
	if tt.HasText("Toggle Dark / Light Theme") {
		t.Fatalf("Esc should close command palette; got %q", tt.Texts())
	}

	// 3. Cmd+, 打开设置弹窗，并切换主题 Dark / Light
	tt.Key(ui.Cmd, ui.KeyComma)
	tt.Frame()
	if !tt.HasText("Themes") {
		t.Fatalf("Cmd+, should open settings dialog; got %q", tt.Texts())
	}
	if err := tt.Click("Themes"); err != nil {
		t.Fatalf("Click Themes: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Dark"); err != nil {
		t.Fatalf("Click Dark theme: %v", err)
	}
	tt.Frame()
	if got := a.ThemeMode(); got != settings.ThemeDark {
		t.Fatalf("ThemeMode = %q, want %q", got, settings.ThemeDark)
	}
	tt.Key(0, ui.KeyEscape)
	tt.Frame()

	// 4. Cmd+1 / Cmd+2 / Cmd+3 / Cmd+E 切换工作台侧栏模块
	tt.Key(ui.Cmd, ui.Key2)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchEnvironmentsOf(string(ws.ID)) {
		t.Fatalf("Cmd+2 path = %q, want %q", got, routes.WorkbenchEnvironmentsOf(string(ws.ID)))
	}
	tt.Key(ui.Cmd, ui.Key3)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchHistoryOf(string(ws.ID)) {
		t.Fatalf("Cmd+3 path = %q, want %q", got, routes.WorkbenchHistoryOf(string(ws.ID)))
	}
	tt.Key(ui.Cmd, ui.Key1)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchCollectionsOf(string(ws.ID)) {
		t.Fatalf("Cmd+1 path = %q, want %q", got, routes.WorkbenchCollectionsOf(string(ws.ID)))
	}
	tt.Key(ui.Cmd, ui.KeyE)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchEnvironmentsOf(string(ws.ID)) {
		t.Fatalf("Cmd+E path = %q, want %q", got, routes.WorkbenchEnvironmentsOf(string(ws.ID)))
	}

	// 5. Alt+Left / Alt+Right 路由后退与前进
	tt.Key(ui.Alt, ui.KeyLeft)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchCollectionsOf(string(ws.ID)) {
		t.Fatalf("Alt+Left path = %q, want %q", got, routes.WorkbenchCollectionsOf(string(ws.ID)))
	}
	tt.Key(ui.Alt, ui.KeyRight)
	tt.Advance(200 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchEnvironmentsOf(string(ws.ID)) {
		t.Fatalf("Alt+Right path = %q, want %q", got, routes.WorkbenchEnvironmentsOf(string(ws.ID)))
	}

	// 6. Cmd+B 新建集合、Cmd+N 新建请求草稿、Cmd+O 打开导入弹窗
	beforeColls := len(db.ActiveWorkspace().Collections)
	tt.Key(ui.Cmd, ui.KeyB)
	tt.Advance(250 * time.Millisecond)
	if got := len(db.ActiveWorkspace().Collections); got != beforeColls+1 {
		t.Fatalf("Cmd+B collections = %d, want %d", got, beforeColls+1)
	}

	tt.Key(ui.Cmd, ui.KeyN)
	tt.Frame()
	if !tt.HasText("Enter URL or paste text") && !tt.HasText("Send") {
		t.Fatalf("Cmd+N should open a new request tab; got %q", tt.Texts())
	}

	tt.Key(ui.Cmd, ui.KeyO)
	tt.Frame()
	if !tt.HasText("Postman") {
		t.Fatalf("Cmd+O should open import dialog; got %q", tt.Texts())
	}
}

func TestGlobalsDialogAndExportDialog(t *testing.T) {
	a, db := newTestApp(t)
	ws := db.ActiveWorkspace()
	if _, err := db.CreateCollection(ws.ID, "Billing API", ""); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	tt := ui.NewTester(a.View, 1280, 800)

	// 1. 通过状态栏 Globals 按钮打开 Globals 弹窗，编辑并保存
	if err := tt.Click("Globals"); err != nil {
		t.Fatalf("Click Globals status bar button: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Shared variables accessible in every workspace via {{variable}}.") {
		t.Fatalf("Globals dialog did not open; got %q", tt.Texts())
	}
	if err := tt.Click("Global variable key"); err != nil {
		t.Fatalf("Click global key input: %v", err)
	}
	tt.Type("globalHost")
	if err := tt.Click("Global variable value"); err != nil {
		t.Fatalf("Click global value input: %v", err)
	}
	tt.Type("https://global.example.com")
	if err := tt.Click("Save Globals"); err != nil {
		t.Fatalf("Click Save Globals: %v", err)
	}
	tt.Frame()

	savedGlobals := db.Globals()
	if len(savedGlobals) != 1 || savedGlobals[0].Key != "globalHost" || savedGlobals[0].Value != "https://global.example.com" {
		t.Fatalf("saved globals = %+v, want globalHost=https://global.example.com", savedGlobals)
	}
	if err := tt.Click("Close globals"); err != nil {
		t.Fatalf("Click Close globals: %v", err)
	}
	tt.Frame()
	if !tt.HasText("Globals (1)") {
		t.Fatalf("status bar should reflect Globals (1); got %q", tt.Texts())
	}

	// 2. 打开 Export 弹窗，测试 JSON 预览与导出保存回调
	origSaver := saveExportFile
	defer func() { saveExportFile = origSaver }()

	var savedName string
	var savedPayload []byte
	done := make(chan struct{}, 1)
	saveExportFile = func(defaultName string, data []byte) (string, error) {
		savedName = defaultName
		savedPayload = append([]byte(nil), data...)
		done <- struct{}{}
		return "/tmp/" + defaultName, nil
	}

	a.OpenExport()
	tt.Frame()
	if !tt.HasText("Export Postman JSON") || !tt.HasText("Billing API") {
		t.Fatalf("Export dialog missing expected content; got %q", tt.Texts())
	}
	if err := tt.Click("Save to File"); err != nil {
		t.Fatalf("Click Save to File: %v", err)
	}
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for export save callback")
	}
	tt.Frame()

	if savedName != "Billing API.postman_collection.json" {
		t.Fatalf("savedName = %q, want Billing API.postman_collection.json", savedName)
	}
	var parsed map[string]any
	if err := json.Unmarshal(savedPayload, &parsed); err != nil {
		t.Fatalf("exported JSON invalid: %v", err)
	}
	if !tt.HasText("Saved to /tmp/Billing API.postman_collection.json") {
		t.Fatalf("expected save confirmation message; got %q", tt.Texts())
	}
}

func TestWorkspaceDropdownLayoutGeometry(t *testing.T) {
	a, db := newTestApp(t)
	if _, err := db.CreateWorkspace("Payments API"); err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	tt := ui.NewTester(a.View, 1280, 800)

	if err := tt.Click(locales.SwitchWorkspace()); err != nil {
		t.Fatalf("Click workspace selector: %v", err)
	}
	tt.Frame()

	createRect, ok := tt.Find(locales.Create())
	if !ok {
		t.Fatalf("could not find Create button; texts=%q", tt.Texts())
	}
	viewAllRect, ok := tt.Find(locales.ViewAll())
	if !ok {
		t.Fatalf("could not find View all button; texts=%q", tt.Texts())
	}

	if math.Abs(float64(createRect.W-viewAllRect.W)) > 2 {
		t.Fatalf("bottom buttons width asymmetric: create=%.1f viewAll=%.1f", createRect.W, viewAllRect.W)
	}
	if math.Abs(float64(createRect.H-viewAllRect.H)) > 1 || createRect.H < 34 || createRect.H > 40 {
		t.Fatalf("bottom buttons height unexpected: create=%.1f viewAll=%.1f (want ~36)", createRect.H, viewAllRect.H)
	}

	saveLayoutPNG(t, os.Getenv("LAYOUT_SNAPSHOT_DIR"), "ws-dropdown-compact.png", tt)
}

func TestWorkbenchToolsSidebarAndTabs(t *testing.T) {
	a, db := newTestApp(t)
	ws := db.ActiveWorkspace()
	tt := ui.NewTester(a.View, 1280, 800)

	// 1. Cmd+4 进入工作台 Tools 侧栏
	tt.Key(ui.Cmd, ui.Key4)
	tt.Advance(250 * time.Millisecond)
	if got := a.Router().Path(); got != routes.WorkbenchToolsOf(string(ws.ID)) {
		t.Fatalf("Cmd+4 path = %q, want %q", got, routes.WorkbenchToolsOf(string(ws.ID)))
	}
	if !tt.HasText(locales.ToolsCatData()) || !tt.HasText(locales.ToolJSONName()) || !tt.HasText(locales.ToolHashName()) {
		t.Fatalf("expected tools list in workbench sidebar; got %q", tt.Texts())
	}

	// 2. 点击侧栏中的哈希工具，右侧打开工具 Tab 并可实时计算
	if err := tt.Click(locales.ToolHashName()); err != nil {
		t.Fatalf("Click Hash tool: %v", err)
	}
	tt.Frame()
	if err := tt.Click("Hash input text"); err != nil {
		t.Fatalf("Click hash input: %v", err)
	}
	tt.Type("hello")
	tt.Frame()
	if !tt.HasText("5d41402abc4b2a76b9719d911017c592") {
		t.Fatalf("expected live MD5 result in workbench tool tab; got %q", tt.Texts())
	}
	saveLayoutPNG(t, os.Getenv("LAYOUT_SNAPSHOT_DIR"), "workbench-tools.png", tt)

	// 3. 切回 Collections 侧栏，右侧工具 Tab 依然保活不丢
	if err := tt.Click("Collections"); err != nil {
		t.Fatalf("Click Collections tab: %v", err)
	}
	tt.Advance(200 * time.Millisecond)
	if !tt.HasText("5d41402abc4b2a76b9719d911017c592") {
		t.Fatalf("expected tool tab preserved after switching sidebar back to Collections; got %q", tt.Texts())
	}
}

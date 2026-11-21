package workspace

import (
	"image/png"
	"os"
	"path/filepath"
	"testing"
	"time"

	"rddpostman/internal/routes"
	"rddpostman/internal/store"

	"github.com/egoist/mygo/ui"
)

type stubNav struct {
	path   string
	pushed []string
}

func (n *stubNav) Path() string       { return n.path }
func (n *stubNav) Push(target string) { n.path = target; n.pushed = append(n.pushed, target) }

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestWorkspacesListSearchFilterAndCreate(t *testing.T) {
	db := newTestStore(t)
	// 把默认工作区命名为接近设计稿的名字，便于断言
	def := db.Snapshot().Workspaces[0]
	if err := db.RenameWorkspace(def.ID, "wang's Workspace"); err != nil {
		t.Fatalf("RenameWorkspace: %v", err)
	}

	nav := &stubNav{path: routes.Workspace}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, db, nav)
	}, 1280, 740)

	// 1. 初始列表渲染：标题、Create workspace、表头列、真实数据行与底部计数
	for _, want := range []string{
		"Workspaces",
		"Create workspace",
		"All",
		"Project Workspaces",
		"External",
		"Name",
		"Creator",
		"Contributors",
		"Last activity",
		"Access",
		"Your role",
		"wang's Workspace",
		"Internal",
		"Everyone in your team",
		"Admin",
		"1 workspace",
	} {
		if !tt.HasText(want) {
			t.Errorf("initial list missing text %q; got %q", want, tt.Texts())
		}
	}

	// 可选：输出浅色 / 深色 / 创建页截图用于人工视觉对比
	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "ws-list-light.png"), tt)
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "ws-list-dark.png"), tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}

	// 2. 点击 Create workspace 进入创建表单页（自动聚焦名称输入框）
	if err := tt.Click("Create workspace"); err != nil {
		t.Fatalf("Click Create workspace: %v", err)
	}
	if !tt.HasText("Create your workspace") {
		t.Fatalf("expected create form title; got %q", tt.Texts())
	}

	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "ws-create-light.png"), tt)
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "ws-create-dark.png"), tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}

	// 直接输入名称（验证自动聚焦），再选择 Partner 类型并创建
	tt.Type("Payments API")
	if err := tt.Click("Partner"); err != nil {
		t.Fatalf("Click Partner: %v", err)
	}
	if err := tt.Click("Create Workspace"); err != nil {
		t.Fatalf("Click Create Workspace: %v", err)
	}

	// 创建后返回列表，显示 2 个工作区
	if !tt.HasText("Payments API") || !tt.HasText("2 workspaces") {
		t.Fatalf("expected newly created workspace in list; got %q", tt.Texts())
	}

	// 3. 切到 External Tab 只显示 Partner 工作区
	if err := tt.Click("External"); err != nil {
		t.Fatalf("Click External tab: %v", err)
	}
	if !tt.HasText("Payments API") || tt.HasText("wang's Workspace") || !tt.HasText("1 workspace") {
		t.Fatalf("External tab filter mismatch; got %q", tt.Texts())
	}

	// 切回 All Tab
	if err := tt.Click("All"); err != nil {
		t.Fatalf("Click All tab: %v", err)
	}
	if !tt.HasText("2 workspaces") {
		t.Fatalf("All tab should show 2 workspaces; got %q", tt.Texts())
	}

	// 4. 搜索框过滤与空态
	if err := tt.Click("Search workspaces"); err != nil {
		t.Fatalf("Click Search workspaces: %v", err)
	}
	tt.Type("Payments")
	if !tt.HasText("Payments API") || tt.HasText("wang's Workspace") {
		t.Fatalf("search filter failed; got %q", tt.Texts())
	}

	tt.Type("XYZNotFound")
	if !tt.HasText("No matching workspaces") {
		t.Fatalf("expected empty search state; got %q", tt.Texts())
	}
	if err := tt.Click("Clear filters"); err != nil {
		t.Fatalf("Click Clear filters: %v", err)
	}
	if !tt.HasText("2 workspaces") {
		t.Fatalf("Clear filters did not restore list; got %q", tt.Texts())
	}

	// 5. 全选与单选复选框
	if err := tt.Click("Select all workspaces"); err != nil {
		t.Fatalf("Click select all: %v", err)
	}
	if !tt.HasText("· 2 selected") {
		t.Fatalf("expected 2 selected; got %q", tt.Texts())
	}
	if err := tt.Click("Clear selection"); err != nil {
		t.Fatalf("Click Clear selection: %v", err)
	}
	if tt.HasText("selected") {
		t.Fatalf("expected selection cleared; got %q", tt.Texts())
	}

	// 6. 点击工作区名称进入工作台（触发路由跳转并切换活跃工作区）
	if err := tt.Click("Payments API"); err != nil {
		t.Fatalf("Click Payments API: %v", err)
	}
	if len(nav.pushed) == 0 || nav.pushed[len(nav.pushed)-1] != routes.Collections {
		t.Fatalf("expected navigation to %q, got %v", routes.Collections, nav.pushed)
	}

	// 7. 切换侧栏菜单项渲染占位页并切回 Workspaces
	if err := tt.Click("API Catalog"); err != nil {
		t.Fatalf("Click API Catalog: %v", err)
	}
	if !tt.HasText("Browse and manage your organization's API catalog.") {
		t.Fatalf("expected API Catalog placeholder; got %q", tt.Texts())
	}
	if err := tt.Click("Workspaces"); err != nil {
		t.Fatalf("Click Workspaces: %v", err)
	}
	if !tt.HasText("Create workspace") {
		t.Fatalf("expected return to Workspaces list; got %q", tt.Texts())
	}
}

func TestWorkspaceRenameAndDelete(t *testing.T) {
	db := newTestStore(t)
	extra, err := db.CreateWorkspace("Temp Space")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	nav := &stubNav{path: routes.Workspace}
	tt := ui.NewTester(func(c *ui.Context) {
		View(c, db, nav)
	}, 1280, 740)

	// 打开更多菜单并重命名
	if err := tt.Click("More actions for Temp Space"); err != nil {
		t.Fatalf("Click more actions: %v", err)
	}
	if err := tt.Click("Rename"); err != nil {
		t.Fatalf("Click Rename: %v", err)
	}
	if !tt.HasText("Rename workspace") {
		t.Fatalf("expected Rename dialog; got %q", tt.Texts())
	}
	if err := tt.Click("Workspace name"); err != nil {
		t.Fatalf("Click rename input: %v", err)
	}
	tt.Key(ui.Cmd, ui.KeyA)
	tt.Type("Renamed Space")
	if err := tt.Click("Save"); err != nil {
		t.Fatalf("Click Save: %v", err)
	}
	if !tt.HasText("Renamed Space") {
		t.Fatalf("expected renamed workspace; got %q", tt.Texts())
	}
	if got := db.Snapshot().Workspace(extra.ID); got == nil || got.Name != "Renamed Space" {
		t.Fatalf("store did not persist rename: %+v", got)
	}

	// 删除非默认工作区
	if err := tt.Click("More actions for Renamed Space"); err != nil {
		t.Fatalf("Click more actions after rename: %v", err)
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatalf("Click Delete menu item: %v", err)
	}
	if !tt.HasText("Delete workspace") {
		t.Fatalf("expected Delete confirm dialog; got %q", tt.Texts())
	}
	if err := tt.Click("Delete"); err != nil {
		t.Fatalf("Click confirm Delete: %v", err)
	}
	if tt.HasText("Renamed Space") {
		t.Fatalf("deleted workspace still visible; got %q", tt.Texts())
	}
	if got := db.Snapshot().Workspace(extra.ID); got != nil {
		t.Fatalf("workspace still present in store after delete")
	}
}

func savePNG(t *testing.T, path string, tt *ui.Tester) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatalf("Create %s: %v", path, err)
	}
	defer f.Close()
	if err := png.Encode(f, tt.Image()); err != nil {
		t.Fatalf("png.Encode: %v", err)
	}
}

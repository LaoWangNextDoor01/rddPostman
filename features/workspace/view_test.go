package workspace

import (
	"image"
	"image/color"
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

func writeTestIconPNG(t *testing.T, dir, filename string) string {
	t.Helper()
	p := filepath.Join(dir, filename)
	img := image.NewRGBA(image.Rect(0, 0, 48, 48))
	c := color.RGBA{R: 16, G: 185, B: 129, A: 255} // #10B981 emerald test icon
	for y := range 48 {
		for x := range 48 {
			img.Set(x, y, c)
		}
	}
	f, err := os.Create(p)
	if err != nil {
		t.Fatalf("create test png: %v", err)
	}
	defer f.Close()
	if err := png.Encode(f, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}
	return p
}

func renderWorkspaceRoute(c *ui.Context, db *store.Store, nav *stubNav) {
	if nav.Path() == routes.HubWorkspaceNew {
		CreateView(c, db, nav)
		return
	}
	ListView(c, db, nav)
}

func TestWorkspacesListSearchFilterCreateAndCustomIcon(t *testing.T) {
	db := newTestStore(t)
	def := db.Snapshot().Workspaces[0]
	if err := db.RenameWorkspace(def.ID, "wang's Workspace"); err != nil {
		t.Fatalf("RenameWorkspace: %v", err)
	}

	// 准备一张真实本地 PNG 测试图标，替换默认文件选择器
	iconPath := writeTestIconPNG(t, t.TempDir(), "payments-logo.png")
	prevPicker := openImagePicker
	openImagePicker = func() (string, error) { return iconPath, nil }
	t.Cleanup(func() { openImagePicker = prevPicker })

	nav := &stubNav{path: routes.HubWorkspaces}
	tt := ui.NewTester(func(c *ui.Context) {
		renderWorkspaceRoute(c, db, nav)
	}, 1280, 740)

	// 1. 初始列表渲染：仅保留精简后的离线列（Name / Creator / Last activity）
	for _, want := range []string{
		"Workspaces",
		"Create workspace",
		"All",
		"Project Workspaces",
		"External",
		"Name",
		"Creator",
		"Last activity",
		"wang's Workspace",
		"Internal",
		"You",
		"1 workspace",
	} {
		if !tt.HasText(want) {
			t.Errorf("initial list missing text %q; got %q", want, tt.Texts())
		}
	}
	for _, removed := range []string{"Contributors", "Access", "Your role"} {
		if tt.HasText(removed) {
			t.Errorf("offline list should not show removed column %q", removed)
		}
	}

	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "list-light.png"), tt)
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "list-dark.png"), tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}

	// 2. 点击 Create workspace 进入创建表单页
	if err := tt.Click("Create workspace"); err != nil {
		t.Fatalf("Click Create workspace: %v", err)
	}
	for _, want := range []string{"Create your workspace", "Workspace icon", "Choose local image", "Description", "(Optional)"} {
		if !tt.HasText(want) {
			t.Fatalf("create page missing %q; got %q", want, tt.Texts())
		}
	}
	for _, removed := range []string{"Who can access", "Create with Postman AI"} {
		if tt.HasText(removed) {
			t.Fatalf("create page should not show removed section %q", removed)
		}
	}

	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "create-default-light.png"), tt)
	}

	// 选择本地图片图标
	if err := tt.Click("Choose local image"); err != nil {
		t.Fatalf("Click Choose local image: %v", err)
	}
	// 等待后台 goroutine 返回并推进帧回收结果
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("Change image") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("Change image") || !tt.HasText("Remove") {
		t.Fatalf("expected custom icon selected state; got %q", tt.Texts())
	}

	// 输入名称、选择 Partner、填写 Description
	tt.Type("Payments API")
	if err := tt.Click("Partner"); err != nil {
		t.Fatalf("Click Partner: %v", err)
	}
	if err := tt.Click("Workspace description"); err != nil {
		t.Fatalf("Click Workspace description: %v", err)
	}
	tt.Type("Offline payment gateway collections.")

	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		savePNG(t, filepath.Join(dir, "create-filled-light.png"), tt)
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "create-filled-dark.png"), tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}

	if err := tt.Click("Create Workspace"); err != nil {
		t.Fatalf("Click Create Workspace: %v", err)
	}

	// 校验列表与持久化数据（Type、Description、Icon）
	if !tt.HasText("Payments API") || !tt.HasText("2 workspaces") {
		t.Fatalf("expected newly created workspace in list; got %q", tt.Texts())
	}
	var createdWS *store.Workspace
	for _, w := range db.Snapshot().Workspaces {
		if w.Name == "Payments API" {
			createdWS = w
			break
		}
	}
	if createdWS == nil {
		t.Fatal("Payments API not found in store")
	}
	if createdWS.Type != "Partner" {
		t.Errorf("Type = %q, want Partner", createdWS.Type)
	}
	if createdWS.Description != "Offline payment gateway collections." {
		t.Errorf("Description = %q, want Offline payment gateway collections.", createdWS.Description)
	}
	if createdWS.Icon == "" || createdWS.Icon == iconPath {
		t.Fatalf("Icon should be copied into workspace dir, got %q (source %q)", createdWS.Icon, iconPath)
	}
	if filepath.Base(createdWS.Icon) != ".rddpostman.icon.png" {
		t.Errorf("copied icon basename = %q, want .rddpostman.icon.png", filepath.Base(createdWS.Icon))
	}
	// 删除源文件后，工作区目录内的副本应仍可读且界面正常渲染
	if err := os.Remove(iconPath); err != nil {
		t.Fatalf("remove source icon: %v", err)
	}
	if _, err := os.Stat(createdWS.Icon); err != nil {
		t.Fatalf("copied workspace icon missing after deleting source: %v", err)
	}
	tt.Frame()

	// 3. 收藏星标持久化
	if err := tt.Click("Favorite Payments API"); err != nil {
		t.Fatalf("Click Favorite: %v", err)
	}
	if got := db.Snapshot().Workspace(createdWS.ID); got == nil || !got.Starred {
		t.Fatalf("expected Payments API to be starred in store")
	}

	// 4. 切到 External Tab 只显示 Partner 工作区
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

	// 5. 全选复选框 + 选中背景态截图
	if err := tt.Click("Select all workspaces"); err != nil {
		t.Fatalf("Click select all: %v", err)
	}
	if !tt.HasText("· 2 selected") {
		t.Fatalf("expected 2 selected; got %q", tt.Texts())
	}
	if dir := os.Getenv("WS_SNAPSHOT_DIR"); dir != "" {
		tt.Advance(250 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "list-selected-light.png"), tt)
		tt.SetDark(true)
		tt.Advance(300 * time.Millisecond)
		savePNG(t, filepath.Join(dir, "list-selected-dark.png"), tt)
		tt.SetDark(false)
		tt.Advance(300 * time.Millisecond)
	}
	if err := tt.Click("Clear selection"); err != nil {
		t.Fatalf("Click Clear selection: %v", err)
	}
	if tt.HasText("selected") {
		t.Fatalf("expected selection cleared; got %q", tt.Texts())
	}

	// 6. 搜索框过滤与空态
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

	// 7. 点击工作区名称进入对应工作台路由
	if err := tt.Click("Payments API"); err != nil {
		t.Fatalf("Click Payments API: %v", err)
	}
	wantTarget := routes.WorkbenchCollectionsOf(string(createdWS.ID))
	if len(nav.pushed) == 0 || nav.pushed[len(nav.pushed)-1] != wantTarget {
		t.Fatalf("expected navigation to %q, got %v", wantTarget, nav.pushed)
	}
}

func TestWorkspaceRenameAndDelete(t *testing.T) {
	db := newTestStore(t)
	extra, err := db.CreateWorkspace("Temp Space")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	nav := &stubNav{path: routes.HubWorkspaces}
	tt := ui.NewTester(func(c *ui.Context) {
		renderWorkspaceRoute(c, db, nav)
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

func TestCreateWorkspaceOpenLocalFolder(t *testing.T) {
	t.Setenv("RDDPOSTMAN_DATA_DIR", t.TempDir())
	db := newTestStore(t)

	// 准备两个真实本地目录：一个在候选中展示，一个通过 Open New Folder 选择
	baseDir := t.TempDir()
	listedDir := filepath.Join(baseDir, "fe-topic")
	pickedDir := filepath.Join(baseDir, "custom-api-repo")
	if err := os.MkdirAll(listedDir, 0o755); err != nil {
		t.Fatalf("mkdir listedDir: %v", err)
	}
	if err := os.MkdirAll(pickedDir, 0o755); err != nil {
		t.Fatalf("mkdir pickedDir: %v", err)
	}

	prevLoader := loadLocalFolders
	prevPicker := openFolderPicker
	loadLocalFolders = func() []localFolderItem {
		return []localFolderItem{{Name: "fe-topic", Path: listedDir, Recent: true}}
	}
	openFolderPicker = func(string) (string, error) {
		return pickedDir, nil
	}
	t.Cleanup(func() {
		loadLocalFolders = prevLoader
		openFolderPicker = prevPicker
	})

	nav := &stubNav{path: routes.HubWorkspaces}
	tt := ui.NewTester(func(c *ui.Context) {
		renderWorkspaceRoute(c, db, nav)
	}, 1280, 740)

	// 1. 进入创建页，等待异步目录扫描填充候选列表
	if err := tt.Click("Create workspace"); err != nil {
		t.Fatalf("Click Create workspace: %v", err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("fe-topic") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("fe-topic") {
		t.Fatalf("expected discovered folder fe-topic; got %q", tt.Texts())
	}

	// 2. 点击列表中的目录项：自动选中并带入名称，提交后在 listedDir 初始化 workspace.json
	if err := tt.Click("fe-topic"); err != nil {
		t.Fatalf("Click fe-topic: %v", err)
	}
	if err := tt.Click("Create Workspace"); err != nil {
		t.Fatalf("Click Create Workspace for listedDir: %v", err)
	}
	if !tt.HasText("fe-topic") {
		t.Fatalf("expected created workspace fe-topic in list; got %q", tt.Texts())
	}
	if _, err := os.Stat(filepath.Join(listedDir, "workspace.json")); err != nil {
		t.Fatalf("expected workspace.json in %s: %v", listedDir, err)
	}

	// 3. 再次进入创建页，点击 Open New Folder 选择新目录并创建
	if err := tt.Click("Create workspace"); err != nil {
		t.Fatalf("Click Create workspace again: %v", err)
	}
	if err := tt.Click("Open New Folder"); err != nil {
		t.Fatalf("Click Open New Folder: %v", err)
	}
	deadline = time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		tt.Frame()
		if tt.HasText("custom-api-repo") {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !tt.HasText("custom-api-repo") {
		t.Fatalf("expected picked folder custom-api-repo; got %q", tt.Texts())
	}
	if err := tt.Click("Create Workspace"); err != nil {
		t.Fatalf("Click Create Workspace for pickedDir: %v", err)
	}
	if !tt.HasText("custom-api-repo") {
		t.Fatalf("expected created workspace custom-api-repo in list; got %q", tt.Texts())
	}
	if _, err := os.Stat(filepath.Join(pickedDir, "workspace.json")); err != nil {
		t.Fatalf("expected workspace.json in %s: %v", pickedDir, err)
	}

	// 校验 recent-folders.json 已记住新选目录
	recents := readRecentFolders()
	if len(recents) == 0 || normalizePathKey(recents[0]) != normalizePathKey(pickedDir) {
		t.Fatalf("recent folders = %v, want %q first", recents, pickedDir)
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

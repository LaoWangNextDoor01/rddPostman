package environment

import (
	"testing"

	"rddpostman/internal/domain"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func newTestStore(t *testing.T) *store.Store {
	t.Helper()
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func TestEnvironmentSidebarStates(t *testing.T) {
	db := newTestStore(t)
	ws, err := db.CreateWorkspace("Test Workspace")
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}

	envA, err := db.CreateEnvironment(ws.ID, "Environment A")
	if err != nil {
		t.Fatalf("CreateEnvironment A failed: %v", err)
	}
	envB, err := db.CreateEnvironment(ws.ID, "Environment B")
	if err != nil {
		t.Fatalf("CreateEnvironment B failed: %v", err)
	}

	// 初始状态：让 envA 为激活环境
	if err := db.SelectEnvironment(ws.ID, envA.ID); err != nil {
		t.Fatalf("SelectEnvironment failed: %v", err)
	}

	var openedEnv domain.ID
	openEnv := func(id domain.ID) {
		openedEnv = id
	}

	view := func(c *ui.Context) {
		SidebarView(c, db, ws.ID, openEnv)
	}

	tt := ui.NewTester(view, 320, 600)
	tt.Frame()

	// 1. 初次渲染验证：
	// - 应该能找到 "Environment A" 和 "Environment B"
	if !tt.HasText("Environment A") || !tt.HasText("Environment B") {
		t.Fatalf("期望侧栏显示两个环境名，实际文本: %q", tt.Texts())
	}

	// 2. 点击 envB 行：选中态测试
	// 点击 envB 行后，openEnv 应当被调用为 envB.ID（用于在右侧打开编辑 Tab）
	if err := tt.Click("Environment B"); err != nil {
		t.Fatalf("点击 envB 行失败: %v", err)
	}
	tt.Frame()

	if openedEnv != envB.ID {
		t.Fatalf("点击 envB 行后期望打开 envB，实际打开了: %v", openedEnv)
	}

	// 此时：
	// envA 仍是生效环境（当前生效状态由后面对号标示）
	// envB 仅是被点击查看/编辑的环境（选中背景色状态）
	// 验证 store 中的 SelectedEnvironmentID 仍是 envA
	snap := db.Snapshot()
	curWS := snap.Workspace(ws.ID)
	if curWS.SelectedEnvironmentID != envA.ID {
		t.Fatalf("点击查看 envB 不应改变生效环境，期望生效仍为 envA, 实际: %v", curWS.SelectedEnvironmentID)
	}

	// 3. 点击 envB 右侧的激活按钮：切换生效态
	if err := tt.Click("Activate Environment B"); err != nil {
		t.Fatalf("点击 envB 激活按钮失败: %v", err)
	}
	tt.Frame()

	snap = db.Snapshot()
	curWS = snap.Workspace(ws.ID)
	if curWS.SelectedEnvironmentID != envB.ID {
		t.Fatalf("点击 envB 激活按钮后，期望生效环境切换为 envB, 实际: %v", curWS.SelectedEnvironmentID)
	}

	// 4. 点击 envA 激活按钮：切回 envA 生效
	if err := tt.Click("Activate Environment A"); err != nil {
		t.Fatalf("点击 envA 激活按钮失败: %v", err)
	}
	tt.Frame()

	snap = db.Snapshot()
	curWS = snap.Workspace(ws.ID)
	if curWS.SelectedEnvironmentID != envA.ID {
		t.Fatalf("点击 envA 激活按钮后，期望生效环境切换为 envA, 实际: %v", curWS.SelectedEnvironmentID)
	}
}

func TestEnvironmentEditorDeleteConfirm(t *testing.T) {
	db := newTestStore(t)
	ws, err := db.CreateWorkspace("Test Workspace")
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	env, err := db.CreateEnvironment(ws.ID, "Staging")
	if err != nil {
		t.Fatalf("CreateEnvironment failed: %v", err)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		EditorView(c, db, ws.ID, env.ID)
	}, 900, 600)

	// 通过顶部「更多操作」菜单点击 Delete 只弹出二次确认框，不直接删除
	if err := tt.Click(locales.MoreActions()); err != nil {
		t.Fatalf("Click MoreActions: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.TreeDelete()); err != nil {
		t.Fatalf("Click Delete: %v", err)
	}
	tt.Frame()
	if !tt.HasText(locales.EnvDeleteTitle()) {
		t.Fatalf("expected delete confirmation dialog; got %q", tt.Texts())
	}
	if len(db.Snapshot().Workspace(ws.ID).Environments) != 2 { // 默认环境 + Staging
		t.Fatalf("environment should not be deleted before confirmation")
	}

	// 点击取消后环境依然存在
	if err := tt.Click(locales.Cancel()); err != nil {
		t.Fatalf("Click Cancel: %v", err)
	}
	tt.Frame()
	if tt.HasText(locales.EnvDeleteTitle()) {
		t.Fatalf("confirm dialog should close after cancel")
	}

	// 再次点击更多操作 -> Delete -> 确认删除 -> 环境被移除
	if err := tt.Click(locales.MoreActions()); err != nil {
		t.Fatalf("Click MoreActions again: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.TreeDelete()); err != nil {
		t.Fatalf("Click Delete again: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.ConfirmDelete()); err != nil {
		t.Fatalf("Click confirm delete: %v", err)
	}
	tt.Frame()
	for _, e := range db.Snapshot().Workspace(ws.ID).Environments {
		if e.ID == env.ID {
			t.Fatalf("environment %q still present after confirming delete", env.Name)
		}
	}
}

func TestEnvironmentEditorDeleteVariableRow(t *testing.T) {
	db := newTestStore(t)
	ws, err := db.CreateWorkspace("Test Workspace")
	if err != nil {
		t.Fatalf("CreateWorkspace failed: %v", err)
	}
	env, err := db.CreateEnvironment(ws.ID, "Staging")
	if err != nil {
		t.Fatalf("CreateEnvironment failed: %v", err)
	}
	env.Vars = []domain.KV{
		{ID: domain.NewID(), Key: "baseUrl", Value: "https://staging.example.com", Enabled: true},
		{ID: domain.NewID(), Key: "apiToken", Value: "secret-123", Enabled: true, Secret: true},
	}
	if err := db.SaveEnvironment(ws.ID, env); err != nil {
		t.Fatalf("SaveEnvironment failed: %v", err)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		EditorView(c, db, ws.ID, env.ID)
	}, 960, 600)
	tt.Frame()

	// 点击首行后面的删除按钮应立即移除 baseUrl 并标记为未保存
	if err := tt.Click(locales.DeleteRowLabel()); err != nil {
		t.Fatalf("Click delete row: %v", err)
	}
	tt.Frame()

	if !tt.HasText(locales.EnvUnsaved()) {
		t.Fatalf("expected unsaved badge after deleting variable row; got %q", tt.Texts())
	}

	// 保存后持久化结果应仅保留 apiToken
	if err := tt.Click(locales.Save()); err != nil {
		t.Fatalf("Click Save: %v", err)
	}
	tt.Frame()

	var savedEnv *domain.Environment
	for _, e := range db.Snapshot().Workspace(ws.ID).Environments {
		if e.ID == env.ID {
			cp := e
			savedEnv = &cp
			break
		}
	}
	if savedEnv == nil {
		t.Fatalf("saved environment not found")
	}
	if len(savedEnv.Vars) != 1 || savedEnv.Vars[0].Key != "apiToken" {
		t.Fatalf("expected only apiToken to remain after row delete, got %+v", savedEnv.Vars)
	}
}

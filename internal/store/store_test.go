package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"rddpostman/internal/domain"
)

func newStoreAt(t *testing.T) (*Store, string) {
	t.Helper()
	dir := t.TempDir()
	s, err := OpenAt(dir)
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s, dir
}

func reopen(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := OpenAt(dir)
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func defaultWS(t *testing.T, s *Store) *Workspace {
	t.Helper()
	for _, w := range s.Snapshot().Workspaces {
		if w.Default {
			return w
		}
	}
	t.Fatal("没有默认工作区")
	return nil
}

func workspaceFile(dir string) string {
	return filepath.Join(dir, "workspaces", "default", "workspace.json")
}

func collectionDir(dir, name string) string {
	return filepath.Join(dir, "workspaces", "default", "collections", name)
}

func TestDefaultWorkspaceOnDisk(t *testing.T) {
	s, dir := newStoreAt(t)
	snap := s.Snapshot()
	if len(snap.Workspaces) != 1 {
		t.Fatalf("首次启动应有 1 个工作区，得到 %d", len(snap.Workspaces))
	}
	ws := snap.Workspaces[0]
	if !ws.Default || ws.Name != "默认工作区" {
		t.Fatalf("默认工作区不对: %+v", ws)
	}
	if _, err := os.Stat(workspaceFile(dir)); err != nil {
		t.Fatalf("工作区目录里应该有 workspace.json: %v", err)
	}
	if err := s.RemoveWorkspace(ws.ID); !errors.Is(err, ErrDefaultWorkspace) {
		t.Errorf("删除默认工作区应返回 ErrDefaultWorkspace，得到 %v", err)
	}

	// 改名后重启仍然是默认工作区。
	if err := s.RenameWorkspace(ws.ID, "我的工作区"); err != nil {
		t.Fatalf("RenameWorkspace: %v", err)
	}
	s2 := reopen(t, dir)
	if got := defaultWS(t, s2); got.Name != "我的工作区" {
		t.Errorf("默认工作区名 = %q", got.Name)
	}
}

func TestCollectionAndRequestsAreFiles(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	coll, err := s.CreateCollection(ws.ID, "开发者平台 API", "描述")
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	collDir := collectionDir(dir, "开发者平台 API")
	if _, err := os.Stat(filepath.Join(collDir, "collection.json")); err != nil {
		t.Fatalf("集合目录里应该有 collection.json: %v", err)
	}

	folder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Users"}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatalf("AddNode folder: %v", err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "获取用户",
		Request: &domain.Request{Method: "GET", URL: "https://api.example.com/users"}}
	if err := s.AddNode(coll.ID, folder.ID, req); err != nil {
		t.Fatalf("AddNode request: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "Users", "folder.json")); err != nil {
		t.Fatalf("目录应该有 folder.json: %v", err)
	}
	reqPath := filepath.Join(collDir, "Users", "获取用户.request.json")
	if _, err := os.Stat(reqPath); err != nil {
		t.Fatalf("请求应该是独立文件: %v", err)
	}

	if err := s.UpdateRequest(coll.ID, req.ID, func(r *domain.Request) {
		r.URL = "{{baseUrl}}/users/{{userId}}"
		r.Query = []domain.KV{{ID: "q1", Key: "page", Value: "1", Enabled: true}}
	}); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	s2 := reopen(t, dir)
	ws2 := defaultWS(t, s2)
	if len(ws2.Collections) != 1 {
		t.Fatalf("重新加载后有 %d 个集合，想要 1", len(ws2.Collections))
	}
	got := domain.FindIn(ws2.Collections[0].Items, req.ID)
	if got == nil || got.Request.URL != "{{baseUrl}}/users/{{userId}}" {
		t.Fatalf("请求没持久化正确: %+v", got)
	}
	if got.Request.Query[0].Key != "page" {
		t.Errorf("query 没持久化: %+v", got.Request.Query)
	}
	if domain.CountRequests(ws2.Collections[0].Items) != 1 {
		t.Errorf("请求数量不对")
	}
}

func TestRenameAndDeleteNodes(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	folder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "旧目录"}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "旧名字",
		Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(coll.ID, folder.ID, req); err != nil {
		t.Fatal(err)
	}

	if err := s.RenameNode(coll.ID, req.ID, "新名字"); err != nil {
		t.Fatalf("RenameNode: %v", err)
	}
	collDir := collectionDir(dir, "集合")
	if _, err := os.Stat(filepath.Join(collDir, "旧目录", "旧名字.request.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("旧请求文件应该已改名: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "旧目录", "新名字.request.json")); err != nil {
		t.Errorf("新请求文件不存在: %v", err)
	}

	if err := s.DeleteNode(coll.ID, folder.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "旧目录")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("目录应该连同子树一起删除: %v", err)
	}
	if domain.FindIn(defaultWS(t, s).Collections[0].Items, folder.ID) != nil {
		t.Errorf("内存里的目录没删掉")
	}
}

func TestPendingUpdateDoesNotResurrectDeletedNode(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	folder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "目录"}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "旧名字",
		Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(coll.ID, folder.ID, req); err != nil {
		t.Fatal(err)
	}

	// 先产生一个防抖中的修改，再改名/删除，落盘时不能把旧状态写回去。
	if err := s.UpdateRequest(coll.ID, req.ID, func(r *domain.Request) {
		r.URL = "https://example.com/new"
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.RenameNode(coll.ID, req.ID, "新名字"); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	collDir := collectionDir(dir, "集合")
	if _, err := os.Stat(filepath.Join(collDir, "目录", "新名字.request.json")); err != nil {
		t.Errorf("改名后的文件不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "目录", "旧名字.request.json")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("旧文件不应该被防抖副本写回来: %v", err)
	}

	if err := s.UpdateRequest(coll.ID, req.ID, func(r *domain.Request) {
		r.URL = "https://example.com/later"
	}); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNode(coll.ID, folder.ID); err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	s2 := reopen(t, dir)
	if got := domain.CountRequests(defaultWS(t, s2).Collections[0].Items); got != 0 {
		t.Errorf("已删除目录下的请求不应被写回，当前有 %d 个请求", got)
	}
}

func TestRenameAndRemoveCollection(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "旧集合", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RenameCollection(coll.ID, "新集合"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	if _, err := os.Stat(collectionDir(dir, "旧集合")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("旧集合目录应该已改名: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(collectionDir(dir, "新集合"), "collection.json"))
	if err != nil {
		t.Fatalf("新集合文件不存在: %v", err)
	}
	var onDisk domain.Collection
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Name != "新集合" {
		t.Errorf("文件里的集合名 = %q", onDisk.Name)
	}

	if err := s.RemoveCollection(coll.ID); err != nil {
		t.Fatalf("RemoveCollection: %v", err)
	}
	if _, err := os.Stat(collectionDir(dir, "新集合")); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("集合目录应该被删除: %v", err)
	}
}

func TestDuplicateNamesGetSuffix(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	c1, err := s.CreateCollection(ws.ID, "重复", "")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := s.CreateCollection(ws.ID, "重复", "")
	if err != nil {
		t.Fatal(err)
	}
	if c1.ID == c2.ID {
		t.Fatal("两个集合应有不同 ID")
	}
	if _, err := os.Stat(collectionDir(dir, "重复")); err != nil {
		t.Errorf("第一个集合目录不存在: %v", err)
	}
	if _, err := os.Stat(collectionDir(dir, "重复 (2)")); err != nil {
		t.Errorf("同名集合应该加序号: %v", err)
	}

	r1 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "同名",
		Request: &domain.Request{Method: "GET"}}
	r2 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "同名",
		Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(c1.ID, "", r1); err != nil {
		t.Fatal(err)
	}
	if err := s.AddNode(c1.ID, "", r2); err != nil {
		t.Fatal(err)
	}
	collDir := collectionDir(dir, "重复")
	if _, err := os.Stat(filepath.Join(collDir, "同名.request.json")); err != nil {
		t.Errorf("第一个请求文件不存在: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "同名 (2).request.json")); err != nil {
		t.Errorf("同名请求应该加序号: %v", err)
	}
}

func TestVariablesLayersAndSecrets(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	if err := s.SetGlobals([]domain.KV{
		{ID: "g1", Key: "host", Value: "global.example.com", Enabled: true},
		{ID: "g2", Key: "onlyGlobal", Value: "global", Enabled: true},
	}); err != nil {
		t.Fatalf("SetGlobals: %v", err)
	}
	if err := s.UpdateWorkspace(ws.ID, func(w *domain.Workspace) {
		w.Variables = []domain.KV{{ID: "w1", Key: "onlyWorkspace", Value: "workspace", Enabled: true}}
	}); err != nil {
		t.Fatalf("UpdateWorkspace: %v", err)
	}
	env, err := s.CreateEnvironment(ws.ID, "dev")
	if err != nil {
		t.Fatalf("CreateEnvironment: %v", err)
	}
	env.Vars = []domain.KV{
		{ID: "e1", Key: "host", Value: "env.example.com", Enabled: true},
		{ID: "e2", Key: "token", Value: "secret-token", Enabled: true, Secret: true},
		{ID: "e3", Key: "onlyEnv", Value: "initial", Enabled: true},
	}
	if err := s.SaveEnvironment(ws.ID, env); err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}
	if err := s.SelectEnvironment(ws.ID, env.ID); err != nil {
		t.Fatalf("SelectEnvironment: %v", err)
	}
	if err := s.SetCurrentValue(ws.ID, "e3", "current"); err != nil {
		t.Fatalf("SetCurrentValue: %v", err)
	}

	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollection(coll.ID, func(c *domain.Collection) {
		c.Variables = []domain.KV{{ID: "c1", Key: "host", Value: "collection.example.com", Enabled: true}}
	}); err != nil {
		t.Fatalf("UpdateCollection: %v", err)
	}
	folder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Users",
		Variables: []domain.KV{{ID: "f1", Key: "onlyFolder", Value: "folder", Enabled: true}}}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "获取用户",
		Variables: []domain.KV{{ID: "r1", Key: "onlyRequest", Value: "request", Enabled: true}},
		Request: &domain.Request{
			Method: "GET",
			URL:    "https://{{host}}/{{onlyGlobal}}/{{onlyWorkspace}}/{{onlyFolder}}/{{onlyRequest}}",
			Headers: []domain.KV{
				{ID: "h1", Key: "Authorization", Value: "Bearer {{token}}", Enabled: true, Secret: true},
			},
		}}
	if err := s.AddNode(coll.ID, folder.ID, req); err != nil {
		t.Fatal(err)
	}

	vars := s.Variables(ws.ID, coll.ID, req.ID)
	if vars["host"].Value != "collection.example.com" || vars["host"].Source != domain.VarCollection {
		t.Errorf("host 应被集合层覆盖: %+v", vars["host"])
	}
	if vars["onlyWorkspace"].Source != domain.VarWorkspace ||
		vars["onlyFolder"].Source != domain.VarFolder ||
		vars["onlyRequest"].Source != domain.VarRequest {
		t.Errorf("各层变量没合并进来: %+v", vars)
	}
	if vars["token"].Value != "secret-token" || !vars["token"].Secret {
		t.Errorf("secret 变量应从 secrets 取值: %+v", vars["token"])
	}
	if vars["onlyEnv"].Value != "current" {
		t.Errorf("环境变量的 current 值应覆盖 initial: %+v", vars["onlyEnv"])
	}

	got, _, missing, err := s.PrepareRequest(ws.ID, coll.ID, req.ID)
	if err != nil {
		t.Fatalf("PrepareRequest: %v", err)
	}
	if len(missing) != 0 {
		t.Fatalf("不该有缺失变量: %v", missing)
	}
	if want := "https://collection.example.com/global/workspace/folder/request"; got.URL != want {
		t.Errorf("URL = %q，想要 %q", got.URL, want)
	}
	if got.Headers[0].Value != "Bearer secret-token" {
		t.Errorf("请求头变量没替换: %q", got.Headers[0].Value)
	}

	// secret 不应该出现在环境文件里。
	envPath := filepath.Join(dir, "workspaces", "default", "environments", "dev.environment.json")
	data, err := os.ReadFile(envPath)
	if err != nil {
		t.Fatalf("环境文件不存在: %v", err)
	}
	if strings.Contains(string(data), "secret-token") {
		t.Errorf("secret 值不应写进环境文件: %s", data)
	}
	secretData, err := os.ReadFile(filepath.Join(dir, "workspaces", "default", "secrets.json"))
	if err != nil {
		t.Fatalf("secrets.json 不存在: %v", err)
	}
	if !strings.Contains(string(secretData), "secret-token") {
		t.Errorf("secret 值应写进 secrets.json: %s", secretData)
	}

	// 重启后环境选中态与 secret 都应恢复。
	s2 := reopen(t, dir)
	got2, _, missing2, err := s2.PrepareRequest(ws.ID, coll.ID, req.ID)
	if err != nil {
		t.Fatalf("重启后 PrepareRequest: %v", err)
	}
	if len(missing2) != 0 || got2.Headers[0].Value != "Bearer secret-token" {
		t.Errorf("重启后 secret 没恢复: %q %v", got2.Headers[0].Value, missing2)
	}
	if ws2 := defaultWS(t, s2); ws2.SelectedEnvironmentID != env.ID {
		t.Errorf("选中环境没恢复: %q", ws2.SelectedEnvironmentID)
	}
	if got := s2.Variables(ws.ID, coll.ID, req.ID); got["onlyEnv"].Value != "current" {
		t.Errorf("current 值没恢复: %+v", got["onlyEnv"])
	}
}

func TestAuthInheritance(t *testing.T) {
	s, _ := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.UpdateCollection(coll.ID, func(c *domain.Collection) {
		c.Auth = domain.Auth{Type: domain.AuthBearer,
			Params: []domain.KV{{ID: "a1", Key: "token", Value: "abc", Enabled: true}}}
	}); err != nil {
		t.Fatal(err)
	}
	folder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Users"}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "获取用户",
		Auth:    domain.Auth{Type: domain.AuthInherit},
		Request: &domain.Request{Method: "GET", URL: "https://example.com"}}
	if err := s.AddNode(coll.ID, folder.ID, req); err != nil {
		t.Fatal(err)
	}
	_, auth, _, err := s.PrepareRequest(ws.ID, coll.ID, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if auth.Type != domain.AuthBearer || auth.Params[0].Value != "abc" {
		t.Errorf("认证应继承集合层: %+v", auth)
	}
}

func TestCustomWorkspaceDirectory(t *testing.T) {
	root := t.TempDir()
	s, err := OpenAt(root)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()

	dir := t.TempDir()
	ws, err := s.CreateWorkspaceAt("团队 A", dir)
	if err != nil {
		t.Fatalf("CreateWorkspaceAt: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "workspace.json")); err != nil {
		t.Fatalf("自选目录里应该有 workspace.json: %v", err)
	}
	coll, err := s.CreateCollection(ws.ID, "集合 A", "")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "collections", "集合 A", "collection.json")); err != nil {
		t.Fatalf("集合应建在自选目录里: %v", err)
	}

	// 重新打开数据根目录，自选工作区应该还在。
	s2 := reopen(t, root)
	found := false
	for _, w := range s2.Snapshot().Workspaces {
		if w.ID == ws.ID {
			found = true
			if len(w.Collections) != 1 || w.Collections[0].ID != coll.ID {
				t.Errorf("自选工作区的集合没加载: %+v", w.Collections)
			}
		}
	}
	if !found {
		t.Fatal("自选工作区没被注册")
	}

	// 移除工作区只取消注册，不删用户目录。
	if err := s2.RemoveWorkspace(ws.ID); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "workspace.json")); err != nil {
		t.Errorf("移除工作区不应删除用户目录: %v", err)
	}
}

func TestTakeOverCopiedWorkspace(t *testing.T) {
	src, _ := newStoreAt(t)
	dir := t.TempDir()
	ws, err := src.CreateWorkspaceAt("团队 A", dir)
	if err != nil {
		t.Fatal(err)
	}
	coll, err := src.CreateCollection(ws.ID, "集合 A", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := src.Close(); err != nil {
		t.Fatal(err)
	}

	// 换一个数据根目录，把已有的工作区目录接管进来，模拟整目录拷贝。
	s := reopen(t, t.TempDir())
	got, err := s.CreateWorkspaceAt("名字会被忽略", dir)
	if err != nil {
		t.Fatalf("接管工作区失败: %v", err)
	}
	if got.ID != ws.ID || got.Name != "团队 A" {
		t.Errorf("接管后的工作区 = %+v，想要 %q", got, ws.ID)
	}
	for _, w := range s.Snapshot().Workspaces {
		if w.ID == ws.ID {
			if len(w.Collections) != 1 || w.Collections[0].ID != coll.ID {
				t.Errorf("接管后集合没加载: %+v", w.Collections)
			}
			return
		}
	}
	t.Fatal("接管后快照里找不到工作区")
}

func TestRemovedWorkspaceStaysRemoved(t *testing.T) {
	s, dir := newStoreAt(t)
	ws, err := s.CreateWorkspaceAt("临时", "")
	if err != nil {
		t.Fatal(err)
	}
	if err := s.RemoveWorkspace(ws.ID); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}

	s2 := reopen(t, dir)
	for _, w := range s2.Snapshot().Workspaces {
		if w.ID == ws.ID {
			t.Fatalf("被移除的工作区不应在重启后自动恢复")
		}
	}
}

func TestMigrationFromV2(t *testing.T) {
	root := t.TempDir()
	oldDir := t.TempDir()
	oldPath := filepath.Join(oldDir, "旧集合.collection.json")
	oldColl := map[string]any{
		"version": 2,
		"id":      "c1",
		"name":    "旧集合",
		"items": []any{
			map[string]any{
				"id": "f1", "kind": "folder", "name": "Users",
				"children": []any{
					map[string]any{
						"id": "r1", "kind": "request", "name": "获取用户",
						"request": map[string]any{
							"method": "GET", "url": "{{baseUrl}}/users",
							"params":  []any{map[string]any{"key": "page", "value": "1", "enabled": true}},
							"headers": []any{map[string]any{"key": "X-Tenant", "value": "acme", "enabled": true}},
							"body":    `{"a":1}`,
							"auth":    "bearer",
						},
					},
				},
			},
		},
	}
	data, err := json.Marshal(oldColl)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(oldPath, data, 0o644); err != nil {
		t.Fatal(err)
	}
	oldReg := map[string]any{
		"version": 2,
		"workspaces": []any{
			map[string]any{"id": "w1", "name": "默认工作区", "default": true},
		},
		"collections": []any{
			map[string]any{"id": "c1", "name": "旧集合", "path": oldPath, "workspaceID": "w1"},
		},
	}
	regData, err := json.Marshal(oldReg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "workspaces.json"), regData, 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := OpenAt(root)
	if err != nil {
		t.Fatalf("迁移后打开失败: %v", err)
	}
	defer s.Close()

	ws := defaultWS(t, s)
	if len(ws.Collections) != 1 || ws.Collections[0].ID != "c1" {
		t.Fatalf("旧集合没迁移过来: %+v", ws.Collections)
	}
	req := domain.FindIn(ws.Collections[0].Items, "r1")
	if req == nil || req.Request == nil {
		t.Fatalf("请求没迁移: %+v", req)
	}
	if req.Request.URL != "{{baseUrl}}/users" || len(req.Request.Query) != 1 ||
		req.Request.Query[0].Key != "page" {
		t.Errorf("请求参数转换不对: %+v", req.Request)
	}
	if req.Request.Body.Mode != domain.BodyRaw || req.Request.Body.Raw != `{"a":1}` {
		t.Errorf("body 转换不对: %+v", req.Request.Body)
	}
	if req.Auth.Type != domain.AuthBearer {
		t.Errorf("auth 转换不对: %+v", req.Auth)
	}
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("迁移成功后旧文件应删除: %v", err)
	}
	newPath := filepath.Join(root, "workspaces", "default", "collections", "旧集合", "Users", "获取用户.request.json")
	if _, err := os.Stat(newPath); err != nil {
		t.Errorf("新请求文件不存在: %v", err)
	}
}

func TestNewerVersionIsReadOnly(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "请求",
		Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(coll.ID, "", req); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(collectionDir(dir, "集合"), "请求.request.json")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	data = []byte(strings.Replace(string(data), `"version": 3`, `"version": 99`, 1))
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}

	s2 := reopen(t, dir)
	if !defaultWS(t, s2).ReadOnly {
		t.Fatal("存在高版本文件时工作区应为只读")
	}
	if err := s2.AddNode(coll.ID, "", &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "x"}); !errors.Is(err, ErrReadOnly) {
		t.Errorf("只读工作区写入应返回 ErrReadOnly，得到 %v", err)
	}
}

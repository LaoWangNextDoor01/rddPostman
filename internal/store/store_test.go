package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

func TestHistoryRecordAndLoad(t *testing.T) {
	s, _ := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "集合", "")
	if err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "请求",
		Request: &domain.Request{Method: "GET", URL: "https://example.com"}}
	if err := s.AddNode(coll.ID, "", req); err != nil {
		t.Fatal(err)
	}

	// 写入两条历史记录。
	for i, status := range []int{200, 404} {
		entry := domain.HistoryEntry{
			CollectionID: coll.ID,
			RequestID:    req.ID,
			Method:       "GET",
			URL:          "https://example.com",
			Status:       status,
			DurationMS:   100 + i*50,
			At:           time.Now(),
		}
		if err := s.RecordHistory(ws.ID, entry); err != nil {
			t.Fatalf("RecordHistory: %v", err)
		}
	}

	// 读取历史。
	history, err := s.History(ws.ID, 10)
	if err != nil {
		t.Fatalf("History: %v", err)
	}
	if len(history) != 2 {
		t.Fatalf("历史记录数量 = %d，想要 2", len(history))
	}
	// 倒序，最新的在前。
	if history[0].Status != 404 || history[1].Status != 200 {
		t.Errorf("历史记录顺序不对: %d %d", history[0].Status, history[1].Status)
	}
	if history[0].WorkspaceID != ws.ID {
		t.Errorf("WorkspaceID 没填充: %q", history[0].WorkspaceID)
	}
	if history[0].ID == "" {
		t.Errorf("ID 没自动生成")
	}
}

func TestEnvironmentIndependentAndClone(t *testing.T) {
	s, _ := newStoreAt(t)
	ws := defaultWS(t, s)

	// 1. 验证工作区自动创建了默认环境
	if len(ws.Environments) != 1 {
		t.Fatalf("初始工作区应有 1 个默认环境，得到 %d", len(ws.Environments))
	}
	defEnv := ws.Environments[0]
	if defEnv.Name != domain.DefaultEnvironmentName || defEnv.ID != ws.DefaultEnvironmentID {
		t.Fatalf("默认环境不符合预期: %+v (DefaultID=%q)", defEnv, ws.DefaultEnvironmentID)
	}

	// 2. 尝试删除默认环境应报错
	if err := s.DeleteEnvironment(ws.ID, defEnv.ID); !errors.Is(err, ErrDefaultEnvironment) {
		t.Fatalf("删除默认环境应返回 ErrDefaultEnvironment，得到 %v", err)
	}

	// 3. 在默认环境（作为开发环境）中添加 baseUrl 和 dev 独有的 debugPort
	if err := s.AddEnvironmentVariable(ws.ID, defEnv.ID, domain.KV{
		Key:     "baseUrl",
		Value:   "http://localhost:3000",
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddEnvironmentVariable baseUrl: %v", err)
	}
	if err := s.AddEnvironmentVariable(ws.ID, defEnv.ID, domain.KV{
		Key:     "debugPort",
		Value:   "9229",
		Enabled: true,
	}); err != nil {
		t.Fatalf("AddEnvironmentVariable debugPort: %v", err)
	}

	// 4. 基于默认环境克隆出「测试环境」
	testEnv, err := s.CloneEnvironment(ws.ID, defEnv.ID, "测试环境")
	if err != nil {
		t.Fatalf("CloneEnvironment: %v", err)
	}
	if len(testEnv.Vars) != 2 || testEnv.Vars[0].Key != "baseUrl" {
		t.Fatalf("克隆环境应该包含相同变量结构: %+v", testEnv.Vars)
	}

	// 5. 修改测试环境的 baseUrl，并删除 debugPort（测试环境不需要调试端口）
	testEnv.Vars[0].Value = "https://test.demo.com"
	testEnv.Vars = testEnv.Vars[:1] // 移除 debugPort
	if err := s.SaveEnvironment(ws.ID, testEnv); err != nil {
		t.Fatalf("SaveEnvironment testEnv: %v", err)
	}

	// 6. 验证两个环境互相独立，互不污染
	devUpdated, err := s.DefaultEnvironment(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(devUpdated.Vars) != 2 {
		t.Fatalf("开发环境依然应保留 2 个变量，得到 %d", len(devUpdated.Vars))
	}
	if devUpdated.Vars[0].Value != "http://localhost:3000" {
		t.Errorf("开发环境的 baseUrl 应该保持 localhost:3000，得到 %q", devUpdated.Vars[0].Value)
	}

	// 7. 测试在请求中切换环境：baseUrl 动态无缝替换
	coll, err := s.CreateCollection(ws.ID, "用户服务", "")
	if err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{
		ID:   NewID(),
		Kind: domain.KindRequest,
		Name: "获取用户",
		Request: &domain.Request{
			Method: "GET",
			URL:    "{{baseUrl}}/api/users",
		},
	}
	if err := s.AddNode(coll.ID, "", req); err != nil {
		t.Fatal(err)
	}

	// 当前为默认环境（dev）
	preparedDev, _, _, err := s.PrepareRequest(ws.ID, coll.ID, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := "http://localhost:3000/api/users"; preparedDev.URL != want {
		t.Errorf("开发环境 URL = %q，想要 %q", preparedDev.URL, want)
	}

	// 切换到测试环境（test）
	if err := s.SelectEnvironment(ws.ID, testEnv.ID); err != nil {
		t.Fatal(err)
	}
	preparedTest, _, _, err := s.PrepareRequest(ws.ID, coll.ID, req.ID)
	if err != nil {
		t.Fatal(err)
	}
	if want := "https://test.demo.com/api/users"; preparedTest.URL != want {
		t.Errorf("测试环境 URL = %q，想要 %q", preparedTest.URL, want)
	}
}

func TestCollectionLevelEnvironmentSwitch(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	// 1. 设置默认开发环境
	defEnv, err := s.DefaultEnvironment(ws.ID)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.AddEnvironmentVariable(ws.ID, defEnv.ID, domain.KV{
		Key:     "baseUrl",
		Value:   "http://localhost:3000",
		Enabled: true,
	}); err != nil {
		t.Fatal(err)
	}

	// 2. 克隆出「测试环境」和「生产环境」
	testEnv, err := s.CloneEnvironment(ws.ID, defEnv.ID, "测试环境")
	if err != nil {
		t.Fatal(err)
	}
	testEnv.Vars[0].Value = "https://test.api.com"
	if err := s.SaveEnvironment(ws.ID, testEnv); err != nil {
		t.Fatal(err)
	}

	prodEnv, err := s.CloneEnvironment(ws.ID, defEnv.ID, "生产环境")
	if err != nil {
		t.Fatal(err)
	}
	prodEnv.Vars[0].Value = "https://prod.api.com"
	if err := s.SaveEnvironment(ws.ID, prodEnv); err != nil {
		t.Fatal(err)
	}

	// 3. 创建集合 A (用户服务) 和 集合 B (支付服务)
	collA, err := s.CreateCollection(ws.ID, "用户服务", "")
	if err != nil {
		t.Fatal(err)
	}
	reqA := &domain.Node{
		ID:      NewID(),
		Kind:    domain.KindRequest,
		Name:    "获取用户",
		Request: &domain.Request{Method: "GET", URL: "{{baseUrl}}/users"},
	}
	if err := s.AddNode(collA.ID, "", reqA); err != nil {
		t.Fatal(err)
	}

	collB, err := s.CreateCollection(ws.ID, "支付服务", "")
	if err != nil {
		t.Fatal(err)
	}
	reqB := &domain.Node{
		ID:      NewID(),
		Kind:    domain.KindRequest,
		Name:    "发起支付",
		Request: &domain.Request{Method: "POST", URL: "{{baseUrl}}/pay"},
	}
	if err := s.AddNode(collB.ID, "", reqB); err != nil {
		t.Fatal(err)
	}

	// 4. 初始状态：两个集合都继承工作区的环境 (默认 localhost:3000)
	envA, inheritedA := s.CollectionEnvironment(ws.ID, collA.ID)
	if !inheritedA || envA != defEnv.ID {
		t.Errorf("集合 A 初始应继承默认环境: %v %v", envA, inheritedA)
	}
	envB, inheritedB := s.CollectionEnvironment(ws.ID, collB.ID)
	if !inheritedB || envB != defEnv.ID {
		t.Errorf("集合 B 初始应继承默认环境: %v %v", envB, inheritedB)
	}

	resA1, _, _, err := s.PrepareRequest(ws.ID, collA.ID, reqA.ID)
	if err != nil || resA1.URL != "http://localhost:3000/users" {
		t.Fatalf("请求 A 初始 URL = %q，想要 localhost:3000", resA1.URL)
	}
	resB1, _, _, err := s.PrepareRequest(ws.ID, collB.ID, reqB.ID)
	if err != nil || resB1.URL != "http://localhost:3000/pay" {
		t.Fatalf("请求 B 初始 URL = %q，想要 localhost:3000", resB1.URL)
	}

	// 5. 将集合 B 单独切换到「测试环境」，集合 A 保持继承
	if err := s.SelectCollectionEnvironment(ws.ID, collB.ID, testEnv.ID); err != nil {
		t.Fatalf("SelectCollectionEnvironment: %v", err)
	}

	envB2, inheritedB2 := s.CollectionEnvironment(ws.ID, collB.ID)
	if inheritedB2 || envB2 != testEnv.ID {
		t.Fatalf("集合 B 应已覆盖为测试环境且 inherited=false: %v %v", envB2, inheritedB2)
	}

	// 此时：集合 A 依然在 localhost:3000，集合 B 独立切换到了 test.api.com！
	resA2, _, _, _ := s.PrepareRequest(ws.ID, collA.ID, reqA.ID)
	resB2, _, _, _ := s.PrepareRequest(ws.ID, collB.ID, reqB.ID)
	if resA2.URL != "http://localhost:3000/users" {
		t.Errorf("集合 A 受到集合 B 切换的影响: %q", resA2.URL)
	}
	if resB2.URL != "https://test.api.com/pay" {
		t.Errorf("集合 B 独立切换测试环境失败: %q", resB2.URL)
	}

	// 6. 将工作区全局环境切换到「生产环境」
	if err := s.SelectEnvironment(ws.ID, prodEnv.ID); err != nil {
		t.Fatal(err)
	}

	// 集合 A 跟随工作区自动变成 prod.api.com，而集合 B 依然固定在 test.api.com，不受全局干扰！
	resA3, _, _, _ := s.PrepareRequest(ws.ID, collA.ID, reqA.ID)
	resB3, _, _, _ := s.PrepareRequest(ws.ID, collB.ID, reqB.ID)
	if resA3.URL != "https://prod.api.com/users" {
		t.Errorf("集合 A 应自动跟随工作区切换到生产环境: %q", resA3.URL)
	}
	if resB3.URL != "https://test.api.com/pay" {
		t.Errorf("集合 B 应依然保持测试环境覆盖: %q", resB3.URL)
	}

	// 7. 将集合 B 解除覆盖（恢复继承）
	if err := s.SelectCollectionEnvironment(ws.ID, collB.ID, ""); err != nil {
		t.Fatal(err)
	}
	resB4, _, _, _ := s.PrepareRequest(ws.ID, collB.ID, reqB.ID)
	if resB4.URL != "https://prod.api.com/pay" {
		t.Errorf("集合 B 恢复继承后应与全局生产环境一致: %q", resB4.URL)
	}

	// 8. 验证持久化：再次设置集合 B 为测试环境，重新打开 store 验证 local.json 正确恢复
	if err := s.SelectCollectionEnvironment(ws.ID, collB.ID, testEnv.ID); err != nil {
		t.Fatal(err)
	}
	s2 := reopen(t, dir)
	ws2 := defaultWS(t, s2)
	if ws2.CollectionEnvironments[collB.ID] != testEnv.ID {
		t.Errorf("重启后集合 B 独立环境覆盖未恢复: %v", ws2.CollectionEnvironments)
	}
	resB5, _, _, _ := s2.PrepareRequest(ws.ID, collB.ID, reqB.ID)
	if resB5.URL != "https://test.api.com/pay" {
		t.Errorf("重启后集合 B 请求解析 URL = %q，想要 test.api.com", resB5.URL)
	}
}

func TestActiveWorkspace(t *testing.T) {
	s, _ := newStoreAt(t)

	// 1. 初始活跃工作区应为默认工作区
	defWS := defaultWS(t, s)
	if s.ActiveWorkspaceID() != defWS.ID {
		t.Fatalf("初始活跃工作区应为 %s，得到 %s", defWS.ID, s.ActiveWorkspaceID())
	}
	if active := s.ActiveWorkspace(); active == nil || active.ID != defWS.ID {
		t.Fatalf("Snapshot.ActiveWorkspace() 应返回默认工作区，得到 %+v", active)
	}

	// 2. 创建新工作区
	teamWS, err := s.CreateWorkspace("团队空间")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}

	// 3. 切换活跃工作区到新工作区
	if err := s.SetActiveWorkspace(teamWS.ID); err != nil {
		t.Fatalf("SetActiveWorkspace: %v", err)
	}
	if s.ActiveWorkspaceID() != teamWS.ID {
		t.Fatalf("切换后 ActiveWorkspaceID 应为 %s，得到 %s", teamWS.ID, s.ActiveWorkspaceID())
	}
	if active := s.Snapshot().ActiveWorkspace(); active == nil || active.Name != "团队空间" {
		t.Fatalf("Snapshot.ActiveWorkspace() 应返回团队空间，得到 %+v", active)
	}

	// 4. 切换到不存在的工作区应报错
	if err := s.SetActiveWorkspace("non-existent"); err == nil {
		t.Fatalf("切换到不存在的工作区应该返回错误")
	}

	// 5. 移除当前活跃的工作区，应该自动回退到默认工作区
	if err := s.RemoveWorkspace(teamWS.ID); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
	if s.ActiveWorkspaceID() != defWS.ID {
		t.Fatalf("移除当前工作区后应自动回退到默认工作区 %s，得到 %s", defWS.ID, s.ActiveWorkspaceID())
	}
	if active := s.ActiveWorkspace(); active == nil || active.ID != defWS.ID {
		t.Fatalf("回退后 ActiveWorkspace 应返回默认工作区，得到 %+v", active)
	}
}

func TestCollectionMarkdownDescriptionUpdateAndReload(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	mdDesc := "# API 文档\n\n本集合包含主要接口。\n\n- 用户接口\n- 支付接口\n\n```json\n{\"code\": 0}\n```"
	coll, err := s.CreateCollection(ws.ID, "核心服务", mdDesc)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if coll.Description != mdDesc {
		t.Fatalf("创建后 Description 不匹配: got %q, want %q", coll.Description, mdDesc)
	}

	// 更新描述
	nextDesc := "## 更新后的文档\n\n包含 **重要变更**。"
	err = s.UpdateCollection(coll.ID, func(c *domain.Collection) {
		c.Description = nextDesc
	})
	if err != nil {
		t.Fatalf("UpdateCollection: %v", err)
	}

	// 验证磁盘上生成了独立的 README.md 文件
	collReadme := filepath.Join(dir, "workspaces", "default", "collections", "核心服务", "README.md")
	readmeBytes, err := os.ReadFile(collReadme)
	if err != nil {
		t.Fatalf("集合目录下应有 README.md 文件: %v", err)
	}
	if string(readmeBytes) != nextDesc {
		t.Fatalf("README.md 内容不匹配: got %q, want %q", string(readmeBytes), nextDesc)
	}

	// 验证 collection.json 中去除了冗余的大文本
	collJSONPath := filepath.Join(dir, "workspaces", "default", "collections", "核心服务", "collection.json")
	jsonBytes, err := os.ReadFile(collJSONPath)
	if err != nil {
		t.Fatalf("读取 collection.json 失败: %v", err)
	}
	var rawColl domain.Collection
	if err := json.Unmarshal(jsonBytes, &rawColl); err != nil {
		t.Fatalf("反序列化 collection.json 失败: %v", err)
	}
	if rawColl.Description != "" {
		t.Fatalf("collection.json 中不应冗余存储 Description，得到 %q", rawColl.Description)
	}

	// 测试目录节点的 README.md 存储
	folder := &domain.Node{
		ID:          NewID(),
		Kind:        domain.KindFolder,
		Name:        "用户中心",
		Description: "# 用户中心模块\n\n- 登录\n- 注册",
	}
	if err := s.AddNode(coll.ID, "", folder); err != nil {
		t.Fatalf("AddNode folder: %v", err)
	}
	folderReadme := filepath.Join(dir, "workspaces", "default", "collections", "核心服务", "用户中心", "README.md")
	folderReadmeBytes, err := os.ReadFile(folderReadme)
	if err != nil {
		t.Fatalf("目录层级下应有独立 README.md: %v", err)
	}
	if string(folderReadmeBytes) != folder.Description {
		t.Fatalf("目录 README.md 内容不匹配: got %q, want %q", string(folderReadmeBytes), folder.Description)
	}

	// 重新加载并验证落盘
	s2 := reopen(t, dir)
	ws2 := defaultWS(t, s2)
	if len(ws2.Collections) == 0 {
		t.Fatal("未加载到集合")
	}
	if ws2.Collections[0].Description != nextDesc {
		t.Fatalf("重新打开后 Collection.Description 不匹配: got %q, want %q", ws2.Collections[0].Description, nextDesc)
	}
	if len(ws2.Collections[0].Items) == 0 || ws2.Collections[0].Items[0].Description != folder.Description {
		t.Fatalf("重新打开后 Folder.Description 不匹配: got %q, want %q", ws2.Collections[0].Items[0].Description, folder.Description)
	}
}

func TestWorkspaceIconCopyAndReplace(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	// 准备外部源 PNG 与 SVG 文件
	srcDir := t.TempDir()
	pngSrc := filepath.Join(srcDir, "logo.png")
	svgSrc := filepath.Join(srcDir, "logo.svg")
	if err := os.WriteFile(pngSrc, []byte("fake-png-bytes"), 0o644); err != nil {
		t.Fatalf("write pngSrc: %v", err)
	}
	if err := os.WriteFile(svgSrc, []byte("<svg/>"), 0o644); err != nil {
		t.Fatalf("write svgSrc: %v", err)
	}

	// 1. 设置 PNG 图标：应复制到工作区目录 .rddpostman.icon.png，workspace.json 只存相对文件名
	if err := s.SetWorkspaceIcon(ws.ID, pngSrc); err != nil {
		t.Fatalf("SetWorkspaceIcon png: %v", err)
	}
	_ = os.Remove(pngSrc) // 模拟源文件被用户删除

	wsDir := filepath.Join(dir, "workspaces", "default")
	copiedPNG := filepath.Join(wsDir, ".rddpostman.icon.png")
	gotBytes, err := os.ReadFile(copiedPNG)
	if err != nil || string(gotBytes) != "fake-png-bytes" {
		t.Fatalf("copied png mismatch: err=%v, data=%q", err, string(gotBytes))
	}

	raw, err := os.ReadFile(workspaceFile(dir))
	if err != nil {
		t.Fatalf("read workspace.json: %v", err)
	}
	var doc domain.Workspace
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatalf("unmarshal workspace.json: %v", err)
	}
	if doc.Icon != ".rddpostman.icon.png" {
		t.Fatalf("workspace.json icon = %q, want .rddpostman.icon.png", doc.Icon)
	}
	if snapWS := s.Snapshot().Workspace(ws.ID); snapWS == nil || snapWS.Icon != copiedPNG {
		t.Fatalf("snapshot icon = %q, want %q", snapWS.Icon, copiedPNG)
	}

	// 2. 替换为 SVG 图标：旧的 .rddpostman.icon.png 应被清理，且重开 Store 后仍正确解析
	if err := s.SetWorkspaceIcon(ws.ID, svgSrc); err != nil {
		t.Fatalf("SetWorkspaceIcon svg: %v", err)
	}
	if _, err := os.Stat(copiedPNG); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("stale png icon should be removed, stat err=%v", err)
	}
	copiedSVG := filepath.Join(wsDir, ".rddpostman.icon.svg")
	s2 := reopen(t, dir)
	if got := s2.Snapshot().Workspace(ws.ID); got == nil || got.Icon != copiedSVG {
		t.Fatalf("reopened icon = %v, want %q", got, copiedSVG)
	}
}

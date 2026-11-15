package store

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"rddpostman/internal/domain"
)

func newStore(t *testing.T) *Store {
	t.Helper()
	s, err := OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("OpenAt: %v", err)
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

func findWorkspace(s *Store, id domain.ID) *Workspace {
	for _, w := range s.Snapshot().Workspaces {
		if w.ID == id {
			return w
		}
	}
	return nil
}

func TestDefaultWorkspace(t *testing.T) {
	s := newStore(t)
	snap := s.Snapshot()
	if len(snap.Workspaces) != 1 {
		t.Fatalf("首次启动应有 1 个工作区，得到 %d", len(snap.Workspaces))
	}
	def := snap.Workspaces[0]
	if !def.Default || def.Name != defaultSpaceName {
		t.Fatalf("默认工作区不对: %+v", def)
	}
	if err := s.RemoveWorkspace(def.ID); !errors.Is(err, ErrDefaultWorkspace) {
		t.Errorf("删除默认工作区应返回 ErrDefaultWorkspace，得到 %v", err)
	}
	// 默认工作区改名后重启仍然是默认。
	if err := s.RenameWorkspace(def.ID, "我的工作区"); err != nil {
		t.Fatalf("RenameWorkspace: %v", err)
	}
	s2, err := OpenAt(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	if got := defaultWS(t, s2); got.Name != "我的工作区" {
		t.Errorf("默认工作区名 = %q", got.Name)
	}
}

func TestCreateCollectionPersists(t *testing.T) {
	s := newStore(t)
	ws := defaultWS(t, s)
	collDir := t.TempDir()

	coll, err := s.CreateCollection(ws.ID, "开发者平台 API", collDir)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	path := filepath.Join(collDir, "开发者平台 API"+collectionExt)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("集合文件没写出来: %v", err)
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
	if err := s.UpdateRequest(coll.ID, req.ID, func(r *domain.Request) {
		r.URL = "{{baseUrl}}/users/{{userId}}"
	}); err != nil {
		t.Fatalf("UpdateRequest: %v", err)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush: %v", err)
	}

	// 换一个 Store 重新加载，验证落盘内容。
	s2, err := OpenAt(s.dir)
	if err != nil {
		t.Fatalf("重新打开: %v", err)
	}
	defer s2.Close()
	ws2 := defaultWS(t, s2)
	if len(ws2.Collections) != 1 {
		t.Fatalf("重新加载后有 %d 个集合，想要 1", len(ws2.Collections))
	}
	got := domain.FindIn(ws2.Collections[0].Items, req.ID)
	if got == nil || got.Request.URL != "{{baseUrl}}/users/{{userId}}" {
		t.Fatalf("请求没持久化正确: %+v", got)
	}
	if domain.CountRequests(ws2.Collections[0].Items) != 1 {
		t.Errorf("请求数量不对")
	}
}

func TestWorkspaceGroupsCollections(t *testing.T) {
	s := newStore(t)
	ws, err := s.CreateWorkspace("团队 A")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	if _, err := s.CreateCollection(ws.ID, "集合 A", t.TempDir()); err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	// 未指定工作区时落到默认工作区。
	if _, err := s.CreateCollection("", "默认集合", t.TempDir()); err != nil {
		t.Fatalf("CreateCollection 默认: %v", err)
	}

	snap := s.Snapshot()
	if len(snap.Workspaces) != 2 {
		t.Fatalf("应该有 2 个工作区，得到 %d", len(snap.Workspaces))
	}
	if got := findWorkspace(s, ws.ID); got == nil || len(got.Collections) != 1 ||
		got.Collections[0].Name != "集合 A" {
		t.Fatalf("团队 A 下的集合不对: %+v", got)
	}
	if def := defaultWS(t, s); len(def.Collections) != 1 || def.Collections[0].Name != "默认集合" {
		t.Fatalf("默认工作区下的集合不对: %+v", def)
	}

	if err := s.RenameWorkspace(ws.ID, "团队 B"); err != nil {
		t.Fatalf("RenameWorkspace: %v", err)
	}
	if got := findWorkspace(s, ws.ID); got.Name != "团队 B" {
		t.Errorf("工作区改名失败: %q", got.Name)
	}

	// 删除自定义工作区，集合移到默认工作区。
	if err := s.RemoveWorkspace(ws.ID); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
	snap = s.Snapshot()
	if len(snap.Workspaces) != 1 {
		t.Fatalf("删除后应只剩默认工作区，得到 %d", len(snap.Workspaces))
	}
	if def := defaultWS(t, s); len(def.Collections) != 2 {
		t.Errorf("默认工作区应该接管 2 个集合，得到 %d", len(def.Collections))
	}
}

func TestCreateCollectionRejectsUnknownWorkspace(t *testing.T) {
	s := newStore(t)
	dir := t.TempDir()
	_, err := s.CreateCollection("nope", "集合", dir)
	if !errors.Is(err, ErrNotFound) {
		t.Fatalf("未知工作区应返回 ErrNotFound，得到 %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "集合"+collectionExt)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("失败时不应留下集合文件: %v", err)
	}
}

func TestRenameCollectionRenamesFile(t *testing.T) {
	s := newStore(t)
	collDir := t.TempDir()
	coll, err := s.CreateCollection("", "旧名字", collDir)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := s.RenameCollection(coll.ID, "新名字"); err != nil {
		t.Fatalf("RenameCollection: %v", err)
	}
	if _, err := os.Stat(filepath.Join(collDir, "旧名字"+collectionExt)); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("旧文件应该已被改名: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(collDir, "新名字"+collectionExt))
	if err != nil {
		t.Fatalf("新文件不存在: %v", err)
	}
	var onDisk domain.Collection
	if err := json.Unmarshal(data, &onDisk); err != nil {
		t.Fatal(err)
	}
	if onDisk.Name != "新名字" {
		t.Errorf("文件内容里的名字 = %q", onDisk.Name)
	}
}

func TestOpenCollectionFromOtherDir(t *testing.T) {
	src := newStore(t)
	other := t.TempDir()
	coll, err := src.CreateCollection("", "外部集合", other)
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}
	if err := src.Flush(); err != nil {
		t.Fatal(err)
	}

	dst := newStore(t)
	opened, err := dst.OpenCollection(filepath.Join(other, "外部集合"+collectionExt))
	if err != nil {
		t.Fatalf("OpenCollection: %v", err)
	}
	if opened.ID != coll.ID {
		t.Errorf("打开后的 ID = %q，想要 %q", opened.ID, coll.ID)
	}
	if got := defaultWS(t, dst); len(got.Collections) != 1 {
		t.Errorf("注册表里应该有 1 个集合")
	}
}

func TestMissingCollectionIsReported(t *testing.T) {
	s := newStore(t)
	collDir := t.TempDir()
	coll, err := s.CreateCollection("", "会消失的集合", collDir)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(collDir, "会消失的集合"+collectionExt)); err != nil {
		t.Fatal(err)
	}

	s2, err := OpenAt(s.dir)
	if err != nil {
		t.Fatal(err)
	}
	defer s2.Close()
	snap := s2.Snapshot()
	if len(defaultWS(t, s2).Collections) != 0 || len(snap.Missing) != 1 || snap.Missing[0] != coll.ID {
		t.Errorf("丢失的集合应进 Missing，得到 missing=%v", snap.Missing)
	}
}

func TestDeleteNodeAndRemoveCollection(t *testing.T) {
	s := newStore(t)
	collDir := t.TempDir()
	coll, err := s.CreateCollection("", "待删除", collDir)
	if err != nil {
		t.Fatal(err)
	}
	n := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "请求",
		Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(coll.ID, "", n); err != nil {
		t.Fatal(err)
	}
	if err := s.DeleteNode(coll.ID, n.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if domain.FindIn(s.Snapshot().Workspaces[0].Collections[0].Items, n.ID) != nil {
		t.Errorf("节点没删掉")
	}
	if err := s.DeleteNode(coll.ID, n.ID); !errors.Is(err, ErrNotFound) {
		t.Errorf("重复删除应返回 ErrNotFound，得到 %v", err)
	}

	path := filepath.Join(collDir, "待删除"+collectionExt)
	if err := s.RemoveCollection(coll.ID, true); err != nil {
		t.Fatalf("RemoveCollection: %v", err)
	}
	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("集合文件应该已删除: %v", err)
	}
	if len(defaultWS(t, s).Collections) != 0 {
		t.Errorf("注册表里应该没有集合了")
	}
}

func TestCreateCollectionRejectsDuplicateName(t *testing.T) {
	s := newStore(t)
	dir := t.TempDir()
	if _, err := s.CreateCollection("", "重复", dir); err != nil {
		t.Fatal(err)
	}
	if _, err := s.CreateCollection("", "重复", dir); !errors.Is(err, ErrExists) {
		t.Errorf("同名集合应返回 ErrExists，得到 %v", err)
	}
}

func TestLegacyRegistryMigrated(t *testing.T) {
	dir := t.TempDir()
	collDir := t.TempDir()
	coll := &domain.Collection{Version: 1, ID: "old1", Name: "旧集合", Items: []*domain.Node{}}
	data, err := json.Marshal(coll)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(collDir, "旧集合"+collectionExt)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	legacy := registry{Version: 1, Collections: []Ref{{ID: "old1", Name: "旧集合", Path: path}}}
	legacyData, err := json.Marshal(legacy)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, legacyRegistryFile), legacyData, 0o644); err != nil {
		t.Fatal(err)
	}

	s, err := OpenAt(dir)
	if err != nil {
		t.Fatalf("打开旧数据: %v", err)
	}
	defer s.Close()
	snap := s.Snapshot()
	if len(snap.Workspaces) != 1 || !snap.Workspaces[0].Default {
		t.Fatalf("迁移后应有默认工作区: %+v", snap.Workspaces)
	}
	if got := snap.Workspaces[0].Collections; len(got) != 1 || got[0].ID != "old1" {
		t.Fatalf("旧集合应挂到默认工作区: %+v", got)
	}
	if _, err := os.Stat(filepath.Join(dir, registryFile)); err != nil {
		t.Errorf("应写出新版注册表: %v", err)
	}
}

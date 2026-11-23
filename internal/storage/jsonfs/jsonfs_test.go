package jsonfs

import (
	"context"
	"testing"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

func TestBackendRoundTrip(t *testing.T) {
	ctx := context.Background()
	b, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	// 1. 首次 List 自动创建默认工作区
	refs, err := b.List(ctx)
	if err != nil || len(refs) != 1 || !refs[0].Default {
		t.Fatalf("List = %+v, err=%v; want 1 default workspace", refs, err)
	}

	// 2. 创建自定义工作区、集合、目录与请求节点
	bundle, err := b.CreateWorkspace(ctx, "QA Team", "")
	if err != nil {
		t.Fatalf("CreateWorkspace: %v", err)
	}
	ref := bundle.Ref

	coll, err := b.CreateCollection(ctx, ref, storage.Input{Name: "Users", Description: "User endpoints"})
	if err != nil {
		t.Fatalf("CreateCollection: %v", err)
	}

	folder := &domain.Node{ID: domain.NewID(), Kind: domain.KindFolder, Name: "Auth"}
	if err := b.SaveNode(ctx, ref, coll.ID, "", folder); err != nil {
		t.Fatalf("SaveNode folder: %v", err)
	}
	reqNode := &domain.Node{
		ID:   domain.NewID(),
		Kind: domain.KindRequest,
		Name: "Login",
		Request: &domain.Request{
			Method: "POST",
			URL:    "https://api.example.com/login",
		},
	}
	if err := b.SaveNode(ctx, ref, coll.ID, folder.ID, reqNode); err != nil {
		t.Fatalf("SaveNode request: %v", err)
	}

	// 3. 环境、Globals 与历史记录持久化
	env := domain.Environment{
		Schema:  domain.SchemaEnvironment,
		Version: domain.Version,
		ID:      domain.NewID(),
		Name:    "Prod",
		Vars:    []domain.KV{{ID: domain.NewID(), Key: "host", Value: "https://prod.example.com", Enabled: true}},
	}
	if err := b.SaveEnvironment(ctx, ref, env); err != nil {
		t.Fatalf("SaveEnvironment: %v", err)
	}

	globals := domain.Globals{
		Schema:    domain.SchemaGlobals,
		Version:   domain.Version,
		Variables: []domain.KV{{ID: domain.NewID(), Key: "app", Value: "rdd", Enabled: true}},
	}
	if err := b.SaveGlobals(ctx, globals); err != nil {
		t.Fatalf("SaveGlobals: %v", err)
	}
	gotGlobals, err := b.LoadGlobals(ctx)
	if err != nil || len(gotGlobals.Variables) != 1 || gotGlobals.Variables[0].Key != "app" {
		t.Fatalf("LoadGlobals = %+v, err=%v", gotGlobals, err)
	}

	entry := domain.HistoryEntry{
		ID:          domain.NewID(),
		WorkspaceID: ref.ID,
		Method:      "POST",
		URL:         "https://api.example.com/login",
		Status:      200,
		DurationMS:  42,
		At:          time.Now(),
	}
	if err := b.AppendHistory(ctx, ref, entry); err != nil {
		t.Fatalf("AppendHistory: %v", err)
	}
	hist, err := b.LoadHistory(ctx, ref, storage.HistoryQuery{Limit: 10})
	if err != nil || len(hist) != 1 || hist[0].URL != entry.URL {
		t.Fatalf("LoadHistory = %+v, err=%v", hist, err)
	}
	if err := b.ClearHistory(ctx, ref); err != nil {
		t.Fatalf("ClearHistory: %v", err)
	}
	histAfterClear, err := b.LoadHistory(ctx, ref, storage.HistoryQuery{Limit: 10})
	if err != nil || len(histAfterClear) != 0 {
		t.Fatalf("LoadHistory after clear = %+v, err=%v", histAfterClear, err)
	}

	// 4. 重新 Load 验证磁盘结构完整恢复
	reloaded, err := b.Load(ctx, ref)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if len(reloaded.Collections) != 1 || reloaded.Collections[0].Name != "Users" {
		t.Fatalf("reloaded collections = %+v", reloaded.Collections)
	}
	if got := domain.FindIn(reloaded.Collections[0].Items, reqNode.ID); got == nil || got.Name != "Login" {
		t.Fatalf("expected Login node in reloaded tree, got %+v", got)
	}
	if len(reloaded.Environments) != 1 || reloaded.Environments[0].Name != "Prod" {
		t.Fatalf("reloaded environments = %+v", reloaded.Environments)
	}

	// 5. 删除节点与取消工作区注册
	if err := b.DeleteNode(ctx, ref, coll.ID, reqNode.ID); err != nil {
		t.Fatalf("DeleteNode: %v", err)
	}
	if err := b.RemoveWorkspace(ctx, ref); err != nil {
		t.Fatalf("RemoveWorkspace: %v", err)
	}
}

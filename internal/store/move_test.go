package store

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"rddpostman/internal/domain"
)

func TestMoveNodeReorderAndPersistence(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	coll, err := s.CreateCollection(ws.ID, "API", "")
	if err != nil {
		t.Fatal(err)
	}
	r1 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "R1", Request: &domain.Request{Method: "GET"}}
	r2 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "R2", Request: &domain.Request{Method: "POST"}}
	r3 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "R3", Request: &domain.Request{Method: "PUT"}}
	r4 := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "R4", Request: &domain.Request{Method: "DELETE"}}
	for _, r := range []*domain.Node{r1, r2, r3, r4} {
		if err := s.AddNode(coll.ID, "", r); err != nil {
			t.Fatal(err)
		}
	}

	// 同级重排：把 R1 (0) 移到 R3 (2) 后面（dstIndex=3） -> [R2, R3, R1, R4]
	if err := s.MoveNode(coll.ID, r1.ID, coll.ID, "", 3); err != nil {
		t.Fatalf("MoveNode R1 after R3: %v", err)
	}
	assertNodeOrder(t, defaultWS(t, s).Collections[0].Items, []domain.ID{r2.ID, r3.ID, r1.ID, r4.ID})

	// 把 R4 (3) 移到最前（dstIndex=0） -> [R4, R2, R3, R1]
	if err := s.MoveNode(coll.ID, r4.ID, coll.ID, "", 0); err != nil {
		t.Fatalf("MoveNode R4 to front: %v", err)
	}
	assertNodeOrder(t, defaultWS(t, s).Collections[0].Items, []domain.ID{r4.ID, r2.ID, r3.ID, r1.ID})

	// 原位无变化（R2 在下标 1，移到 dstIndex=1 或 2 均为空操作）
	if err := s.MoveNode(coll.ID, r2.ID, coll.ID, "", 1); err != nil {
		t.Fatalf("no-op MoveNode: %v", err)
	}
	if err := s.MoveNode(coll.ID, r2.ID, coll.ID, "", 2); err != nil {
		t.Fatalf("no-op MoveNode adjacent: %v", err)
	}
	assertNodeOrder(t, defaultWS(t, s).Collections[0].Items, []domain.ID{r4.ID, r2.ID, r3.ID, r1.ID})

	// 重启重新从磁盘加载，验证排序完全持久化。
	s2 := reopen(t, dir)
	assertNodeOrder(t, defaultWS(t, s2).Collections[0].Items, []domain.ID{r4.ID, r2.ID, r3.ID, r1.ID})
}

func TestMoveNodeRebalanceWhenGapExhausted(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)
	coll, err := s.CreateCollection(ws.ID, "Tight", "")
	if err != nil {
		t.Fatal(err)
	}
	a := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "A", Request: &domain.Request{Method: "GET"}}
	b := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "B", Request: &domain.Request{Method: "GET"}}
	c := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "C", Request: &domain.Request{Method: "GET"}}
	for _, n := range []*domain.Node{a, b, c} {
		if err := s.AddNode(coll.ID, "", n); err != nil {
			t.Fatal(err)
		}
	}
	// 反复把最后一个节点插到 index=1（A 与 B 之间），逼近中点间隔耗尽并触发自动重排步长。
	for i := 0; i < 15; i++ {
		items := defaultWS(t, s).Collections[0].Items
		last := items[len(items)-1]
		if err := s.MoveNode(coll.ID, last.ID, coll.ID, "", 1); err != nil {
			t.Fatalf("iteration %d MoveNode: %v", i, err)
		}
	}
	want := make([]domain.ID, 3)
	for i, n := range defaultWS(t, s).Collections[0].Items {
		want[i] = n.ID
	}
	s2 := reopen(t, dir)
	assertNodeOrder(t, defaultWS(t, s2).Collections[0].Items, want)
}

func TestMoveNodeAcrossFoldersAndCollections(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	collA, err := s.CreateCollection(ws.ID, "CollA", "")
	if err != nil {
		t.Fatal(err)
	}
	collB, err := s.CreateCollection(ws.ID, "CollB", "")
	if err != nil {
		t.Fatal(err)
	}

	folder1 := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Auth", Description: "Auth docs"}
	if err := s.AddNode(collA.ID, "", folder1); err != nil {
		t.Fatal(err)
	}
	subFolder := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "OAuth"}
	if err := s.AddNode(collA.ID, folder1.ID, subFolder); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{
		ID:      NewID(),
		Kind:    domain.KindRequest,
		Name:    "Login",
		Request: &domain.Request{Method: "POST", URL: "https://example.com/login"},
	}
	if err := s.AddNode(collA.ID, subFolder.ID, req); err != nil {
		t.Fatal(err)
	}
	destFolderB := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Target"}
	if err := s.AddNode(collB.ID, "", destFolderB); err != nil {
		t.Fatal(err)
	}

	// 制造一个尚未落盘的防抖请求修改，确保跨集合移动目录时修改不会丢失或复活在旧集合。
	if err := s.UpdateRequest(collA.ID, req.ID, func(r *domain.Request) {
		r.URL = "https://example.com/v2/login"
	}); err != nil {
		t.Fatal(err)
	}

	// 把整个 Auth 目录从 CollA 移入 CollB/Target 下。
	if err := s.MoveNode(collA.ID, folder1.ID, collB.ID, destFolderB.ID, -1); err != nil {
		t.Fatalf("MoveNode across collections: %v", err)
	}
	if err := s.Flush(); err != nil {
		t.Fatalf("Flush after move: %v", err)
	}

	// 验证磁盘位置已迁移
	oldPath := filepath.Join(collectionDir(dir, "CollA"), "Auth")
	if _, err := os.Stat(oldPath); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("old folder path should not exist: %v", err)
	}
	newReqPath := filepath.Join(collectionDir(dir, "CollB"), "Target", "Auth", "OAuth", "Login.request.json")
	if _, err := os.Stat(newReqPath); err != nil {
		t.Fatalf("moved request file missing on disk: %v", err)
	}

	// 重启后校验完整子树与最新 URL
	s2 := reopen(t, dir)
	ws2 := defaultWS(t, s2)
	var gotA, gotB *domain.Collection
	for _, c := range ws2.Collections {
		if c.ID == collA.ID {
			gotA = c
		}
		if c.ID == collB.ID {
			gotB = c
		}
	}
	if len(gotA.Items) != 0 {
		t.Fatalf("CollA should be empty after move, got %d items", len(gotA.Items))
	}
	gotReq := domain.FindIn(gotB.Items, req.ID)
	if gotReq == nil || gotReq.Request == nil || gotReq.Request.URL != "https://example.com/v2/login" {
		t.Fatalf("moved request not preserved after reopen: %+v", gotReq)
	}
	gotFolder := domain.FindIn(gotB.Items, folder1.ID)
	if gotFolder == nil || gotFolder.Description != "Auth docs" {
		t.Fatalf("moved folder description lost: %+v", gotFolder)
	}

	// 把请求移到 CollB 根层
	if err := s2.MoveNode(collB.ID, req.ID, collB.ID, "", 0); err != nil {
		t.Fatalf("MoveNode request to root: %v", err)
	}
	s3 := reopen(t, dir)
	ws3 := defaultWS(t, s3)
	var gotB3 *domain.Collection
	for _, c := range ws3.Collections {
		if c.ID == collB.ID {
			gotB3 = c
		}
	}
	if len(gotB3.Items) != 2 || gotB3.Items[0].ID != req.ID {
		t.Fatalf("expected Login at index 0 of CollB root, got %+v", gotB3.Items)
	}
}

func TestMoveNodeValidationAndErrors(t *testing.T) {
	s, _ := newStoreAt(t)
	ws := defaultWS(t, s)
	otherWS, err := s.CreateWorkspaceAt("Other WS", "")
	if err != nil {
		t.Fatal(err)
	}

	coll, err := s.CreateCollection(ws.ID, "Main", "")
	if err != nil {
		t.Fatal(err)
	}
	otherColl, err := s.CreateCollection(otherWS.ID, "Other", "")
	if err != nil {
		t.Fatal(err)
	}

	parent := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Parent"}
	if err := s.AddNode(coll.ID, "", parent); err != nil {
		t.Fatal(err)
	}
	child := &domain.Node{ID: NewID(), Kind: domain.KindFolder, Name: "Child"}
	if err := s.AddNode(coll.ID, parent.ID, child); err != nil {
		t.Fatal(err)
	}
	req := &domain.Node{ID: NewID(), Kind: domain.KindRequest, Name: "Req", Request: &domain.Request{Method: "GET"}}
	if err := s.AddNode(coll.ID, child.ID, req); err != nil {
		t.Fatal(err)
	}

	// 不能移入自身
	if err := s.MoveNode(coll.ID, parent.ID, coll.ID, parent.ID, -1); !errors.Is(err, ErrInvalidMove) {
		t.Errorf("move folder into self: got %v, want ErrInvalidMove", err)
	}
	// 不能移入子孙目录
	if err := s.MoveNode(coll.ID, parent.ID, coll.ID, child.ID, -1); !errors.Is(err, ErrInvalidMove) {
		t.Errorf("move folder into descendant: got %v, want ErrInvalidMove", err)
	}
	// 不能移入请求节点
	if err := s.MoveNode(coll.ID, parent.ID, coll.ID, req.ID, -1); !errors.Is(err, ErrInvalidMove) {
		t.Errorf("move into request: got %v, want ErrInvalidMove", err)
	}
	// 不能跨工作区移动
	if err := s.MoveNode(coll.ID, req.ID, otherColl.ID, "", -1); !errors.Is(err, ErrInvalidMove) {
		t.Errorf("move across workspaces: got %v, want ErrInvalidMove", err)
	}
}

func TestMoveCollectionReorder(t *testing.T) {
	s, dir := newStoreAt(t)
	ws := defaultWS(t, s)

	c1, err := s.CreateCollection(ws.ID, "Alpha", "")
	if err != nil {
		t.Fatal(err)
	}
	c2, err := s.CreateCollection(ws.ID, "Beta", "")
	if err != nil {
		t.Fatal(err)
	}
	c3, err := s.CreateCollection(ws.ID, "Gamma", "")
	if err != nil {
		t.Fatal(err)
	}

	// 把 Gamma 移到最前 -> [Gamma, Alpha, Beta]
	if err := s.MoveCollection(c3.ID, 0); err != nil {
		t.Fatalf("MoveCollection c3 to 0: %v", err)
	}
	assertCollectionOrder(t, defaultWS(t, s).Collections, []domain.ID{c3.ID, c1.ID, c2.ID})

	// 把 Gamma 移到 Beta 后面 -> [Alpha, Beta, Gamma]
	if err := s.MoveCollection(c3.ID, 3); err != nil {
		t.Fatalf("MoveCollection c3 to end: %v", err)
	}
	assertCollectionOrder(t, defaultWS(t, s).Collections, []domain.ID{c1.ID, c2.ID, c3.ID})

	// 把 Alpha 移到 Beta 后面 -> [Beta, Alpha, Gamma]
	if err := s.MoveCollection(c1.ID, 2); err != nil {
		t.Fatalf("MoveCollection c1 after c2: %v", err)
	}
	assertCollectionOrder(t, defaultWS(t, s).Collections, []domain.ID{c2.ID, c1.ID, c3.ID})

	// 重启后顺序依然保持
	s2 := reopen(t, dir)
	assertCollectionOrder(t, defaultWS(t, s2).Collections, []domain.ID{c2.ID, c1.ID, c3.ID})
}

func assertNodeOrder(t *testing.T, items []*domain.Node, want []domain.ID) {
	t.Helper()
	if len(items) != len(want) {
		t.Fatalf("len(items) = %d, want %d", len(items), len(want))
	}
	for i, id := range want {
		if items[i].ID != id {
			t.Fatalf("items[%d].ID = %q, want %q", i, items[i].ID, id)
		}
	}
}

func assertCollectionOrder(t *testing.T, colls []*domain.Collection, want []domain.ID) {
	t.Helper()
	if len(colls) != len(want) {
		t.Fatalf("len(collections) = %d, want %d", len(colls), len(want))
	}
	for i, id := range want {
		if colls[i].ID != id {
			t.Fatalf("collections[%d].ID = %q, want %q", i, colls[i].ID, id)
		}
	}
}

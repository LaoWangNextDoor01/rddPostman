package collection

import (
	"testing"

	"rddpostman/internal/domain"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

func dragBetween(tt *ui.Tester, fromX, fromY, toX, toY float32) {
	tt.Press(fromX, fromY)
	for k := 1; k <= 5; k++ {
		f := float32(k) / 5
		tt.Move(fromX+(toX-fromX)*f, fromY+(toY-fromY)*f)
	}
	tt.Release(toX, toY)
	tt.Frame()
}

func TestTreeDragAndDropMoveNodes(t *testing.T) {
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ws := db.ActiveWorkspace()
	collA, err := db.CreateCollection(ws.ID, "Core API", "")
	if err != nil {
		t.Fatal(err)
	}
	collB, err := db.CreateCollection(ws.ID, "Admin API", "")
	if err != nil {
		t.Fatal(err)
	}

	req1 := &domain.Node{ID: domain.NewID(), Kind: domain.KindRequest, Name: "List Users", Request: &domain.Request{Method: "GET"}}
	req2 := &domain.Node{ID: domain.NewID(), Kind: domain.KindRequest, Name: "Create User", Request: &domain.Request{Method: "POST"}}
	folder := &domain.Node{ID: domain.NewID(), Kind: domain.KindFolder, Name: "Auth Folder"}
	subReq := &domain.Node{ID: domain.NewID(), Kind: domain.KindRequest, Name: "Get Token", Request: &domain.Request{Method: "POST"}}
	emptyFolder := &domain.Node{ID: domain.NewID(), Kind: domain.KindFolder, Name: "Empty Group"}

	if err := db.AddNode(collA.ID, "", req1); err != nil {
		t.Fatal(err)
	}
	if err := db.AddNode(collA.ID, "", req2); err != nil {
		t.Fatal(err)
	}
	if err := db.AddNode(collA.ID, "", folder); err != nil {
		t.Fatal(err)
	}
	if err := db.AddNode(collA.ID, folder.ID, subReq); err != nil {
		t.Fatal(err)
	}
	if err := db.AddNode(collA.ID, "", emptyFolder); err != nil {
		t.Fatal(err)
	}

	tt := ui.NewTester(func(c *ui.Context) {
		WorkbenchView(c, db, nil, ws.ID, WorkbenchSlots{})
	}, 1000, 720)
	tt.Frame()

	find := func(label string) ui.Rect {
		t.Helper()
		r, ok := tt.Find(label)
		if !ok {
			t.Fatalf("could not find %q in %q", label, tt.Texts())
		}
		return r
	}

	// 1. 短距离点击仍然正常打开 Tab，不误触发拖拽。
	r1Box := find("List Users")
	tt.Press(r1Box.X+10, r1Box.Y+r1Box.H/2)
	tt.Release(r1Box.X+11, r1Box.Y+r1Box.H/2)
	tt.Frame()
	if !hasBreadcrumb(tt.Texts(), "List Users") {
		t.Fatalf("short click should still open tab; got %q", tt.Texts())
	}

	// 2. 同级拖拽排序：把 List Users (0) 拖到 Create User (1) 下半区 -> [Create User, List Users, Auth Folder, Empty Group]
	r1Box = find("List Users")
	r2Box := find("Create User")
	dragBetween(tt, r1Box.X+10, r1Box.Y+r1Box.H/2, r2Box.X+10, r2Box.Y+r2Box.H-3)

	items := db.ActiveWorkspace().Collections[0].Items
	if items[0].ID != req2.ID || items[1].ID != req1.ID {
		t.Fatalf("expected [Create User, List Users], got [%s, %s]", items[0].Name, items[1].Name)
	}

	// 3. 拖入目录（命中目录行中部 50% 区域）：把 Create User 拖入 Auth Folder。
	r2Box = find("Create User")
	folderBox := find("Auth Folder")
	dragBetween(tt, r2Box.X+10, r2Box.Y+r2Box.H/2, folderBox.X+10, folderBox.Y+folderBox.H/2)

	collAState := db.ActiveWorkspace().Collections[0]
	gotFolder := domain.FindIn(collAState.Items, folder.ID)
	if gotFolder == nil || len(gotFolder.Children) != 2 || gotFolder.Children[1].ID != req2.ID {
		t.Fatalf("expected Create User moved into Auth Folder, got children=%+v", gotFolder)
	}

	// 4. 拖入空目录引导区（Empty Group 下的 Folder is empty 占位块）：把 List Users 拖进去。
	r1Box = find("List Users")
	emptyHint := find(locales.FolderEmptyNodeTitle())
	dragBetween(tt, r1Box.X+10, r1Box.Y+r1Box.H/2, emptyHint.X+20, emptyHint.Y+10)

	collAState = db.ActiveWorkspace().Collections[0]
	gotEmpty := domain.FindIn(collAState.Items, emptyFolder.ID)
	if gotEmpty == nil || len(gotEmpty.Children) != 1 || gotEmpty.Children[0].ID != req1.ID {
		t.Fatalf("expected List Users moved into Empty Group via empty placeholder, got %+v", gotEmpty)
	}

	// 5. 跨集合拖拽：把 Empty Group 目录拖到 Admin API 集合行上。
	emptyBox := find("Empty Group")
	collBBox := find("Admin API")
	dragBetween(tt, emptyBox.X+10, emptyBox.Y+emptyBox.H/2, collBBox.X+10, collBBox.Y+collBBox.H/2)

	colls := db.ActiveWorkspace().Collections
	if domain.FindIn(colls[1].Items, emptyFolder.ID) == nil {
		t.Fatalf("expected Empty Group moved to Admin API; collB items=%+v", colls[1].Items)
	}
	if domain.FindIn(colls[1].Items, req1.ID) == nil {
		t.Fatalf("expected List Users subtree moved along with Empty Group")
	}

	// 6. 阻止目录拖入自身子节点：把 Auth Folder 拖到 Get Token 上，应被忽略。
	folderBox = find("Auth Folder")
	subReqBox := find("Get Token")
	dragBetween(tt, folderBox.X+10, folderBox.Y+folderBox.H/2, subReqBox.X+10, subReqBox.Y+subReqBox.H/2)

	if domain.FindIn(db.ActiveWorkspace().Collections[0].Items, folder.ID) == nil {
		t.Fatalf("Auth Folder should remain intact after invalid drop onto descendant")
	}

	// 7. 拖拽集合行重排：把 Admin API (1) 拖到 Core API (0) 上半区 -> [Admin API, Core API]。
	collBBox = find("Admin API")
	collABox := find("Core API")
	dragBetween(tt, collBBox.X+10, collBBox.Y+collBBox.H/2, collABox.X+10, collABox.Y+2)

	colls = db.ActiveWorkspace().Collections
	if colls[0].ID != collB.ID || colls[1].ID != collA.ID {
		t.Fatalf("expected collections reordered to [Admin API, Core API], got [%s, %s]", colls[0].Name, colls[1].Name)
	}

	// 8. 拖拽过程中按 Esc 取消，不发生移动。
	collABox = find("Core API")
	collBBox = find("Admin API")
	tt.Press(collABox.X+10, collABox.Y+collABox.H/2)
	tt.Move(collBBox.X+10, collBBox.Y+2)
	tt.Key(0, ui.KeyEscape)
	tt.Release(collBBox.X+10, collBBox.Y+2)
	tt.Frame()
	colls = db.ActiveWorkspace().Collections
	if colls[0].ID != collB.ID {
		t.Fatalf("Escape should cancel drag; collections=%+v", colls)
	}
}

func TestMoveNodeDialog(t *testing.T) {
	db, err := store.OpenAt(t.TempDir())
	if err != nil {
		t.Fatalf("store.OpenAt: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })

	ws := db.ActiveWorkspace()
	collA, _ := db.CreateCollection(ws.ID, "Source Coll", "")
	_, _ = db.CreateCollection(ws.ID, "Dest Coll", "")
	folderA := &domain.Node{ID: domain.NewID(), Kind: domain.KindFolder, Name: "Folder A"}
	childFolder := &domain.Node{ID: domain.NewID(), Kind: domain.KindFolder, Name: "Nested Child"}
	req := &domain.Node{ID: domain.NewID(), Kind: domain.KindRequest, Name: "Ping", Request: &domain.Request{Method: "GET"}}
	_ = db.AddNode(collA.ID, "", folderA)
	_ = db.AddNode(collA.ID, folderA.ID, childFolder)
	_ = db.AddNode(collA.ID, folderA.ID, req)

	state := newWorkspace()
	state.movePending = &treeNode{
		id:       string(folderA.ID),
		name:     folderA.Name,
		kind:     treeFolder,
		collID:   string(collA.ID),
		parentID: "",
	}
	state.moveDialogOpen = true

	tt := ui.NewTester(func(c *ui.Context) {
		snap := db.ActiveWorkspace()
		props := pageProps{
			workspaceID: snap.ID,
			collections: snap.Collections,
			state:       state,
			actions: pageActions{
				moveNode: func(srcCollID, nodeID, dstCollID, dstParentID domain.ID, dstIndex int) {
					_ = db.MoveNode(srcCollID, nodeID, dstCollID, dstParentID, dstIndex)
				},
			},
		}
		ui.Box(c).Fill()
		moveNodeDialog(c, props)
	}, 800, 600)
	tt.Frame()

	// 弹窗内不应展示被移动目录自身及其子孙目录
	if tt.HasText("Nested Child") {
		t.Fatalf("move dialog should exclude descendant folder 'Nested Child'; got %q", tt.Texts())
	}
	if !tt.HasText("Dest Coll") {
		t.Fatalf("move dialog should list 'Dest Coll'; got %q", tt.Texts())
	}

	if err := tt.Click("Dest Coll"); err != nil {
		t.Fatalf("Click Dest Coll: %v", err)
	}
	tt.Frame()
	if err := tt.Click(locales.MoveNodeConfirm()); err != nil {
		t.Fatalf("Click Move confirm: %v", err)
	}
	tt.Frame()

	if state.moveDialogOpen {
		t.Fatalf("move dialog should close after confirming")
	}
	colls := db.ActiveWorkspace().Collections
	if domain.FindIn(colls[1].Items, folderA.ID) == nil || domain.FindIn(colls[1].Items, req.ID) == nil {
		t.Fatalf("expected Folder A and its child Ping moved into Dest Coll; got %+v", colls[1].Items)
	}
}

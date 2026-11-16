package collection

import (
	"testing"
	"time"

	"rddpostman/internal/domain"

	"github.com/egoist/mygo/ui"
)

func TestOverviewTabRender(t *testing.T) {
	ws := newWorkspace()
	coll := &domain.Collection{
		ID:          "coll-1",
		Name:        "测试集合",
		Description: "# 接口说明\n\n- 任务 1\n- 任务 2\n\n```json\n{\"ok\": true}\n```",
		UpdatedAt:   time.Now(),
		Items: []*domain.Node{
			{
				ID:   "req-1",
				Kind: domain.KindRequest,
				Name: "登录接口",
				Request: &domain.Request{
					Method: "POST",
					URL:    "https://api.example.com/login",
				},
			},
		},
	}

	props := pageProps{
		workspaceID:   "ws-1",
		workspaceName: "默认空间",
		collections:   []*domain.Collection{coll},
		state:         ws,
		actions: pageActions{
			rename: func(id, name string) {},
			updateDescription: func(id, description string) {
				coll.Description = description
			},
			navigate: func(path string) {},
		},
	}

	// 1. 测试查看模式
	view := func(c *ui.Context) {
		overviewTab(c, props, coll, nil)
	}

	tt := ui.NewTester(view, 960, 700)
	tt.Frame()
	img := tt.Image()
	if img == nil {
		t.Fatal("查看模式渲染失败：图像为空")
	}

	// 2. 测试编辑模式（编写 Tab）
	desc := &descState{
		id:      string(coll.ID),
		draft:   coll.Description,
		editing: true,
		mode:    "write",
	}
	title := &titleState{id: string(coll.ID)}

	editView := func(c *ui.Context) {
		ui.Column(c).Width(docWidth).Children(func() {
			editableDescription(c, props, string(coll.ID), coll.Description, desc)
			editableTitle(c, props, string(coll.ID), coll.Name, title)
		})
	}
	ttEdit := ui.NewTester(editView, 960, 700)
	ttEdit.Frame()
	if ttEdit.Image() == nil {
		t.Fatal("编辑模式渲染失败：图像为空")
	}

	// 3. 测试预览 Tab
	desc.mode = "preview"
	ttEdit.Frame()
	if ttEdit.Image() == nil {
		t.Fatal("预览模式渲染失败：图像为空")
	}

	// 4. 测试空描述态
	collEmpty := &domain.Collection{
		ID:        "coll-2",
		Name:      "空描述集合",
		UpdatedAt: time.Now(),
	}
	emptyView := func(c *ui.Context) {
		overviewTab(c, props, collEmpty, nil)
	}
	ttEmpty := ui.NewTester(emptyView, 960, 700)
	ttEmpty.Frame()
	if ttEmpty.Image() == nil {
		t.Fatal("空描述态渲染失败：图像为空")
	}

	// 5. 测试三级目录概览面包屑渲染（集合 -> 目录 1 -> 目录 2）
	subFolder := &domain.Node{
		ID:        "folder-sub",
		Name:      "二级目录",
		Kind:      domain.KindFolder,
		UpdatedAt: time.Now(),
	}
	rootFolder := &domain.Node{
		ID:        "folder-root",
		Name:      "一级目录",
		Kind:      domain.KindFolder,
		UpdatedAt: time.Now(),
		Children:  []*domain.Node{subFolder},
	}
	collMulti := &domain.Collection{
		ID:        "coll-multi",
		Name:      "根集合",
		UpdatedAt: time.Now(),
		Items:     []*domain.Node{rootFolder},
	}
	propsMulti := pageProps{
		workspaceID: "ws-1",
		collections: []*domain.Collection{collMulti},
		state:       ws,
		actions: pageActions{
			openTab: func(id string) {},
		},
	}
	multiView := func(c *ui.Context) {
		overviewTab(c, propsMulti, collMulti, subFolder)
	}
	ttMulti := ui.NewTester(multiView, 960, 700)
	ttMulti.Frame()
	if ttMulti.Image() == nil {
		t.Fatal("多级目录概览渲染失败：图像为空")
	}
}

func TestEditableDescriptionAutoSave(t *testing.T) {
	savedDesc := ""
	props := pageProps{
		actions: pageActions{
			updateDescription: func(id, description string) {
				savedDesc = description
			},
		},
	}

	// 1. 点击“完成”按钮自动保存并退出编辑态
	desc := &descState{
		id:      "coll-1",
		draft:   "新更新的描述内容",
		editing: true,
		mode:    "write",
	}

	view := func(c *ui.Context) {
		ui.Column(c).Width(docWidth).Children(func() {
			editableDescription(c, props, "coll-1", "原始描述", desc)
		})
	}

	tt := ui.NewTester(view, 960, 700)
	tt.Frame()

	// 点击“完成”按钮
	if err := tt.Click("完成"); err != nil {
		t.Fatalf("点击完成按钮失败: %v", err)
	}
	tt.Frame()

	if savedDesc != "新更新的描述内容" {
		t.Errorf("点击完成应自动保存，得到 %q", savedDesc)
	}
	if desc.editing {
		t.Errorf("点击完成应退出编辑模式")
	}

	// 2. 切换到“预览”时自动触发保存最新修改
	desc.editing = true
	desc.draft = "预览中应保存的内容"
	desc.mode = "write"
	savedDesc = ""

	tt.Frame()
	if err := tt.Click("预览"); err != nil {
		t.Fatalf("点击预览按钮失败: %v", err)
	}
	tt.Frame()

	if savedDesc != "预览中应保存的内容" {
		t.Errorf("切换预览应自动保存，得到 %q", savedDesc)
	}
	if desc.mode != "preview" {
		t.Errorf("应进入 preview 模式，得到 %q", desc.mode)
	}
}

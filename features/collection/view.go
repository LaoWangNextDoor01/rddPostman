// Package collection 实现集合工作区：左侧集合树，右侧请求 tab。
package collection

import (
	"rddpostman/components/theme"
	"rddpostman/internal/domain"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"
	"rddpostman/locales"

	"github.com/egoist/mygo/ui"
)

// View 绘制集合工作区。集合数据来自 store，当前工作区由标题栏选择。
func View(c *ui.Context, db *store.Store, n nav.Navigator, workspaceID domain.ID, workspaceName string) {
	m := theme.Metrics
	root := ui.Box(c).Grow(1)
	state := *ui.Local(root, "workspace", newWorkspace)
	width := ui.Local(root, "sidebar-width", func() float32 {
		return m.SidebarWidth
	})
	collections := workspaceCollections(db, workspaceID)
	environments, selectedEnv := workspaceEnvironments(db, workspaceID)
	props := pageProps{
		workspaceID:   workspaceID,
		workspaceName: workspaceName,
		collections:   collections,
		environments:  environments,
		selectedEnv:   selectedEnv,
		state:         state,
	}
	props.actions = pageActions{
		// 创建集合
		createCollection: func() {
			if db == nil {
				return
			}
			// 新建与查看共用同一个概览页：直接建好默认集合再打开。
			coll, err := db.CreateCollection(workspaceID, locales.NewCollection(), "")
			if err != nil {
				return
			}
			state.openTab(string(coll.ID))
		},
		createNode: func(targetID domain.ID, kind domain.Kind) {
			createNodeIn(db, state, collections, kind, string(targetID))
		},
		deleteRow: func(row *treeNode) {
			removeTreeRow(db, state, workspaceID, collections, row)
		},
		selectEnvironment: func(id domain.ID) {
			_ = db.SelectEnvironment(workspaceID, id)
		},
		openTab: func(id string) {
			state.selected = id
			state.openTab(id)
		},
		rename: func(id, name string) {
			coll, node := findNode(collections, id)
			if coll == nil {
				return
			}
			if node == nil {
				_ = db.RenameCollection(coll.ID, name)
				return
			}
			_ = db.RenameNode(coll.ID, node.ID, name)
		},
		navigate: n.Push,
	}
	root.Children(func() {
		ui.Split(c, width, func() {
			sidebar(c, props)
		}, func() {
			mainPane(c, props)
		}).Fill().SplitReset(m.SidebarWidth)
	})
}

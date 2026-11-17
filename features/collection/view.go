// Package collection 实现集合工作区：左侧集合树，右侧请求 tab。
package collection

import (
	"context"
	"time"

	"rddpostman/components/theme"
	"rddpostman/internal/domain"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"
	"rddpostman/internal/transport"

	"github.com/egoist/mygo/ui"
)

// View 绘制集合工作区。集合数据与当前活跃工作区均来自 store 快照。
func View(c *ui.Context, db *store.Store, n nav.Navigator) {
	if db == nil {
		return
	}
	snap := db.Snapshot()
	ws := snap.ActiveWorkspace()
	if ws == nil {
		return
	}
	workspaceID := ws.ID
	workspaceName := ws.Name
	m := theme.Metrics
	root := ui.Box(c).Grow(1)
	state := *ui.Local(root, "workspace-"+string(workspaceID), newWorkspace)
	width := ui.Local(root, "sidebar-width", func() float32 {
		return m.SidebarWidth
	})
	collections := ws.Collections
	environments := ws.Environments
	selectedEnv := ws.SelectedEnvironmentID
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
		createCollection: func(name, desc string) (*domain.Collection, error) {
			coll, err := db.CreateCollection(workspaceID, name, desc)
			if err != nil {
				return nil, err
			}
			return coll, nil
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
		updateDescription: func(id, desc string) {
			coll, node := findNode(collections, id)
			if coll == nil {
				return
			}
			if node == nil {
				_ = db.UpdateCollection(coll.ID, func(c *domain.Collection) {
					c.Description = desc
				})
				return
			}
			_ = db.UpdateNode(coll.ID, node.ID, func(n *domain.Node) {
				n.Description = desc
			})
		},
		updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
			_ = db.UpdateNode(collID, nodeID, fn)
		},
		resolveAuth: func(collID, nodeID domain.ID) domain.Auth {
			return db.ResolveAuth(collID, nodeID)
		},
		sendRequest: func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
			ctx, cancel := context.WithCancel(context.Background())
			go func() {
				// 1. 变量插值与认证继承解析
				prepReq, prepAuth, _, err := db.PrepareRequestWith(workspaceID, collID, nodeID, req, auth)
				if err != nil {
					resp := &transport.Response{
						StatusCode: 0,
						StatusText: "Error",
						Error:      "准备请求参数失败: " + err.Error(),
						Timestamp:  time.Now(),
					}
					onDone(resp)
					c.Invalidate()
					return
				}

				// 2. 真实网络发送
				resp, _ := transport.Send(ctx, prepReq, prepAuth)

				// 3. 记录请求历史
				if resp != nil {
					_ = db.RecordHistory(workspaceID, domain.HistoryEntry{
						WorkspaceID:  workspaceID,
						CollectionID: collID,
						RequestID:    nodeID,
						Method:       prepReq.Method,
						URL:          prepReq.URL,
						Status:       resp.StatusCode,
						DurationMS:   resp.DurationMS,
						Size:         resp.Size,
						Error:        resp.Error,
						At:           resp.Timestamp,
					})
				}

				// 4. 回调响应并触发重绘
				onDone(resp)
				c.Invalidate()
			}()
			return cancel
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

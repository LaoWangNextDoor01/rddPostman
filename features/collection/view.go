// Package collection 实现集合工作区：左侧集合树，右侧请求 tab。
package collection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rddpostman/components/theme"
	"rddpostman/internal/curl"
	"rddpostman/internal/domain"
	"rddpostman/internal/importer"
	"rddpostman/internal/nav"
	"rddpostman/internal/store"
	"rddpostman/internal/transport"

	"github.com/egoist/mygo/ui"
)

// WorkbenchSlots 定义外部 WorkbenchLayout 注入到分栏工作台的槽位。
type WorkbenchSlots struct {
	RenderLeft   func(renderCollections func(), openRequest func(nodeID string), openEnv func(envID domain.ID))
	RenderEnvTab func(envID domain.ID)
}

// View 绘制集合工作区（保留默认单侧栏入口）。
func View(c *ui.Context, db *store.Store, n nav.Navigator) {
	WorkbenchView(c, db, n, "", WorkbenchSlots{})
}

// WorkbenchView 在外部传入的左侧栏容器与常驻右侧多 Tab 编辑区（支持 Request / Overview / Environment）
// 之间建立分栏布局。当左侧在 Collections / History / Environments 间切换时，右侧主编辑区完全保持挂载不动。
func WorkbenchView(
	c *ui.Context,
	db *store.Store,
	n nav.Navigator,
	wsID domain.ID,
	slots WorkbenchSlots,
) {
	if db == nil {
		return
	}
	snap := db.Snapshot()
	var ws *store.Workspace
	if wsID != "" {
		ws = snap.Workspace(wsID)
	}
	if ws == nil {
		ws = snap.ActiveWorkspace()
	}
	if ws == nil {
		return
	}
	workspaceID := ws.ID
	workspaceName := ws.Name
	m := theme.Metrics
	root := ui.Box(c).Fill()
	state := *ui.Local(root, "workspace-"+string(workspaceID), newWorkspace)
	width := ui.Local(root, "sidebar-width", func() float32 {
		return m.SidebarWidth
	})
	if *width < 180 {
		*width = 180
	} else if *width > 480 {
		*width = 480
	}
	collections := ws.Collections
	environments := ws.Environments
	selectedEnv := ws.SelectedEnvironmentID
	state.pruneMissingEnvs(environments)
	props := pageProps{
		workspaceID:   workspaceID,
		workspaceName: workspaceName,
		collections:   collections,
		environments:  environments,
		selectedEnv:   selectedEnv,
		state:         state,
	}
	props.actions = pageActions{
		// 新建集合并直接以 Tab 形式在右侧打开（模仿 Postman，不弹模态框）
		createCollectionTab: func() {
			name := nextCollectionName(collections)
			coll, err := db.CreateCollection(workspaceID, name, "")
			if err != nil || coll == nil {
				return
			}
			state.openTab(string(coll.ID))
			state.selected = string(coll.ID)
			state.focusNewCollID = string(coll.ID)
		},
		createNode: func(targetID domain.ID, kind domain.Kind) {
			createNodeIn(db, state, collections, kind, string(targetID))
		},
		duplicateNode: func(nodeID string) {
			coll, node := findNode(collections, nodeID)
			if coll == nil || node == nil {
				return
			}
			cloned, err := db.DuplicateNode(coll.ID, node.ID)
			if err != nil {
				return
			}
			state.openTab(string(cloned.ID))
			state.selected = string(cloned.ID)
		},
		importCurl: func(targetID domain.ID, curlCmd string) error {
			req, parsedAuth, err := curl.FromCurl(curlCmd)
			if err != nil {
				return err
			}
			coll, parentNode := findNode(collections, string(targetID))
			if coll == nil {
				if len(collections) > 0 {
					coll = collections[0]
				} else {
					var createErr error
					coll, createErr = db.CreateCollection(workspaceID, "默认集合", "")
					if createErr != nil {
						return createErr
					}
				}
			}
			reqName := "cURL Request"
			if req.URL != "" {
				parts := strings.Split(strings.TrimRight(req.URL, "/"), "/")
				if len(parts) > 0 && parts[len(parts)-1] != "" {
					reqName = parts[len(parts)-1]
				}
			}
			node := &domain.Node{
				ID:      domain.NewID(),
				Kind:    domain.KindRequest,
				Name:    reqName,
				Request: &req,
				Auth:    parsedAuth,
			}
			parentID := domain.ID("")
			if parentNode != nil && parentNode.Kind == domain.KindFolder {
				parentID = parentNode.ID
			} else if parentNode != nil && parentNode.Kind == domain.KindRequest {
				if p, ok := domain.ParentOf(coll.Items, parentNode.ID); ok && p != nil {
					parentID = p.ID
				}
			}
			if err := db.AddNode(coll.ID, parentID, node); err != nil {
				return err
			}
			state.openTab(string(node.ID))
			state.selected = string(node.ID)
			return nil
		},
		importCollection: func(jsonData string) error {
			parsed, err := importer.ParsePostmanCollection([]byte(jsonData))
			if err != nil {
				return err
			}
			imported, err := db.ImportCollection(workspaceID, parsed)
			if err != nil {
				return err
			}
			state.openTab(string(imported.ID))
			state.selected = string(imported.ID)
			return nil
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
		openEnv: func(id domain.ID) {
			state.openTab(EnvTabID(id))
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
		updateAuth: func(id string, auth domain.Auth) {
			coll, node := findNode(collections, id)
			if coll == nil {
				return
			}
			if node == nil {
				_ = db.UpdateCollection(coll.ID, func(c *domain.Collection) {
					c.Auth = auth
				})
				return
			}
			_ = db.UpdateNode(coll.ID, node.ID, func(n *domain.Node) {
				n.Auth = auth
			})
		},
		updateVariables: func(id string, vars []domain.KV) {
			coll, node := findNode(collections, id)
			if coll == nil {
				return
			}
			if node == nil {
				_ = db.UpdateCollection(coll.ID, func(c *domain.Collection) {
					c.Variables = vars
				})
				return
			}
			_ = db.UpdateNode(coll.ID, node.ID, func(n *domain.Node) {
				n.Variables = vars
			})
		},
		updateNode: func(collID, nodeID domain.ID, fn func(*domain.Node)) {
			_ = db.UpdateNode(collID, nodeID, fn)
		},
		saveRequest: func(id string, req domain.Request, auth domain.Auth, name string) {
			// 1. 如果该请求是尚未入库的草稿
			if state != nil && state.drafts != nil {
				if draft, ok := state.drafts[id]; ok {
					node := draft.node
					if strings.TrimSpace(name) != "" {
						node.Name = strings.TrimSpace(name)
					}
					node.Request = req.Clone()
					node.Auth = auth
					if err := db.AddNode(draft.collID, draft.parentID, node); err != nil {
						return
					}
					delete(state.drafts, id)
					state.setUnsaved(id, false)
					return
				}
			}

			// 2. 如果是已有持久化节点
			coll, node := findNode(collections, id)
			if coll != nil && node != nil {
				_ = db.UpdateNode(coll.ID, node.ID, func(n *domain.Node) {
					if strings.TrimSpace(name) != "" {
						n.Name = strings.TrimSpace(name)
					}
					n.Request = req.Clone()
					n.Auth = auth
				})
				if state != nil {
					state.setUnsaved(id, false)
				}
			}
		},
		resolveAuth: func(collID, nodeID domain.ID) domain.Auth {
			return db.ResolveAuth(collID, nodeID)
		},
		resolveVariables: func(collID, nodeID domain.ID) map[string]domain.Variable {
			if db == nil {
				return nil
			}
			return db.Variables(workspaceID, collID, nodeID)
		},
		sendRequest: func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func() {
			ctx, cancel := context.WithCancel(context.Background())
			// Context 是构建期的值，不能带进 goroutine；Services 可以。
			services := c.Services()
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
					services.Invalidate()
					return
				}

				// 2. 真实网络发送
				resp, _ := transport.Send(ctx, prepReq, prepAuth)

				// 3. 记录请求历史并同步由脚本更新的环境变量（pm.environment.set）
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

					if len(resp.UpdatedEnv) > 0 && selectedEnv != "" {
						for k, v := range resp.UpdatedEnv {
							_ = db.SetEnvironmentVariable(workspaceID, selectedEnv, k, v)
						}
					}
				}

				// 4. 回调响应并触发重绘
				onDone(resp)
				services.Invalidate()
			}()
			return cancel
		},
		renderEnvTab: slots.RenderEnvTab,
		navigate: func(target string) {
			if n != nil {
				n.Push(target)
			}
		},
	}
	renderCollections := func() {
		sidebar(c, props)
	}
	openEnv := func(envID domain.ID) {
		state.openTab(EnvTabID(envID))
	}
	root.Children(func() {
		ui.Split(c, width, func() {
			if slots.RenderLeft != nil {
				slots.RenderLeft(renderCollections, props.actions.openTab, openEnv)
			} else {
				renderCollections()
			}
		}, func() {
			mainPane(c, props)
		}).Fill().SplitReset(m.SidebarWidth)
	})

	importCurlDialog(c, props)
	importPostmanDialog(c, props)
}

func nextCollectionName(existing []*domain.Collection) string {
	used := make(map[string]bool, len(existing))
	for _, c := range existing {
		used[strings.ToLower(strings.TrimSpace(c.Name))] = true
	}
	if !used["new collection"] {
		return "New Collection"
	}
	for i := 2; ; i++ {
		cand := fmt.Sprintf("New Collection %d", i)
		if !used[strings.ToLower(cand)] {
			return cand
		}
	}
}

// Package collection 实现集合工作区：左侧集合树，右侧请求 tab。
package collection

import (
	"context"
	"fmt"
	"strings"
	"time"

	"rddpostman/components/theme"
	v2 "rddpostman/components/v2"
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
	RenderLeft     func(renderCollections func(), openRequest func(nodeID string), openEnv func(envID domain.ID), openTool func(toolID string), activeToolID string)
	RenderEnvTab   func(envID domain.ID)
	RenderToolTab  func(toolID string)
	ToolTabLabel   func(toolID string) string
	ConsumeCommand func() *WorkbenchCommand
	ModalOpen      bool
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
	shortcutsDisabled := slots.ModalOpen || state.importCurlOpen || state.importPostmanOpen
	props := pageProps{
		workspaceID:       workspaceID,
		workspaceName:     workspaceName,
		collections:       collections,
		environments:      environments,
		selectedEnv:       selectedEnv,
		shortcutsDisabled: shortcutsDisabled,
		state:             state,
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
		openTool: func(toolID string) {
			state.openTab(ToolTabID(toolID))
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
				resp := executeRequestSync(ctx, db, workspaceID, selectedEnv, collID, nodeID, req, auth)
				onDone(resp)
				services.Invalidate()
			}()
			return cancel
		},
		runRequestSync: func(ctx context.Context, collID, nodeID domain.ID, req domain.Request, auth domain.Auth) *transport.Response {
			return executeRequestSync(ctx, db, workspaceID, selectedEnv, collID, nodeID, req, auth)
		},
		renderEnvTab:  slots.RenderEnvTab,
		renderToolTab: slots.RenderToolTab,
		toolTabLabel:  slots.ToolTabLabel,
		navigate: func(target string) {
			if n != nil {
				n.Push(target)
			}
		},
	}

	// 消费来自菜单、全局搜索或全局快捷键的工作台指令
	if slots.ConsumeCommand != nil {
		if cmd := slots.ConsumeCommand(); cmd != nil {
			switch cmd.Kind {
			case CmdNewRequest:
				if len(collections) > 0 {
					props.actions.createNode(collections[0].ID, domain.KindRequest)
				} else if props.actions.createCollectionTab != nil {
					props.actions.createCollectionTab()
				}
			case CmdNewCollection:
				if props.actions.createCollectionTab != nil {
					props.actions.createCollectionTab()
				}
			case CmdImportPostman:
				state.importPostmanOpen = true
				state.importPostmanText = ""
				state.importPostmanErr = ""
			case CmdImportCurl:
				state.importCurlOpen = true
				state.importCurlText = ""
				state.importCurlErr = ""
			case CmdOpenTab:
				if cmd.TargetID != "" {
					props.actions.openTab(cmd.TargetID)
				}
			case CmdOpenEnv:
				if cmd.EnvID != "" {
					props.actions.openEnv(cmd.EnvID)
				}
			case CmdOpenTool:
				if cmd.ToolID != "" {
					props.actions.openTool(cmd.ToolID)
				}
			case CmdCloseActiveTab:
				if state.active != "" {
					state.closeTab(state.active)
				}
			case CmdNextTab:
				state.nextTab()
			case CmdPrevTab:
				state.prevTab()
			case CmdFocusURL:
				state.focusURLReq = true
			}
		}
	}

	// 工作台级 Tab 快捷键：关闭当前 Tab (Cmd/Ctrl+W)、切换相邻 Tab (Ctrl+Tab / Ctrl+Shift+Tab / Cmd+Alt+←/→)
	if !shortcutsDisabled {
		if c.Shortcut(ui.Cmd, ui.KeyW) {
			if state.active != "" {
				state.closeTab(state.active)
			}
		}
		if c.Shortcut(ui.Ctrl, ui.KeyTab) || c.Shortcut(ui.Cmd|ui.Alt, ui.KeyRight) {
			state.nextTab()
		}
		if c.Shortcut(ui.Ctrl|ui.Shift, ui.KeyTab) || c.Shortcut(ui.Cmd|ui.Alt, ui.KeyLeft) {
			state.prevTab()
		}
	}
	renderCollections := func() {
		sidebar(c, props)
	}
	openEnv := func(envID domain.ID) {
		state.openTab(EnvTabID(envID))
	}
	openTool := func(toolID string) {
		state.openTab(ToolTabID(toolID))
	}
	activeToolID, _ := parseToolTabID(state.active)
	root.Children(func() {
		ui.Split(c, width, func() {
			if slots.RenderLeft != nil {
				slots.RenderLeft(renderCollections, props.actions.openTab, openEnv, openTool, activeToolID)
			} else {
				renderCollections()
			}
		}, func() {
			mainPane(c, props)
		}).Fill().SplitReset(m.SidebarWidth)
	})

	importCurlDialog(c, props)
	importPostmanDialog(c, props)
	deleteTreeRowDialog(c, props)
}

func deleteTreeRowDialog(c *ui.Context, props pageProps) {
	state := props.state
	if state == nil || state.deletePending == nil {
		return
	}
	target := state.deletePending
	title := "删除请求"
	desc := fmt.Sprintf("确定要删除请求「%s」吗？此操作不可撤销。", target.name)
	switch target.kind {
	case treeCollection:
		title = "删除集合"
		desc = fmt.Sprintf("确定要删除集合「%s」吗？该集合下的所有目录和请求也将一并删除，此操作不可撤销。", target.name)
	case treeFolder:
		title = "删除目录"
		desc = fmt.Sprintf("确定要删除目录「%s」吗？该目录下的所有子目录和请求也将一并删除，此操作不可撤销。", target.name)
	}
	v2.ConfirmDialog(c, &state.deleteConfirmOpen, v2.ConfirmDialogOpts{
		Key:          "coll-del-confirm",
		Title:        title,
		Description:  desc,
		ConfirmLabel: "确认删除",
		CancelLabel:  "取消",
		Danger:       true,
		OnConfirm: func() {
			row := state.deletePending
			state.deletePending = nil
			if row != nil && props.actions.deleteRow != nil {
				props.actions.deleteRow(row)
			}
		},
		OnCancel: func() {
			state.deletePending = nil
		},
	})
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

func executeRequestSync(
	ctx context.Context,
	db *store.Store,
	workspaceID, selectedEnv, collID, nodeID domain.ID,
	req domain.Request,
	auth domain.Auth,
) *transport.Response {
	prepReq, prepAuth, _, err := db.PrepareRequestWith(workspaceID, collID, nodeID, req, auth)
	if err != nil {
		return &transport.Response{
			StatusCode: 0,
			StatusText: "Error",
			Error:      "准备请求参数失败: " + err.Error(),
			Timestamp:  time.Now(),
		}
	}

	resp, _ := transport.Send(ctx, prepReq, prepAuth)
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
	return resp
}

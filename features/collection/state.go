package collection

import (
	"context"

	"rddpostman/internal/domain"
	"rddpostman/internal/transport"
)

// 页面状态与动作写在这里，仅 collection 包内使用。

// pageProps 是传给各子组件的只读快照与会话状态。
type pageProps struct {
	workspaceID       domain.ID
	workspaceName     string
	collections       []*domain.Collection
	environments      []domain.Environment
	selectedEnv       domain.ID
	shortcutsDisabled bool
	state             *workspace
	actions           pageActions
}

// pageActions 是所有写操作的外置回调，统一由 View 接到 store。
type pageActions struct {
	createCollectionTab func()
	createNode          func(targetID domain.ID, kind domain.Kind)
	duplicateNode     func(nodeID string)
	importCurl        func(targetID domain.ID, curlCmd string) error
	importCollection  func(jsonData string) error
	deleteRow         func(row *treeNode)
	selectEnvironment func(id domain.ID)
	openTab           func(id string)
	openEnv           func(id domain.ID)
	openTool          func(toolID string)
	rename            func(id, name string)
	updateDescription func(id, description string)
	updateAuth        func(id string, auth domain.Auth)
	updateVariables   func(id string, vars []domain.KV)
	updateNode        func(collID, nodeID domain.ID, fn func(*domain.Node))
	saveRequest       func(id string, req domain.Request, auth domain.Auth, name string)
	sendRequest       func(collID, nodeID domain.ID, req domain.Request, auth domain.Auth, onDone func(*transport.Response)) func()
	runRequestSync    func(ctx context.Context, collID, nodeID domain.ID, req domain.Request, auth domain.Auth) *transport.Response
	resolveAuth       func(collID, nodeID domain.ID) domain.Auth
	resolveVariables  func(collID, nodeID domain.ID) map[string]domain.Variable
	renderEnvTab      func(envID domain.ID)
	renderToolTab     func(toolID string)
	toolTabLabel      func(toolID string) string
	navigate          func(path string)
}

package collection

import "rddpostman/internal/domain"

// 页面状态与动作写在这里，仅 collection 包内使用。

// pageProps 是传给各子组件的只读快照与会话状态。
type pageProps struct {
	workspaceID   domain.ID
	workspaceName string
	collections   []*domain.Collection
	environments  []domain.Environment
	selectedEnv   domain.ID
	state         *workspace
	actions       pageActions
}

// pageActions 是所有写操作的外置回调，统一由 View 接到 store。
type pageActions struct {
	createCollection  func(name, description string) (*domain.Collection, error)
	createNode        func(targetID domain.ID, kind domain.Kind)
	deleteRow         func(row *treeNode)
	selectEnvironment func(id domain.ID)
	openTab           func(id string)
	rename            func(id, name string)
	updateDescription func(id, description string)
	updateNode        func(collID, nodeID domain.ID, fn func(*domain.Node))
	navigate          func(path string)
}

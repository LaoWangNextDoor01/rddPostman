package collection

import (
	"rddpostman/internal/domain"
	"rddpostman/internal/store"
	"rddpostman/locales"
)

// createNodeIn 在 targetID 对应的容器里新建目录或请求：
// 集合/目录建在自身，请求建在它所在的父目录。
func createNodeIn(db *store.Store, state *workspace, collections []*domain.Collection,
	kind domain.Kind, targetID string) {
	coll, target := findNode(collections, targetID)
	if coll == nil {
		if len(collections) == 0 {
			return
		}
		coll = collections[0]
	}
	parent := domain.ID("")
	if target != nil {
		if target.Kind == domain.KindFolder {
			parent = target.ID
		} else if p, ok := domain.ParentOf(coll.Items, target.ID); ok && p != nil {
			parent = p.ID
		}
	}

	n := &domain.Node{ID: store.NewID(), Kind: kind}
	if kind == domain.KindFolder {
		n.Name = locales.NewFolder()
		if err := db.AddNode(coll.ID, parent, n); err != nil {
			return
		}
		state.selected = string(n.ID)
		state.openTab(string(n.ID))
		return
	}

	// 新建请求：默认不要保存，以未保存草稿状态打开 Tab，Tab 显示未保存小圆点
	// 默认设置 Postman 规范的标准默认请求头（用户可自由修改、禁用、删除或追加）
	n.Name = locales.NewRequest()
	n.Request = &domain.Request{
		Method:  "GET",
		URL:     "",
		Headers: domain.DefaultRequestHeaders(),
	}
	state.addDraft(coll.ID, parent, n)
}

// removeTreeRow 删除树行：集合删整个集合，目录/请求删对应节点，
// 然后把已失效的 tab 清理掉。
func removeTreeRow(db *store.Store, state *workspace, workspaceID domain.ID,
	collections []*domain.Collection, row *treeNode) {
	if row.kind == treeCollection {
		if err := db.RemoveCollection(domain.ID(row.id)); err != nil {
			return
		}
	} else {
		coll, _ := findNode(collections, row.id)
		if coll == nil {
			return
		}
		if err := db.DeleteNode(coll.ID, domain.ID(row.id)); err != nil {
			return
		}
	}
	state.pruneTabs(workspaceCollections(db, workspaceID))
}

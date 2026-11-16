package collection

import (
	"strings"

	"rddpostman/internal/domain"
	"rddpostman/internal/store"
)

// workspaceEnvironments 返回工作区的环境列表与当前选中的环境 ID。
func workspaceEnvironments(db *store.Store, id domain.ID) ([]domain.Environment, domain.ID) {
	if db == nil {
		return nil, ""
	}
	for _, w := range db.Snapshot().Workspaces {
		if w.ID == id {
			return w.Environments, w.SelectedEnvironmentID
		}
	}
	return nil, ""
}

func workspaceCollections(db *store.Store, id domain.ID) []*domain.Collection {
	if db == nil {
		return nil
	}
	for _, w := range db.Snapshot().Workspaces {
		if w.ID == id {
			return w.Collections
		}
	}
	return nil
}

func findNode(collections []*domain.Collection, id string) (*domain.Collection, *domain.Node) {
	if id == "" {
		return nil, nil
	}
	for _, coll := range collections {
		if string(coll.ID) == id {
			return coll, nil
		}
		if n := domain.FindIn(coll.Items, domain.ID(id)); n != nil {
			return coll, n
		}
	}
	return nil, nil
}

// CrumbItem 表示面包屑中的一级。
type CrumbItem struct {
	ID   string
	Name string
	Kind domain.Kind // KindFolder, KindRequest, 或 "" (表示集合)
}

// nodeTrail 返回从集合根到指定节点的所有层级项（包含集合自身和当前节点）。
func nodeTrail(collections []*domain.Collection, id string) []CrumbItem {
	coll, node := findNode(collections, id)
	if coll == nil {
		return nil
	}
	crumbs := []CrumbItem{
		{ID: string(coll.ID), Name: coll.Name},
	}
	if node == nil {
		return crumbs
	}
	trail := findNodeTrail(coll.Items, node.ID, nil)
	return append(crumbs, trail...)
}

func findNodeTrail(items []*domain.Node, id domain.ID, acc []CrumbItem) []CrumbItem {
	for _, n := range items {
		cur := CrumbItem{
			ID:   string(n.ID),
			Name: n.Name,
			Kind: n.Kind,
		}
		next := append(append([]CrumbItem(nil), acc...), cur)
		if n.ID == id {
			return next
		}
		if got := findNodeTrail(n.Children, id, next); got != nil {
			return got
		}
	}
	return nil
}

// nodePath 返回「集合 / 目录 / 请求」形式的路径字符串。
func nodePath(collections []*domain.Collection, id string) string {
	crumbs := nodeTrail(collections, id)
	if len(crumbs) == 0 {
		return ""
	}
	names := make([]string, len(crumbs))
	for i, c := range crumbs {
		names[i] = c.Name
	}
	return strings.Join(names, " / ")
}

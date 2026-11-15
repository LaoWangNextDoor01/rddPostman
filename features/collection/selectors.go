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

// nodePath 返回「集合 / 目录 / 请求」形式的路径，用于面包屑。
func nodePath(collections []*domain.Collection, id string) string {
	coll, node := findNode(collections, id)
	if coll == nil {
		return ""
	}
	if node == nil {
		return coll.Name
	}
	if trail, ok := findTrail(coll.Items, node.ID, nil); ok {
		return coll.Name + " / " + strings.Join(trail, " / ")
	}
	return coll.Name
}

func findTrail(items []*domain.Node, id domain.ID, trail []string) ([]string, bool) {
	for _, n := range items {
		next := append(trail, n.Name)
		if n.ID == id {
			return next, true
		}
		if got, ok := findTrail(n.Children, id, next); ok {
			return got, true
		}
	}
	return nil, false
}

package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

// ---- 集合 ----

// CreateCollection 在工作区的 collections/ 下新建集合。
func (s *Store) CreateCollection(wsID domain.ID, name, description string) (*domain.Collection, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return nil, ErrNotFound
	}
	if st.readOnly {
		return nil, ErrReadOnly
	}
	coll, err := s.backend.CreateCollection(context.Background(), st.ref,
		storage.Input{Name: name, Description: description})
	if err != nil {
		return nil, err
	}
	st.collections = append(st.collections, coll)
	s.collIndex[coll.ID] = st
	sortCollections(st.collections)
	s.publishLocked()
	return coll, nil
}

// RenameCollection 修改集合名，目录一起改名。
func (s *Store) RenameCollection(collID domain.ID, name string) error {
	return s.UpdateCollection(collID, func(c *domain.Collection) {
		c.Name = name
	})
}

// UpdateCollection 修改集合元数据。
func (s *Store) UpdateCollection(collID domain.ID, fn func(*domain.Collection)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := old.Clone()
	fn(next)
	next.UpdatedAt = time.Now()
	if err := s.backend.SaveCollection(context.Background(), st.ref, next); err != nil {
		return err
	}
	s.replaceCollectionLocked(st, next)
	s.publishLocked()
	return nil
}

// RemoveCollection 删除集合及其全部文件。
func (s *Store) RemoveCollection(collID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	if err := s.backend.DeleteCollection(context.Background(), st.ref, old.ID); err != nil {
		return err
	}
	delete(s.collIndex, old.ID)
	out := st.collections[:0]
	for _, c := range st.collections {
		if c.ID != old.ID {
			out = append(out, c)
		}
	}
	st.collections = out
	if st.local.CollectionEnvironments != nil {
		if _, ok := st.local.CollectionEnvironments[old.ID]; ok {
			next := st.local.Clone()
			delete(next.CollectionEnvironments, old.ID)
			_ = s.backend.SaveLocal(context.Background(), st.ref, next)
			st.local = next
		}
	}
	for key := range s.dirtyNode {
		if key.coll == old.ID {
			delete(s.dirtyNode, key)
		}
	}
	s.publishLocked()
	return nil
}

// ---- 目录与请求 ----

// AddNode 在 parentID 下追加节点；parentID 为空时加到集合根层。
func (s *Store) AddNode(collID, parentID domain.ID, n *domain.Node) error {
	if n == nil {
		return errors.New("节点为空")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := old.Clone()
	prepareNode(n)
	if parentID == "" {
		n.Order = domain.NextOrder(next.Items)
		next.Items = append(next.Items, n)
	} else {
		parent := domain.FindIn(next.Items, parentID)
		if parent == nil {
			return ErrNotFound
		}
		if parent.Kind != domain.KindFolder {
			return errors.New("只能在目录下创建")
		}
		n.Order = domain.NextOrder(parent.Children)
		parent.Children = append(parent.Children, n)
	}
	next.UpdatedAt = time.Now()

	persist, err := s.preparePersistNodeLocked(st, n)
	if err != nil {
		return err
	}
	if err := s.backend.SaveNode(context.Background(), st.ref, collID, parentID, persist); err != nil {
		return err
	}
	delete(s.dirtyNode, nodeKey{ws: st.ref.ID, coll: collID, node: n.ID})
	s.replaceCollectionLocked(st, next)
	s.publishLocked()
	return nil
}

// RenameNode 修改节点名称，文件/目录一起改名。
func (s *Store) RenameNode(collID, id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("名称不能为空")
	}
	return s.updateNodePersist(collID, id, func(n *domain.Node) {
		n.Name = name
	})
}

// UpdateNode 修改节点（名称、变量、认证等），并立即持久化结构。
func (s *Store) UpdateNode(collID, id domain.ID, fn func(*domain.Node)) error {
	return s.updateNodePersist(collID, id, fn)
}

func (s *Store) updateNodePersist(collID, id domain.ID, fn func(*domain.Node)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := old.Clone()
	n := domain.FindIn(next.Items, id)
	if n == nil {
		return ErrNotFound
	}
	fn(n)
	n.UpdatedAt = time.Now()
	next.UpdatedAt = time.Now()

	persist, err := s.preparePersistNodeLocked(st, n)
	if err != nil {
		return err
	}
	parentID := domain.ID("")
	if p, ok := domain.ParentOf(next.Items, id); ok && p != nil {
		parentID = p.ID
	}
	if err := s.backend.SaveNode(context.Background(), st.ref, collID, parentID, persist); err != nil {
		return err
	}
	// 结构写入已经落盘，丢掉可能存在的旧防抖副本，避免旧名字覆盖新名字。
	delete(s.dirtyNode, nodeKey{ws: st.ref.ID, coll: collID, node: id})
	s.replaceCollectionLocked(st, next)
	s.publishLocked()
	return nil
}

// DeleteNode 删除节点及其子树。
func (s *Store) DeleteNode(collID, id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := old.Clone()
	target := domain.FindIn(next.Items, id)
	if target == nil {
		return ErrNotFound
	}
	removed := nodeIDs(target)
	items, ok := domain.RemoveChild(next.Items, id)
	if !ok {
		return ErrNotFound
	}
	if err := s.backend.DeleteNode(context.Background(), st.ref, collID, id); err != nil {
		return err
	}
	next.Items = items
	next.UpdatedAt = time.Now()
	for _, nid := range removed {
		delete(s.dirtyNode, nodeKey{ws: st.ref.ID, coll: collID, node: nid})
	}
	s.replaceCollectionLocked(st, next)
	s.publishLocked()
	return nil
}

// UpdateRequest 修改请求配置。高频编辑走防抖，不阻塞界面。
func (s *Store) UpdateRequest(collID, id domain.ID, fn func(*domain.Request)) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := old.Clone()
	n := domain.FindIn(next.Items, id)
	if n == nil || n.Request == nil {
		return ErrNotFound
	}
	fn(n.Request)
	n.UpdatedAt = time.Now()
	next.UpdatedAt = time.Now()

	s.replaceCollectionLocked(st, next)
	s.dirtyNode[nodeKey{ws: st.ref.ID, coll: collID, node: id}] = n.Clone()
	s.scheduleLocked()
	s.publishLocked()
	return nil
}

// prepareNode 补齐新建节点需要的元数据。
func prepareNode(n *domain.Node) {
	if n.ID == "" {
		n.ID = domain.NewID()
	}
	if n.Kind == "" {
		if n.Request != nil {
			n.Kind = domain.KindRequest
		} else {
			n.Kind = domain.KindFolder
		}
	}
	if n.Name = strings.TrimSpace(n.Name); n.Name == "" {
		n.Name = "未命名"
	}
	if n.Kind == domain.KindRequest && n.Request == nil {
		n.Request = &domain.Request{}
	}
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	n.UpdatedAt = now
	if n.Request != nil {
		if n.Request.CreatedAt.IsZero() {
			n.Request.CreatedAt = now
		}
		n.Request.UpdatedAt = now
	}
}

// nodeIDs 返回节点及其子树的全部 ID。
func nodeIDs(n *domain.Node) []domain.ID {
	if n == nil {
		return nil
	}
	out := []domain.ID{n.ID}
	for _, ch := range n.Children {
		out = append(out, nodeIDs(ch)...)
	}
	return out
}

func nodeExists(st *state, collID, nodeID domain.ID) bool {
	for _, c := range st.collections {
		if c.ID == collID {
			return domain.FindIn(c.Items, nodeID) != nil
		}
	}
	return false
}

// DuplicateNode 克隆已有节点（请求或目录）并放置在同级位置。
func (s *Store) DuplicateNode(collID, nodeID domain.ID) (*domain.Node, error) {
	s.mu.Lock()
	st, old := s.findCollectionLocked(collID)
	if st == nil {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	src := domain.FindIn(old.Items, nodeID)
	if src == nil {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	parentID := domain.ID("")
	if p, ok := domain.ParentOf(old.Items, nodeID); ok && p != nil {
		parentID = p.ID
	}
	clone := src.Clone()
	clone.ID = domain.NewID()
	clone.Name = clone.Name + " Copy"
	s.mu.Unlock()

	if err := s.AddNode(collID, parentID, clone); err != nil {
		return nil, err
	}
	return clone, nil
}

// ImportCollection 将一个完整的集合树（包括其子目录、请求和变量）导入到指定工作区。
func (s *Store) ImportCollection(wsID domain.ID, coll *domain.Collection) (*domain.Collection, error) {
	if coll == nil {
		return nil, errors.New("集合为空")
	}
	createdColl, err := s.CreateCollection(wsID, coll.Name, coll.Description)
	if err != nil {
		return nil, err
	}
	if len(coll.Variables) > 0 || coll.Auth.Type != "" {
		_ = s.UpdateCollection(createdColl.ID, func(c *domain.Collection) {
			c.Variables = coll.Variables
			c.Auth = coll.Auth
		})
	}

	var addSubTree func(parentID domain.ID, nodes []*domain.Node) error
	addSubTree = func(parentID domain.ID, nodes []*domain.Node) error {
		for _, n := range nodes {
			cloned := n.Clone()
			cloned.ID = domain.NewID()
			children := cloned.Children
			cloned.Children = nil
			if err := s.AddNode(createdColl.ID, parentID, cloned); err != nil {
				return err
			}
			if len(children) > 0 {
				if err := addSubTree(cloned.ID, children); err != nil {
					return err
				}
			}
		}
		return nil
	}

	if err := addSubTree("", coll.Items); err != nil {
		return nil, err
	}

	s.mu.Lock()
	_, finalColl := s.findCollectionLocked(createdColl.ID)
	s.mu.Unlock()
	return finalColl, nil
}

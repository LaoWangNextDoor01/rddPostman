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

// MoveNode 将节点 nodeID（目录连同子树，或单个请求）移动到目标集合 dstCollID 的 dstParentID 下
// （dstParentID 为空表示集合根层）的第 dstIndex 个位置。
// dstIndex 按移动前的目标子列表计，小于 0 或超出长度时追加到末尾。
// 支持同级重排、跨目录移动，以及同一工作区内的跨集合移动。
func (s *Store) MoveNode(srcCollID, nodeID, dstCollID, dstParentID domain.ID, dstIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	srcSt, oldSrc := s.findCollectionLocked(srcCollID)
	dstSt, oldDst := s.findCollectionLocked(dstCollID)
	if srcSt == nil || dstSt == nil {
		return ErrNotFound
	}
	if srcSt != dstSt {
		return ErrInvalidMove
	}
	if srcSt.readOnly {
		return ErrReadOnly
	}
	srcNode := domain.FindIn(oldSrc.Items, nodeID)
	if srcNode == nil {
		return ErrNotFound
	}
	if dstParentID != "" {
		dstParent := domain.FindIn(oldDst.Items, dstParentID)
		if dstParent == nil {
			return ErrNotFound
		}
		if dstParent.Kind != domain.KindFolder {
			return ErrInvalidMove
		}
		if srcCollID == dstCollID && srcNode.Kind == domain.KindFolder && srcNode.Find(dstParentID) != nil {
			return ErrInvalidMove
		}
	}

	var oldParentID domain.ID
	if p, ok := domain.ParentOf(oldSrc.Items, nodeID); ok && p != nil {
		oldParentID = p.ID
	}
	oldSiblings := containerItems(oldSrc, oldParentID)
	oldIndex := indexOfNode(oldSiblings, nodeID)
	if oldIndex < 0 {
		return ErrNotFound
	}

	sameParent := srcCollID == dstCollID && oldParentID == dstParentID
	targetLen := len(containerItems(oldDst, dstParentID))
	insertIdx := resolveInsertIndex(dstIndex, targetLen, oldIndex, sameParent)
	if sameParent && insertIdx == oldIndex {
		return nil
	}

	// 移动前先将待移动子树上的防抖修改落盘，避免跨集合移动后旧键丢失或写回原处。
	if err := s.flushSubtreeLocked(srcSt, srcCollID, srcNode); err != nil {
		return err
	}

	now := time.Now()
	nextSrc := oldSrc.Clone()
	nextDst := nextSrc
	if dstCollID != srcCollID {
		nextDst = oldDst.Clone()
	}

	moved := domain.FindIn(nextSrc.Items, nodeID)
	if moved == nil {
		return ErrNotFound
	}
	movedClone := moved.Clone()
	removedItems, ok := domain.RemoveChild(nextSrc.Items, nodeID)
	if !ok {
		return ErrNotFound
	}
	nextSrc.Items = removedItems
	nextSrc.UpdatedAt = now
	if nextDst != nextSrc {
		nextDst.UpdatedAt = now
	}

	dstSiblings := containerItems(nextDst, dstParentID)
	updatedSiblings, rebalanced := insertNodeWithOrder(dstSiblings, movedClone, insertIdx)
	setContainerItems(nextDst, dstParentID, updatedSiblings)
	movedClone.UpdatedAt = now

	persistMoved, err := s.preparePersistNodeLocked(srcSt, movedClone)
	if err != nil {
		return err
	}
	ctx := context.Background()
	if err := s.backend.MoveNode(ctx, srcSt.ref, srcCollID, nodeID, dstCollID, dstParentID, persistMoved); err != nil {
		return err
	}
	for _, sib := range rebalanced {
		sib.UpdatedAt = now
		persistSib, err := s.preparePersistNodeLocked(srcSt, sib)
		if err != nil {
			return err
		}
		if err := s.backend.SaveNode(ctx, srcSt.ref, dstCollID, dstParentID, persistSib); err != nil {
			return err
		}
	}

	s.replaceCollectionLocked(srcSt, nextSrc)
	if nextDst != nextSrc {
		s.replaceCollectionLocked(dstSt, nextDst)
	}
	s.publishLocked()
	return nil
}

// MoveCollection 调整集合在所属工作区内的排序位置。
// dstIndex 按移动前的集合列表计，小于 0 或超出长度时移到末尾。
func (s *Store) MoveCollection(collID domain.ID, dstIndex int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	st, old := s.findCollectionLocked(collID)
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	oldIndex := -1
	for i, c := range st.collections {
		if c.ID == old.ID {
			oldIndex = i
			break
		}
	}
	if oldIndex < 0 {
		return ErrNotFound
	}
	insertIdx := resolveInsertIndex(dstIndex, len(st.collections), oldIndex, true)
	if insertIdx == oldIndex {
		return nil
	}

	remaining := make([]*domain.Collection, 0, len(st.collections)-1)
	for i, c := range st.collections {
		if i != oldIndex {
			remaining = append(remaining, c.Clone())
		}
	}
	moved := old.Clone()
	nextList, changed := insertCollectionWithOrder(remaining, moved, insertIdx)

	now := time.Now()
	ctx := context.Background()
	for _, c := range changed {
		c.UpdatedAt = now
		if err := s.backend.SaveCollection(ctx, st.ref, c); err != nil {
			return err
		}
	}
	st.collections = nextList
	s.publishLocked()
	return nil
}

func (s *Store) flushSubtreeLocked(st *state, collID domain.ID, root *domain.Node) error {
	for _, nid := range nodeIDs(root) {
		key := nodeKey{ws: st.ref.ID, coll: collID, node: nid}
		n, ok := s.dirtyNode[key]
		if !ok || n == nil {
			continue
		}
		persist, err := s.preparePersistNodeLocked(st, n)
		if err != nil {
			return err
		}
		if err := s.backend.SaveNode(context.Background(), st.ref, collID, "", persist); err != nil {
			return err
		}
		delete(s.dirtyNode, key)
	}
	return nil
}

func containerItems(coll *domain.Collection, parentID domain.ID) []*domain.Node {
	if parentID == "" {
		return coll.Items
	}
	if p := domain.FindIn(coll.Items, parentID); p != nil {
		return p.Children
	}
	return nil
}

func setContainerItems(coll *domain.Collection, parentID domain.ID, items []*domain.Node) {
	if parentID == "" {
		coll.Items = items
		return
	}
	if p := domain.FindIn(coll.Items, parentID); p != nil {
		p.Children = items
	}
}

func indexOfNode(items []*domain.Node, id domain.ID) int {
	for i, n := range items {
		if n.ID == id {
			return i
		}
	}
	return -1
}

// resolveInsertIndex 将调用方传入的移动前下标 dstIndex 转换为移除源项后的插入下标。
func resolveInsertIndex(dstIndex, targetLen, oldIndex int, sameParent bool) int {
	if sameParent {
		maxLen := targetLen - 1
		if dstIndex < 0 || dstIndex >= targetLen {
			return maxLen
		}
		if dstIndex > oldIndex {
			return dstIndex - 1
		}
		return dstIndex
	}
	if dstIndex < 0 || dstIndex > targetLen {
		return targetLen
	}
	return dstIndex
}

const orderStep int64 = 1024

// insertNodeWithOrder 将 moved 插入 remaining 的 idx 位置并计算 Order。
// 若相邻间隔足够，仅修改 moved.Order；若间隔耗尽或原序非严格递增，则对同级重排步长并返回其余需要落盘的兄弟节点。
func insertNodeWithOrder(remaining []*domain.Node, moved *domain.Node, idx int) ([]*domain.Node, []*domain.Node) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(remaining) {
		idx = len(remaining)
	}
	out := make([]*domain.Node, 0, len(remaining)+1)
	out = append(out, remaining[:idx]...)
	out = append(out, moved)
	out = append(out, remaining[idx:]...)

	if order, ok := computeInsertOrder(len(remaining), func(i int) int64 { return remaining[i].Order }, idx); ok {
		moved.Order = order
		return out, nil
	}
	var rebalanced []*domain.Node
	for i, n := range out {
		want := int64(i+1) * orderStep
		if n.Order != want {
			n.Order = want
			if n != moved {
				rebalanced = append(rebalanced, n)
			}
		}
	}
	return out, rebalanced
}

func insertCollectionWithOrder(remaining []*domain.Collection, moved *domain.Collection, idx int) ([]*domain.Collection, []*domain.Collection) {
	if idx < 0 {
		idx = 0
	}
	if idx > len(remaining) {
		idx = len(remaining)
	}
	out := make([]*domain.Collection, 0, len(remaining)+1)
	out = append(out, remaining[:idx]...)
	out = append(out, moved)
	out = append(out, remaining[idx:]...)

	if order, ok := computeInsertOrder(len(remaining), func(i int) int64 { return remaining[i].Order }, idx); ok {
		moved.Order = order
		return out, []*domain.Collection{moved}
	}
	var changed []*domain.Collection
	for i, c := range out {
		want := int64(i+1) * orderStep
		if c.Order != want || c == moved {
			c.Order = want
			changed = append(changed, c)
		}
	}
	return out, changed
}

// computeInsertOrder 在保持同级严格递增的前提下尝试仅通过单次赋值得到新 Order。
func computeInsertOrder(n int, orderAt func(int) int64, idx int) (int64, bool) {
	if n == 0 {
		return orderStep, true
	}
	for i := 0; i < n; i++ {
		if orderAt(i) <= 0 {
			return 0, false
		}
		if i > 0 && orderAt(i) <= orderAt(i-1) {
			return 0, false
		}
	}
	switch {
	case idx == 0:
		first := orderAt(0)
		if first > orderStep {
			return first - orderStep, true
		}
		if first > 1 {
			return first / 2, true
		}
		return 0, false
	case idx == n:
		return orderAt(n-1) + orderStep, true
	default:
		prev, next := orderAt(idx-1), orderAt(idx)
		if next-prev > 1 {
			return prev + (next-prev)/2, true
		}
		return 0, false
	}
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

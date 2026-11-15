// Package store 是数据层门面：内存快照 + 写时复制 + 防抖落盘。
//
// 持久化由 storage.Backend 负责，当前是 storage/jsonfs 的本地文件实现；
// 将来换 SQLite 或远端只需要替换 Backend，本包和 UI 不用改。
//
// 读路径返回不可变的 Snapshot，UI 每帧直接读；写路径统一走方法，
// 先在副本上修改、持久化成功后再替换快照，失败不会留下半更新状态。
package store

import (
	"context"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/platform"
	"rddpostman/internal/storage"
	"rddpostman/internal/storage/jsonfs"
)

// 存储层错误别名，方便调用方直接判断。
var (
	ErrNotFound         = storage.ErrNotFound
	ErrExists           = storage.ErrExists
	ErrDefaultWorkspace = storage.ErrDefaultWorkspace
	ErrReadOnly         = storage.ErrReadOnly
)

const flushDelay = 300 * time.Millisecond

// Workspace 是给 UI 读的一个工作区及其数据。
type Workspace struct {
	ID                    domain.ID
	Name                  string
	Default               bool
	ReadOnly              bool
	Collections           []*domain.Collection
	Environments          []domain.Environment
	SelectedEnvironmentID domain.ID
}

// Snapshot 是给 UI 读的不可变视图，替换时整体换新，读方不需要加锁。
type Snapshot struct {
	Workspaces []*Workspace
	Globals    []domain.KV
	// Problems 是加载时跳过的损坏或高版本文件，UI 可以提示用户。
	Problems []string
}

type state struct {
	ref         storage.Ref
	ws          *domain.Workspace
	collections []*domain.Collection
	envs        []domain.Environment
	local       domain.Local
	secrets     map[domain.ID]string
	readOnly    bool
}

type nodeKey struct {
	ws   domain.ID
	coll domain.ID
	node domain.ID
}

// Store 管理所有工作区。读走 Snapshot；写一律通过方法。
type Store struct {
	backend   storage.Backend
	mu        sync.Mutex
	refs      []storage.Ref
	states    map[domain.ID]*state
	globals   domain.Globals
	problems  []string
	dirtyNode map[nodeKey]*domain.Node
	timer     *time.Timer

	snap atomic.Pointer[Snapshot]
}

// Open 在应用数据目录里打开存储。
func Open() (*Store, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return nil, err
	}
	return OpenAt(dir)
}

// OpenAt 在指定目录里打开存储，主要供测试使用。
func OpenAt(dir string) (*Store, error) {
	backend, err := jsonfs.New(dir)
	if err != nil {
		return nil, err
	}
	s := &Store{
		backend:   backend,
		states:    map[domain.ID]*state{},
		dirtyNode: map[nodeKey]*domain.Node{},
		globals:   domain.Globals{Schema: domain.SchemaGlobals, Version: domain.Version},
	}
	s.snap.Store(&Snapshot{})
	ctx := context.Background()

	refs, err := backend.List(ctx)
	if err != nil {
		return nil, err
	}
	s.refs = refs
	if g, err := backend.LoadGlobals(ctx); err == nil {
		s.globals = g
	} else {
		s.problems = append(s.problems, err.Error())
	}
	for _, ref := range refs {
		bundle, err := backend.Load(ctx, ref)
		if err != nil {
			s.problems = append(s.problems, fmt.Sprintf("打开工作区 %s: %v", ref.Path, err))
			continue
		}
		s.states[ref.ID] = &state{
			ref:         ref,
			ws:          bundle.Workspace,
			collections: bundle.Collections,
			envs:        bundle.Environments,
			local:       bundle.Local,
			secrets:     bundle.Secrets,
			readOnly:    bundle.ReadOnly,
		}
		s.problems = append(s.problems, bundle.Problems...)
	}
	s.publishLocked()
	return s, nil
}

// Snapshot 返回当前不可变视图，UI 每帧直接读。
func (s *Store) Snapshot() *Snapshot { return s.snap.Load() }

// DefaultWorkspaceID 返回默认工作区 ID。
func (s *Store) DefaultWorkspaceID() domain.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ref := range s.refs {
		if ref.Default {
			return ref.ID
		}
	}
	if len(s.refs) > 0 {
		return s.refs[0].ID
	}
	return ""
}

// Globals 返回跨工作区变量的副本。
func (s *Store) Globals() []domain.KV {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]domain.KV(nil), s.globals.Variables...)
}

// SetGlobals 保存跨工作区变量。
func (s *Store) SetGlobals(vars []domain.KV) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	g := s.globals
	g.Variables = append([]domain.KV(nil), vars...)
	if err := s.backend.SaveGlobals(context.Background(), g); err != nil {
		return err
	}
	s.globals = g
	s.publishLocked()
	return nil
}

// ---- 工作区 ----

// CreateWorkspace 在默认位置新建工作区。
func (s *Store) CreateWorkspace(name string) (*domain.Workspace, error) {
	return s.CreateWorkspaceAt(name, "")
}

// CreateWorkspaceAt 在 dir 建工作区；dir 为空时由后端在数据根目录下分配。
// dir 是用户自选目录时，整个目录就是工作区，可直接复制或用 Git 管理。
func (s *Store) CreateWorkspaceAt(name, dir string) (*domain.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bundle, err := s.backend.CreateWorkspace(context.Background(), name, dir)
	if err != nil {
		return nil, err
	}
	s.states[bundle.Ref.ID] = &state{
		ref:         bundle.Ref,
		ws:          bundle.Workspace,
		collections: bundle.Collections,
		envs:        bundle.Environments,
		local:       bundle.Local,
		secrets:     bundle.Secrets,
	}
	s.refs = append(s.refs, bundle.Ref)
	s.publishLocked()
	return bundle.Workspace, nil
}

// RenameWorkspace 修改工作区名；目录保持稳定。
func (s *Store) RenameWorkspace(id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("工作区名不能为空")
	}
	return s.UpdateWorkspace(id, func(ws *domain.Workspace) {
		ws.Name = name
	})
}

// UpdateWorkspace 修改工作区元数据（名称、描述、工作区级变量等）。
func (s *Store) UpdateWorkspace(id domain.ID, fn func(*domain.Workspace)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[id]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := st.ws.Clone()
	fn(next)
	next.UpdatedAt = time.Now()
	if err := s.backend.UpdateWorkspace(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.ws = next
	s.publishLocked()
	return nil
}

// RemoveWorkspace 取消注册，磁盘上的工作区目录保持原样。
func (s *Store) RemoveWorkspace(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[id]
	if st == nil {
		return ErrNotFound
	}
	if st.ref.Default {
		return ErrDefaultWorkspace
	}
	if err := s.backend.RemoveWorkspace(context.Background(), st.ref); err != nil {
		return err
	}
	delete(s.states, id)
	out := s.refs[:0]
	for _, ref := range s.refs {
		if ref.ID != id {
			out = append(out, ref)
		}
	}
	s.refs = out
	for key := range s.dirtyNode {
		if key.ws == id {
			delete(s.dirtyNode, key)
		}
	}
	s.publishLocked()
	return nil
}

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
	out := st.collections[:0]
	for _, c := range st.collections {
		if c.ID != old.ID {
			out = append(out, c)
		}
	}
	st.collections = out
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

// ---- 环境 ----

// CreateEnvironment 新建一个环境。
func (s *Store) CreateEnvironment(wsID domain.ID, name string) (domain.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Environment{}, errors.New("环境名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return domain.Environment{}, ErrNotFound
	}
	if st.readOnly {
		return domain.Environment{}, ErrReadOnly
	}
	now := time.Now()
	env := domain.Environment{
		Schema:    domain.SchemaEnvironment,
		Version:   domain.Version,
		ID:        domain.NewID(),
		Name:      name,
		Vars:      []domain.KV{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.backend.SaveEnvironment(context.Background(), st.ref, env); err != nil {
		return domain.Environment{}, err
	}
	st.envs = append(st.envs, env)
	sortEnvironments(st.envs)
	// 第一个环境自动成为当前环境，否则用户创建完还得再点一次。
	if st.local.SelectedEnvironmentID == "" && st.ws.DefaultEnvironmentID == "" {
		next := st.local.Clone()
		next.SelectedEnvironmentID = env.ID
		if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
			return domain.Environment{}, err
		}
		st.local = next
	}
	s.publishLocked()
	return env, nil
}

// SaveEnvironment 新建或更新环境；secret 值单独存 secrets.json。
func (s *Store) SaveEnvironment(wsID domain.ID, env domain.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	persist := env.Clone()
	if err := s.splitEnvSecretsLocked(st, &persist); err != nil {
		return err
	}
	if err := s.backend.SaveEnvironment(context.Background(), st.ref, persist); err != nil {
		return err
	}
	replaced := false
	for i := range st.envs {
		if st.envs[i].ID == env.ID {
			st.envs[i] = env
			replaced = true
			break
		}
	}
	if !replaced {
		st.envs = append(st.envs, env)
		sortEnvironments(st.envs)
	}
	s.publishLocked()
	return nil
}

// DeleteEnvironment 删除环境。
func (s *Store) DeleteEnvironment(wsID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	if err := s.backend.DeleteEnvironment(context.Background(), st.ref, envID); err != nil {
		return err
	}
	out := st.envs[:0]
	for _, e := range st.envs {
		if e.ID != envID {
			out = append(out, e)
		}
	}
	st.envs = out
	if st.local.SelectedEnvironmentID == envID {
		st.local.SelectedEnvironmentID = ""
		if err := s.backend.SaveLocal(context.Background(), st.ref, st.local); err != nil {
			return err
		}
	}
	s.publishLocked()
	return nil
}

// SelectEnvironment 切换当前环境；空 ID 表示不使用环境。
func (s *Store) SelectEnvironment(wsID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if envID != "" && !hasEnvironment(st.envs, envID) {
		return ErrNotFound
	}
	next := st.local.Clone()
	next.SelectedEnvironmentID = envID
	if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.local = next
	s.publishLocked()
	return nil
}

// SetCurrentValue 设置环境变量的本机覆盖值（不同步）。
func (s *Store) SetCurrentValue(wsID, varID domain.ID, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	next := st.local.Clone()
	if next.CurrentValues == nil {
		next.CurrentValues = map[domain.ID]string{}
	}
	next.CurrentValues[varID] = value
	if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.local = next
	s.publishLocked()
	return nil
}

// ---- 变量与请求准备 ----

// Variables 合并一个请求可见的全部变量，优先级从低到高：
// 全局 < 工作区 < 当前环境 < 集合 < 目录（越深越优先）< 请求。
func (s *Store) Variables(wsID, collID, nodeID domain.ID) map[string]domain.Variable {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return nil
	}
	return s.variablesLocked(st, collID, nodeID)
}

// PrepareRequest 返回替换变量后的请求与最终生效的认证配置。
func (s *Store) PrepareRequest(wsID, collID, nodeID domain.ID) (domain.Request, domain.Auth, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.states[wsID]
	if st == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}
	var coll *domain.Collection
	for _, c := range st.collections {
		if c.ID == collID {
			coll = c
			break
		}
	}
	if coll == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}
	n := domain.FindIn(coll.Items, nodeID)
	if n == nil || n.Request == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}

	vars := s.variablesLocked(st, collID, nodeID)
	req, missing := domain.Prepare(*n.Request, vars)
	preparedAuth, authMissing := domain.PrepareAuth(s.authChainLocked(coll, nodeID), vars)
	return req, preparedAuth, mergeMissing(missing, authMissing), nil
}

func (s *Store) variablesLocked(st *state, collID, nodeID domain.ID) map[string]domain.Variable {
	layers := []domain.Layer{
		{Source: domain.VarGlobal, Vars: s.globals.Variables},
		{Source: domain.VarWorkspace, Vars: st.ws.Variables},
	}
	if env := selectedEnvironment(st); env != nil {
		layers = append(layers, domain.Layer{
			Source:  domain.VarEnvironment,
			Vars:    env.Vars,
			Current: st.local.CurrentValues,
			Secrets: st.secrets,
		})
	}
	var coll *domain.Collection
	for _, c := range st.collections {
		if c.ID == collID {
			coll = c
			break
		}
	}
	if coll == nil {
		return domain.Merge(layers)
	}
	layers = append(layers, domain.Layer{Source: domain.VarCollection, Vars: coll.Variables})
	if nodeID != "" {
		for _, folder := range domain.AncestorFolders(coll.Items, nodeID) {
			layers = append(layers, domain.Layer{Source: domain.VarFolder, Vars: folder.Variables})
		}
		if n := domain.FindIn(coll.Items, nodeID); n != nil {
			layers = append(layers, domain.Layer{Source: domain.VarRequest, Vars: n.Variables})
		}
	}
	return domain.Merge(layers)
}

func (s *Store) authChainLocked(coll *domain.Collection, nodeID domain.ID) domain.Auth {
	chain := []domain.Auth{coll.Auth}
	for _, folder := range domain.AncestorFolders(coll.Items, nodeID) {
		chain = append(chain, folder.Auth)
	}
	if n := domain.FindIn(coll.Items, nodeID); n != nil {
		chain = append(chain, n.Auth)
	}
	return domain.ResolveAuth(chain)
}

// ---- 落盘 ----

// Flush 立即把待写内容落盘。
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

// Close 落盘并停止防抖定时器。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	return s.flushLocked()
}

func (s *Store) scheduleLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushDelay, func() {
		if err := s.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "store: 保存失败: %v\n", err)
		}
	})
}

func (s *Store) flushLocked() error {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	keys := make([]nodeKey, 0, len(s.dirtyNode))
	for key := range s.dirtyNode {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ws != keys[j].ws {
			return keys[i].ws < keys[j].ws
		}
		if keys[i].coll != keys[j].coll {
			return keys[i].coll < keys[j].coll
		}
		return keys[i].node < keys[j].node
	})

	var firstErr error
	for _, key := range keys {
		n := s.dirtyNode[key]
		st := s.states[key.ws]
		if st == nil || n == nil || !nodeExists(st, key.coll, key.node) {
			delete(s.dirtyNode, key)
			continue
		}
		persist, err := s.preparePersistNodeLocked(st, n)
		if err == nil {
			err = s.backend.SaveNode(context.Background(), st.ref, key.coll, "", persist)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		delete(s.dirtyNode, key)
	}
	return firstErr
}

// preparePersistNodeLocked 生成用于落盘的节点副本：secret 值剥离到 secrets.json。
func (s *Store) preparePersistNodeLocked(st *state, n *domain.Node) (*domain.Node, error) {
	out := n.Clone()
	if err := s.splitSecretsLocked(st, out); err != nil {
		return nil, err
	}
	return out, nil
}

// splitSecretsLocked 把 secret KV 的值移进工作区的 secrets 表并清空副本里的值。
// 目录变量、请求参数、请求头、认证参数都走同一套逻辑。
func (s *Store) splitSecretsLocked(st *state, n *domain.Node) error {
	found := false
	found = s.stashSecretsLocked(st, n.Variables) || found
	found = s.stashSecretsLocked(st, n.Auth.Params) || found
	if n.Request != nil {
		found = s.stashSecretsLocked(st, n.Request.PathVariables) || found
		found = s.stashSecretsLocked(st, n.Request.Query) || found
		found = s.stashSecretsLocked(st, n.Request.Headers) || found
		found = s.stashSecretsLocked(st, n.Request.Cookies) || found
		found = s.stashSecretsLocked(st, n.Request.Body.Fields) || found
	}
	if !found {
		return nil
	}
	return s.backend.SaveSecrets(context.Background(), st.ref, st.secrets)
}

// splitSecretsLocked 的环境版本。
func (s *Store) splitEnvSecretsLocked(st *state, env *domain.Environment) error {
	if !s.stashSecretsLocked(st, env.Vars) {
		return nil
	}
	return s.backend.SaveSecrets(context.Background(), st.ref, st.secrets)
}

// stashSecretsLocked 把 secret 值记进内存 secrets 表，并清空待落盘副本里的值。
// 空值表示用户清除了 secret，需要把旧值从表里删掉。
func (s *Store) stashSecretsLocked(st *state, kvs []domain.KV) bool {
	found := false
	for i := range kvs {
		if !kvs[i].Secret || kvs[i].ID == "" {
			continue
		}
		found = true
		if kvs[i].Value == "" {
			delete(st.secrets, kvs[i].ID)
		} else {
			if st.secrets == nil {
				st.secrets = map[domain.ID]string{}
			}
			st.secrets[kvs[i].ID] = kvs[i].Value
		}
		kvs[i].Value = ""
	}
	return found
}

// ---- 内部辅助 ----

func (s *Store) findCollectionLocked(id domain.ID) (*state, *domain.Collection) {
	for _, st := range s.states {
		for _, c := range st.collections {
			if c.ID == id {
				return st, c
			}
		}
	}
	return nil, nil
}

func (s *Store) replaceCollectionLocked(st *state, next *domain.Collection) {
	for i, c := range st.collections {
		if c.ID == next.ID {
			st.collections[i] = next
			return
		}
	}
	st.collections = append(st.collections, next)
	sortCollections(st.collections)
}

func (s *Store) publishLocked() {
	snap := &Snapshot{
		Globals:  append([]domain.KV(nil), s.globals.Variables...),
		Problems: append([]string(nil), s.problems...),
	}
	for _, ref := range s.refs {
		st := s.states[ref.ID]
		if st == nil {
			continue
		}
		w := &Workspace{
			ID:           st.ws.ID,
			Name:         st.ws.Name,
			Default:      ref.Default,
			ReadOnly:     st.readOnly,
			Collections:  append([]*domain.Collection(nil), st.collections...),
			Environments: append([]domain.Environment(nil), st.envs...),
		}
		w.SelectedEnvironmentID = st.local.SelectedEnvironmentID
		if w.SelectedEnvironmentID == "" {
			w.SelectedEnvironmentID = st.ws.DefaultEnvironmentID
		}
		snap.Workspaces = append(snap.Workspaces, w)
	}
	s.snap.Store(snap)
}

func selectedEnvironment(st *state) *domain.Environment {
	id := st.local.SelectedEnvironmentID
	if id == "" {
		id = st.ws.DefaultEnvironmentID
	}
	if id == "" {
		return nil
	}
	for i := range st.envs {
		if st.envs[i].ID == id {
			return &st.envs[i]
		}
	}
	return nil
}

func hasEnvironment(envs []domain.Environment, id domain.ID) bool {
	for _, e := range envs {
		if e.ID == id {
			return true
		}
	}
	return false
}

func sortCollections(list []*domain.Collection) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Order != list[j].Order {
			return list[i].Order < list[j].Order
		}
		return list[i].Name < list[j].Name
	})
}

func sortEnvironments(list []domain.Environment) {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
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

func mergeMissing(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string(nil), a...), b...) {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

// NewID 生成稳定的随机 ID。
func NewID() domain.ID { return domain.NewID() }

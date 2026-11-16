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
	"sort"
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
	ErrNotFound           = storage.ErrNotFound
	ErrExists             = storage.ErrExists
	ErrDefaultWorkspace   = storage.ErrDefaultWorkspace
	ErrReadOnly           = storage.ErrReadOnly
	ErrDefaultEnvironment = errors.New("store: 默认环境不可删除")
)

// Workspace 是给 UI 读的一个工作区及其数据。
type Workspace struct {
	ID                     domain.ID
	Name                   string
	Default                bool
	ReadOnly               bool
	Collections            []*domain.Collection
	Environments           []domain.Environment
	DefaultEnvironmentID   domain.ID
	SelectedEnvironmentID  domain.ID
	CollectionEnvironments map[domain.ID]domain.ID
}

// Snapshot 是给 UI 读的不可变视图，替换时整体换新，读方不需要加锁。
type Snapshot struct {
	ActiveWorkspaceID domain.ID
	Workspaces        []*Workspace
	Globals           []domain.KV
	// Problems 是加载时跳过的损坏或高版本文件，UI 可以提示用户。
	Problems []string
}

// ActiveWorkspace 返回当前活跃工作区。若未设置或对应工作区已被删除，回退到默认工作区或第一个。
func (s *Snapshot) ActiveWorkspace() *Workspace {
	if s == nil {
		return nil
	}
	if s.ActiveWorkspaceID != "" {
		for _, w := range s.Workspaces {
			if w.ID == s.ActiveWorkspaceID {
				return w
			}
		}
	}
	for _, w := range s.Workspaces {
		if w.Default {
			return w
		}
	}
	if len(s.Workspaces) > 0 {
		return s.Workspaces[0]
	}
	return nil
}

// Workspace 按 ID 查找工作区。
func (s *Snapshot) Workspace(id domain.ID) *Workspace {
	if s == nil {
		return nil
	}
	for _, w := range s.Workspaces {
		if w.ID == id {
			return w
		}
	}
	return nil
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
	collIndex map[domain.ID]*state // 集合 ID → 所属工作区，O(1) 查找
	globals   domain.Globals
	problems  []string
	dirtyNode map[nodeKey]*domain.Node
	timer     *time.Timer
	activeWS  domain.ID

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

// New 使用指定的 Backend 打开存储，方便测试注入 mock 或切换存储实现。
func New(backend storage.Backend) (*Store, error) {
	s := &Store{
		backend:   backend,
		states:    map[domain.ID]*state{},
		collIndex: map[domain.ID]*state{},
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
		st := &state{
			ref:         ref,
			ws:          bundle.Workspace,
			collections: bundle.Collections,
			envs:        bundle.Environments,
			local:       bundle.Local,
			secrets:     bundle.Secrets,
			readOnly:    bundle.ReadOnly,
		}
		s.states[ref.ID] = st
		for _, c := range st.collections {
			s.collIndex[c.ID] = st
		}
		s.problems = append(s.problems, bundle.Problems...)
		if !st.readOnly {
			_ = s.ensureEnvironmentsLocked(ctx, st)
		}
	}
	for _, ref := range refs {
		if ref.Default {
			s.activeWS = ref.ID
			break
		}
	}
	if s.activeWS == "" && len(refs) > 0 {
		s.activeWS = refs[0].ID
	}
	s.publishLocked()
	return s, nil
}

// OpenAt 在指定目录里打开存储，主要供测试使用。
func OpenAt(dir string) (*Store, error) {
	backend, err := jsonfs.New(dir)
	if err != nil {
		return nil, err
	}
	return New(backend)
}

// Snapshot 返回当前不可变视图，UI 每帧直接读。
func (s *Store) Snapshot() *Snapshot { return s.snap.Load() }

// ActiveWorkspaceID 返回当前活跃工作区 ID。
func (s *Store) ActiveWorkspaceID() domain.ID {
	if snap := s.snap.Load(); snap != nil {
		return snap.ActiveWorkspaceID
	}
	return ""
}

// ActiveWorkspace 返回当前活跃工作区的快照对象。
func (s *Store) ActiveWorkspace() *Workspace {
	if snap := s.snap.Load(); snap != nil {
		return snap.ActiveWorkspace()
	}
	return nil
}

// SetActiveWorkspace 切换当前活跃工作区并立即发布新快照。
func (s *Store) SetActiveWorkspace(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.states[id] == nil {
		return fmt.Errorf("store: 工作区不存在 %s", id)
	}
	if s.activeWS == id {
		return nil
	}
	s.activeWS = id
	s.publishLocked()
	return nil
}

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
	now := time.Now()
	for i := range vars {
		if vars[i].ID == "" {
			vars[i].ID = domain.NewID()
		}
		if vars[i].CreatedAt.IsZero() {
			vars[i].CreatedAt = now
		}
		vars[i].UpdatedAt = now
	}
	g := s.globals
	if g.CreatedAt.IsZero() {
		g.CreatedAt = now
	}
	g.UpdatedAt = now
	g.Variables = append([]domain.KV(nil), vars...)
	if err := s.backend.SaveGlobals(context.Background(), g); err != nil {
		return err
	}
	s.globals = g
	s.publishLocked()
	return nil
}

// ---- 内部辅助 ----

func (s *Store) findCollectionLocked(id domain.ID) (*state, *domain.Collection) {
	st := s.collIndex[id]
	if st == nil {
		return nil, nil
	}
	for _, c := range st.collections {
		if c.ID == id {
			return st, c
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
	s.collIndex[next.ID] = st
	sortCollections(st.collections)
}

func (s *Store) publishLocked() {
	// 如果当前活跃工作区无效或已被移除，回退到默认工作区或首个可用工作区。
	activeValid := false
	for _, ref := range s.refs {
		if ref.ID == s.activeWS {
			activeValid = true
			break
		}
	}
	if !activeValid {
		s.activeWS = ""
		for _, ref := range s.refs {
			if ref.Default {
				s.activeWS = ref.ID
				break
			}
		}
		if s.activeWS == "" && len(s.refs) > 0 {
			s.activeWS = s.refs[0].ID
		}
	}

	snap := &Snapshot{
		ActiveWorkspaceID: s.activeWS,
		Globals:           append([]domain.KV(nil), s.globals.Variables...),
		Problems:          append([]string(nil), s.problems...),
	}
	for _, ref := range s.refs {
		st := s.states[ref.ID]
		if st == nil {
			continue
		}
		w := &Workspace{
			ID:                    st.ws.ID,
			Name:                  st.ws.Name,
			Default:               ref.Default,
			ReadOnly:              st.readOnly,
			Collections:           append([]*domain.Collection(nil), st.collections...),
			Environments:          append([]domain.Environment(nil), st.envs...),
			DefaultEnvironmentID:  st.ws.DefaultEnvironmentID,
			SelectedEnvironmentID: st.local.SelectedEnvironmentID,
		}
		if w.SelectedEnvironmentID == "" {
			w.SelectedEnvironmentID = st.ws.DefaultEnvironmentID
		}
		if len(st.local.CollectionEnvironments) > 0 {
			w.CollectionEnvironments = make(map[domain.ID]domain.ID, len(st.local.CollectionEnvironments))
			for k, v := range st.local.CollectionEnvironments {
				w.CollectionEnvironments[k] = v
			}
		}
		snap.Workspaces = append(snap.Workspaces, w)
	}
	s.snap.Store(snap)
}

func sortCollections(list []*domain.Collection) {
	sort.Slice(list, func(i, j int) bool {
		if list[i].Order != list[j].Order {
			return list[i].Order < list[j].Order
		}
		return list[i].Name < list[j].Name
	})
}

// NewID 生成稳定的随机 ID。
func NewID() domain.ID { return domain.NewID() }

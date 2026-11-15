// Package store 负责工作空间、集合文件的注册表与本地持久化。
//
// 层级：工作空间 → 集合 → 目录/请求。
// 集合本身是用户选定目录里的 <名称>.collection.json；
// 应用数据目录只放 workspaces.json（工作空间与集合注册表），
// 以及后续的环境、历史、密钥。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/platform"
)

// SchemaVersion 是注册表与集合文件的格式版本，升级时据此迁移。
const SchemaVersion = 2

const (
	collectionExt      = ".collection.json"
	registryFile       = "workspaces.json"
	legacyRegistryFile = "collections.json"
	defaultSpaceName   = "默认工作区"
	flushDelay         = 300 * time.Millisecond
)

var (
	// ErrNotFound 表示工作空间、集合或节点不存在，或者集合文件已丢失。
	ErrNotFound = errors.New("store: 找不到工作空间、集合或节点")
	// ErrExists 表示目标位置已有同名集合。
	ErrExists = errors.New("store: 同名集合已存在")
	// ErrDefaultWorkspace 表示试图删除系统默认工作空间。
	ErrDefaultWorkspace = errors.New("store: 默认工作区不可删除")
)

// Ref 是注册表里的一条集合记录。
type Ref struct {
	ID          domain.ID `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	WorkspaceID domain.ID `json:"workspaceID"`
}

type registry struct {
	Version     int                `json:"version"`
	Workspaces  []domain.Workspace `json:"workspaces"`
	Collections []Ref              `json:"collections"`
}

// Workspace 是给 UI 读的一个工作空间及其集合。
type Workspace struct {
	ID          domain.ID
	Name        string
	Default     bool
	Collections []*domain.Collection
}

// Snapshot 是给 UI 读的不可变视图，替换时整体换新，读方不需要加锁。
type Snapshot struct {
	Workspaces []*Workspace
	// Missing 是文件已被移动或删除的集合 ID，UI 灰显并提示重新定位。
	Missing []domain.ID
}

// Store 管理注册表与所有集合文件。
// 读走 Snapshot；写一律通过方法，内部用写时复制更新快照。
type Store struct {
	dir     string
	regPath string

	mu       sync.Mutex
	reg      registry
	byID     map[domain.ID]*domain.Collection
	missing  map[domain.ID]bool
	dirty    map[domain.ID]bool
	regDirty bool
	timer    *time.Timer

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
	s := &Store{
		dir:     dir,
		regPath: filepath.Join(dir, registryFile),
		byID:    map[domain.ID]*domain.Collection{},
		missing: map[domain.ID]bool{},
		dirty:   map[domain.ID]bool{},
	}
	s.snap.Store(&Snapshot{})
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) load() error {
	data, err := os.ReadFile(s.regPath)
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &s.reg); err != nil {
			return fmt.Errorf("读取工作空间注册表: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		// 兼容旧版只有集合的注册表。
		if legacy, lerr := os.ReadFile(filepath.Join(s.dir, legacyRegistryFile)); lerr == nil {
			if err := json.Unmarshal(legacy, &s.reg); err != nil {
				return fmt.Errorf("读取旧版集合注册表: %w", err)
			}
			s.regDirty = true
		}
	default:
		return err
	}

	s.reg.Version = SchemaVersion
	defaultID := s.ensureDefaultWorkspaceLocked()
	// 旧数据或异常数据里没有归属工作空间的集合，统一挂到默认工作区。
	for i := range s.reg.Collections {
		if s.reg.Collections[i].WorkspaceID == "" {
			s.reg.Collections[i].WorkspaceID = defaultID
			s.regDirty = true
		}
	}

	for _, ref := range s.reg.Collections {
		coll, err := readCollection(ref.Path)
		if err != nil {
			s.missing[ref.ID] = true
			continue
		}
		s.byID[ref.ID] = coll
	}
	s.publishLocked()

	if s.regDirty {
		if err := s.flushLocked(); err != nil {
			fmt.Fprintf(os.Stderr, "store: 写入注册表失败: %v\n", err)
		}
	}
	return nil
}

// Snapshot 返回当前不可变视图，UI 每帧直接读它。
func (s *Store) Snapshot() *Snapshot { return s.snap.Load() }

// DefaultWorkspaceID 返回默认工作区 ID，新建集合时没有指定归属时使用。
func (s *Store) DefaultWorkspaceID() domain.ID {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.ensureDefaultWorkspaceLocked()
}

// CreateWorkspace 新建一个自定义工作空间。
func (s *Store) CreateWorkspace(name string) (*domain.Workspace, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("store: 工作空间名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	ws := domain.Workspace{ID: NewID(), Name: name}
	s.reg.Workspaces = append(s.reg.Workspaces, ws)
	s.regDirty = true
	s.publishLocked()
	// 结构变更立刻落盘，防抖只留给树和请求内容的编辑。
	if err := s.flushLocked(); err != nil {
		return nil, err
	}
	return &ws, nil
}

// RenameWorkspace 修改工作空间名，默认工作区也可以改名。
func (s *Store) RenameWorkspace(id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("store: 工作空间名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.reg.Workspaces {
		if s.reg.Workspaces[i].ID != id {
			continue
		}
		s.reg.Workspaces[i].Name = name
		s.regDirty = true
		s.publishLocked()
		return s.flushLocked()
	}
	return ErrNotFound
}

// RemoveWorkspace 删除自定义工作空间；默认工作区不可删除。
// 其下的集合会移到默认工作区，文件不动。
func (s *Store) RemoveWorkspace(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	idx := -1
	for i, w := range s.reg.Workspaces {
		if w.ID != id {
			continue
		}
		if w.Default {
			return ErrDefaultWorkspace
		}
		idx = i
		break
	}
	if idx < 0 {
		return ErrNotFound
	}

	defaultID := s.ensureDefaultWorkspaceLocked()
	s.reg.Workspaces = append(s.reg.Workspaces[:idx], s.reg.Workspaces[idx+1:]...)
	for i := range s.reg.Collections {
		if s.reg.Collections[i].WorkspaceID == id {
			s.reg.Collections[i].WorkspaceID = defaultID
		}
	}
	s.regDirty = true
	s.publishLocked()
	return s.flushLocked()
}

// CreateCollection 在 workspaceID 下注册集合，文件写到 dir 里的 <name>.collection.json。
// workspaceID 为空时放到默认工作区。
func (s *Store) CreateCollection(workspaceID domain.ID, name, dir string) (*domain.Collection, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("store: 集合名不能为空")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("store: 目标目录不可用: %w", err)
	}
	if !info.IsDir() {
		return nil, errors.New("store: 目标不是目录")
	}

	path := filepath.Join(dir, fileName(name))
	if _, err := os.Stat(path); err == nil {
		return nil, ErrExists
	}

	coll := &domain.Collection{
		Version: SchemaVersion,
		ID:      NewID(),
		Name:    name,
		Items:   []*domain.Node{},
	}
	if err := writeCollection(path, coll); err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if workspaceID == "" {
		workspaceID = s.ensureDefaultWorkspaceLocked()
	} else if s.workspaceIndexLocked(workspaceID) < 0 {
		_ = os.Remove(path)
		return nil, ErrNotFound
	}
	s.reg.Collections = append(s.reg.Collections, Ref{
		ID:          coll.ID,
		Name:        coll.Name,
		Path:        path,
		WorkspaceID: workspaceID,
	})
	s.byID[coll.ID] = coll
	s.regDirty = true
	s.publishLocked()
	if err := s.flushLocked(); err != nil {
		return nil, err
	}
	return coll, nil
}

// OpenCollection 把一个已有的集合文件加入注册表；已注册时直接返回。
// 未指定归属时放进默认工作区。
func (s *Store) OpenCollection(path string) (*domain.Collection, error) {
	coll, err := readCollection(path)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	for _, ref := range s.reg.Collections {
		if ref.Path == path {
			return s.byID[ref.ID], nil
		}
		if ref.ID == coll.ID {
			return nil, fmt.Errorf("store: 集合 %q 已从其他路径打开", ref.Name)
		}
	}

	s.reg.Collections = append(s.reg.Collections, Ref{
		ID:          coll.ID,
		Name:        coll.Name,
		Path:        path,
		WorkspaceID: s.ensureDefaultWorkspaceLocked(),
	})
	s.byID[coll.ID] = coll
	delete(s.missing, coll.ID)
	s.regDirty = true
	s.publishLocked()
	if err := s.flushLocked(); err != nil {
		return nil, err
	}
	return coll, nil
}

// RenameCollection 改名；如果文件在用户目录里，文件名一起改。
func (s *Store) RenameCollection(id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("store: 集合名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.refIndexLocked(id)
	if idx < 0 {
		return ErrNotFound
	}
	ref := s.reg.Collections[idx]

	if coll := s.byID[id]; coll != nil {
		next := coll.Clone()
		next.Name = name
		s.byID[id] = next
		s.dirty[id] = true
	}

	newPath := filepath.Join(filepath.Dir(ref.Path), fileName(name))
	if newPath != ref.Path {
		if _, err := os.Stat(newPath); err == nil {
			return ErrExists
		}
		if err := os.Rename(ref.Path, newPath); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		ref.Path = newPath
	}
	ref.Name = name
	s.reg.Collections[idx] = ref
	s.regDirty = true
	s.publishLocked()
	// 改文件名的同时立刻写内容，避免文件名和文件里的名字短时间不一致。
	return s.flushLocked()
}

// RemoveCollection 从注册表移除集合，deleteFile 为 true 时同时删除文件。
func (s *Store) RemoveCollection(id domain.ID, deleteFile bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	idx := s.refIndexLocked(id)
	if idx < 0 {
		return ErrNotFound
	}
	path := s.reg.Collections[idx].Path
	s.reg.Collections = append(s.reg.Collections[:idx], s.reg.Collections[idx+1:]...)
	delete(s.byID, id)
	delete(s.missing, id)
	delete(s.dirty, id)
	s.regDirty = true
	s.publishLocked()
	if err := s.flushLocked(); err != nil {
		return err
	}

	if deleteFile {
		if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
			return err
		}
	}
	return nil
}

// AddNode 在 parentID 下追加节点；parentID 为空时加到集合根层。
func (s *Store) AddNode(collID, parentID domain.ID, n *domain.Node) error {
	if n == nil {
		return errors.New("store: 节点为空")
	}
	return s.mutate(collID, func(c *domain.Collection) error {
		if parentID == "" {
			c.Items = append(c.Items, n)
			return nil
		}
		parent := domain.FindIn(c.Items, parentID)
		if parent == nil {
			return ErrNotFound
		}
		if parent.Kind != domain.KindFolder {
			return errors.New("store: 只能在目录下创建")
		}
		parent.Children = append(parent.Children, n)
		return nil
	})
}

// RenameNode 修改节点名称。
func (s *Store) RenameNode(collID, id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("store: 名称不能为空")
	}
	return s.mutate(collID, func(c *domain.Collection) error {
		n := domain.FindIn(c.Items, id)
		if n == nil {
			return ErrNotFound
		}
		n.Name = name
		return nil
	})
}

// DeleteNode 删除节点及其子树。
func (s *Store) DeleteNode(collID, id domain.ID) error {
	return s.mutate(collID, func(c *domain.Collection) error {
		items, ok := domain.RemoveChild(c.Items, id)
		if !ok {
			return ErrNotFound
		}
		c.Items = items
		return nil
	})
}

// UpdateRequest 修改请求配置，例如保存 URL、参数或请求头。
func (s *Store) UpdateRequest(collID, id domain.ID, fn func(*domain.Request)) error {
	return s.mutate(collID, func(c *domain.Collection) error {
		n := domain.FindIn(c.Items, id)
		if n == nil || n.Request == nil {
			return ErrNotFound
		}
		fn(n.Request)
		return nil
	})
}

// Flush 立即把待写内容落盘。
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

// Close 落盘并停止防抖定时器。
func (s *Store) Close() error {
	return s.Flush()
}

func (s *Store) mutate(id domain.ID, fn func(*domain.Collection) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	cur := s.byID[id]
	if cur == nil {
		return ErrNotFound
	}
	next := cur.Clone()
	if err := fn(next); err != nil {
		return err
	}
	s.byID[id] = next
	s.dirty[id] = true
	s.publishLocked()
	s.scheduleLocked()
	return nil
}

func (s *Store) publishLocked() {
	snap := &Snapshot{Workspaces: make([]*Workspace, 0, len(s.reg.Workspaces))}
	index := make(map[domain.ID]*Workspace, len(s.reg.Workspaces))
	var defaultWS *Workspace
	for _, w := range s.reg.Workspaces {
		v := &Workspace{ID: w.ID, Name: w.Name, Default: w.Default}
		snap.Workspaces = append(snap.Workspaces, v)
		index[w.ID] = v
		if w.Default {
			defaultWS = v
		}
	}
	for _, ref := range s.reg.Collections {
		if s.missing[ref.ID] {
			snap.Missing = append(snap.Missing, ref.ID)
			continue
		}
		coll := s.byID[ref.ID]
		if coll == nil {
			continue
		}
		ws := index[ref.WorkspaceID]
		if ws == nil {
			ws = defaultWS
		}
		if ws == nil {
			continue
		}
		ws.Collections = append(ws.Collections, coll)
	}
	s.snap.Store(snap)
}

// ensureDefaultWorkspaceLocked 保证默认工作区存在，返回它的 ID。
func (s *Store) ensureDefaultWorkspaceLocked() domain.ID {
	for _, w := range s.reg.Workspaces {
		if w.Default {
			return w.ID
		}
	}
	w := domain.Workspace{ID: NewID(), Name: defaultSpaceName, Default: true}
	s.reg.Workspaces = append([]domain.Workspace{w}, s.reg.Workspaces...)
	s.regDirty = true
	return w.ID
}

func (s *Store) scheduleLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushDelay, func() {
		if err := s.Flush(); err != nil {
			// 落盘失败不 panic，内存状态仍然可用；下次修改会再试。
			fmt.Fprintf(os.Stderr, "store: 保存失败: %v\n", err)
		}
	})
}

func (s *Store) flushLocked() error {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}

	ids := make([]domain.ID, 0, len(s.dirty))
	for id := range s.dirty {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })

	var firstErr error
	for _, id := range ids {
		idx := s.refIndexLocked(id)
		if idx < 0 {
			delete(s.dirty, id)
			continue
		}
		if err := writeCollection(s.reg.Collections[idx].Path, s.byID[id]); err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		delete(s.dirty, id)
	}
	if s.regDirty {
		if err := writeJSON(s.regPath, s.reg); err != nil {
			if firstErr == nil {
				firstErr = err
			}
		} else {
			s.regDirty = false
		}
	}
	return firstErr
}

func (s *Store) refIndexLocked(id domain.ID) int {
	for i, ref := range s.reg.Collections {
		if ref.ID == id {
			return i
		}
	}
	return -1
}

func (s *Store) workspaceIndexLocked(id domain.ID) int {
	for i, w := range s.reg.Workspaces {
		if w.ID == id {
			return i
		}
	}
	return -1
}

// NewID 生成稳定的随机 ID，供新建集合和节点使用。
func NewID() domain.ID {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		panic(fmt.Sprintf("store: 生成 ID 失败: %v", err))
	}
	return domain.ID(hex.EncodeToString(b))
}

// fileName 把集合名转成文件名；保留中文，替换路径分隔符。
func fileName(name string) string {
	repl := strings.NewReplacer("/", "-", "\\", "-", ":", "-", "\x00", "")
	name = strings.TrimSpace(repl.Replace(name))
	if name == "" {
		name = "未命名集合"
	}
	return name + collectionExt
}

func readCollection(path string) (*domain.Collection, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var c domain.Collection
	if err := json.Unmarshal(data, &c); err != nil {
		return nil, fmt.Errorf("解析集合 %s: %w", path, err)
	}
	if c.Items == nil {
		c.Items = []*domain.Node{}
	}
	if c.Version == 0 {
		c.Version = SchemaVersion
	}
	return &c, nil
}

func writeCollection(path string, c *domain.Collection) error {
	if c == nil {
		return errors.New("store: 集合为空")
	}
	return writeJSON(path, c)
}

func writeJSON(path string, v any) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), 0o644)
}

// writeAtomic 先写同目录临时文件再改名，避免中途退出留下半个 JSON。
func writeAtomic(path string, data []byte, perm os.FileMode) error {
	tmp, err := os.CreateTemp(filepath.Dir(path), ".rddpostman-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

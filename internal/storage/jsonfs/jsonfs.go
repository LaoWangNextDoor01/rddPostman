// Package jsonfs 是 storage.Backend 的本地文件实现。
//
// 一个工作区就是一个真实目录，集合和目录直接映射成子目录，每个请求是一个
// 独立文件。用户可以直接用 Finder、Git 或 iCloud 管理这些数据：
//
//	<dataRoot>/
//	├── workspaces.json                 工作区注册表
//	├── globals.json                    跨工作区变量
//	└── workspaces/default/
//	    ├── workspace.json
//	    ├── local.json                  本机状态（不同步）
//	    ├── secrets.json                secret 真实值（0600）
//	    ├── environments/dev.environment.json
//	    └── collections/开发者平台 API/
//	        ├── collection.json
//	        ├── Users/
//	        │   ├── folder.json
//	        │   └── 获取用户.request.json
//	        └── 登录.request.json
package jsonfs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

const (
	registryFile      = "workspaces.json"
	legacyRegistry    = "collections.json"
	globalsFile       = "globals.json"
	workspaceFile     = "workspace.json"
	localFile         = "local.json"
	secretsFile       = "secrets.json"
	collectionsDir    = "collections"
	environmentsDir   = "environments"
	collectionFile    = "collection.json"
	folderFile        = "folder.json"
	requestSuffix     = ".request.json"
	environmentSuffix = ".environment.json"
	workspacesDir     = "workspaces"
	defaultDirName    = "default"
	defaultName       = "默认工作区"
	orderStep         = 1024
)

// FS 是本地 JSON 文件后端。
type FS struct {
	root string

	mu    sync.Mutex
	refs  []storage.Ref
	index map[domain.ID]*wsIndex
}

// wsIndex 记录一个工作区里节点 ID 与磁盘路径的对应关系。
// 节点 ID 不随改名变化，路径在改名后由这里更新。
type wsIndex struct {
	nodes        map[domain.ID]nodeEntry
	collections  map[domain.ID]string // 集合 ID → 集合目录
	environments map[domain.ID]string // 环境 ID → 文件路径
}

type nodeEntry struct {
	path     string // 目录节点存目录，请求节点存文件
	kind     domain.Kind
	collID   domain.ID
	parentID domain.ID
}

// New 在 root 下打开文件后端，目录不存在时创建。
func New(root string) (*FS, error) {
	if err := os.MkdirAll(root, 0o700); err != nil {
		return nil, err
	}
	return &FS{
		root:  root,
		index: map[domain.ID]*wsIndex{},
	}, nil
}

// Root 返回数据根目录。
func (f *FS) Root() string { return f.root }

// List 返回工作区注册表，首次运行会创建默认工作区，旧数据在这里迁移。
func (f *FS) List(ctx context.Context) ([]storage.Ref, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	reg, changed, err := f.loadRegistryLocked()
	if err != nil {
		return nil, err
	}
	if f.ensureDefaultLocked(&reg) {
		changed = true
	}
	if changed {
		if err := f.writeRegistryLocked(reg); err != nil {
			return nil, err
		}
	}

	f.refs = refsOf(reg)
	out := make([]storage.Ref, len(f.refs))
	copy(out, f.refs)
	return out, nil
}

// Load 读取一个工作区的全部文档。
func (f *FS) Load(ctx context.Context, ref storage.Ref) (*storage.Bundle, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.loadLocked(ref)
}

func (f *FS) loadLocked(ref storage.Ref) (*storage.Bundle, error) {
	dir := f.abs(ref.Path)
	ws, err := readWorkspace(dir)
	if err != nil {
		return nil, err
	}
	ws.Default = ref.Default

	b := &storage.Bundle{
		Ref:       ref,
		Workspace: ws,
		Local:     domain.Local{Schema: domain.SchemaLocal, Version: domain.Version},
		Secrets:   map[domain.ID]string{},
	}
	idx := &wsIndex{
		nodes:        map[domain.ID]nodeEntry{},
		collections:  map[domain.ID]string{},
		environments: map[domain.ID]string{},
	}
	f.index[ref.ID] = idx

	if local, err := readLocal(dir); err == nil {
		b.Local = local
	} else if !errors.Is(err, os.ErrNotExist) {
		b.Problems = append(b.Problems, problemText(err))
		if errors.Is(err, storage.ErrNewerVersion) {
			b.ReadOnly = true
		}
	}
	if secrets, err := readSecrets(dir); err == nil {
		b.Secrets = secrets
	} else if !errors.Is(err, os.ErrNotExist) {
		b.Problems = append(b.Problems, problemText(err))
		if errors.Is(err, storage.ErrNewerVersion) {
			b.ReadOnly = true
		}
	}

	f.loadEnvironmentsLocked(dir, idx, b)
	f.loadCollectionsLocked(dir, idx, b)

	applySecrets(ws.Variables, b.Secrets)
	for _, c := range b.Collections {
		applyCollectionSecrets(c, b.Secrets)
	}
	return b, nil
}

func (f *FS) loadCollectionsLocked(dir string, idx *wsIndex, b *storage.Bundle) {
	root := filepath.Join(dir, collectionsDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			b.Problems = append(b.Problems, problemText(err))
		}
		return
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		coll, err := f.readCollectionLocked(filepath.Join(root, e.Name()), idx, b)
		if err != nil {
			if errors.Is(err, storage.ErrNewerVersion) {
				b.ReadOnly = true
			}
			b.Problems = append(b.Problems, problemText(err))
			continue
		}
		b.Collections = append(b.Collections, coll)
	}
	sort.Slice(b.Collections, func(i, j int) bool {
		return lessOrdered(b.Collections[i].Order, b.Collections[i].Name,
			b.Collections[j].Order, b.Collections[j].Name)
	})
}

func (f *FS) readCollectionLocked(dir string, idx *wsIndex, b *storage.Bundle) (*domain.Collection, error) {
	path := filepath.Join(dir, collectionFile)
	var c domain.Collection
	if err := readDoc(path, domain.SchemaCollection, &c); err != nil {
		return nil, err
	}
	normalizeCollection(&c)
	idx.collections[c.ID] = dir

	items, err := f.readChildrenLocked(dir, "", c.ID, idx, b)
	if err != nil {
		return nil, err
	}
	c.Items = items
	idx.collections[c.ID] = dir
	return &c, nil
}

// readChildrenLocked 扫描一个目录，返回排好序的子节点。
func (f *FS) readChildrenLocked(dir string, parentID, collID domain.ID,
	idx *wsIndex, b *storage.Bundle) ([]*domain.Node, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	out := make([]*domain.Node, 0, len(entries))
	for _, e := range entries {
		full := filepath.Join(dir, e.Name())
		switch {
		case e.IsDir():
			if !exists(filepath.Join(full, folderFile)) {
				continue // 用户自己的其他目录，不动它
			}
			n, err := f.readFolderLocked(full, parentID, collID, idx, b)
			if err != nil {
				if errors.Is(err, storage.ErrNewerVersion) {
					b.ReadOnly = true
				}
				b.Problems = append(b.Problems, problemText(err))
				continue
			}
			out = append(out, n)
		case strings.HasSuffix(e.Name(), requestSuffix):
			n, err := readRequestFile(full)
			if err != nil {
				if errors.Is(err, storage.ErrNewerVersion) {
					b.ReadOnly = true
				}
				b.Problems = append(b.Problems, problemText(err))
				continue
			}
			idx.nodes[n.ID] = nodeEntry{path: full, kind: domain.KindRequest,
				collID: collID, parentID: parentID}
			out = append(out, n)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		return lessOrdered(out[i].Order, out[i].Name, out[j].Order, out[j].Name)
	})
	return out, nil
}

func (f *FS) readFolderLocked(dir string, parentID, collID domain.ID,
	idx *wsIndex, b *storage.Bundle) (*domain.Node, error) {
	path := filepath.Join(dir, folderFile)
	var n domain.Node
	if err := readDoc(path, domain.SchemaFolder, &n); err != nil {
		return nil, err
	}
	normalizeNode(&n, domain.KindFolder)
	idx.nodes[n.ID] = nodeEntry{path: dir, kind: domain.KindFolder,
		collID: collID, parentID: parentID}
	children, err := f.readChildrenLocked(dir, n.ID, collID, idx, b)
	if err != nil {
		return nil, err
	}
	n.Children = children
	return &n, nil
}

func (f *FS) loadEnvironmentsLocked(dir string, idx *wsIndex, b *storage.Bundle) {
	root := filepath.Join(dir, environmentsDir)
	entries, err := os.ReadDir(root)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			b.Problems = append(b.Problems, problemText(err))
		}
		return
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), environmentSuffix) {
			continue
		}
		path := filepath.Join(root, e.Name())
		var env domain.Environment
		if err := readDoc(path, domain.SchemaEnvironment, &env); err != nil {
			if errors.Is(err, storage.ErrNewerVersion) {
				b.ReadOnly = true
			}
			b.Problems = append(b.Problems, problemText(err))
			continue
		}
		normalizeEnvironment(&env, e.Name())
		applySecrets(env.Vars, b.Secrets)
		idx.environments[env.ID] = path
		b.Environments = append(b.Environments, env)
	}
	sort.Slice(b.Environments, func(i, j int) bool {
		return b.Environments[i].Name < b.Environments[j].Name
	})
}

// CreateWorkspace 新建工作区；dir 为空时在数据根目录下分配。
func (f *FS) CreateWorkspace(ctx context.Context, name, dir string) (*storage.Bundle, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, errors.New("工作区名不能为空")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	reg, _, err := f.loadRegistryLocked()
	if err != nil {
		return nil, err
	}
	id := domain.NewID()
	var abs string
	if strings.TrimSpace(dir) == "" {
		abs = f.uniqueWorkspaceDir(name, id)
	} else {
		abs = filepath.Clean(dir)
		// 目录里已经有 workspace.json 时直接接管，支持拷贝整个工作区目录后打开。
		if exists(filepath.Join(abs, workspaceFile)) {
			ws, err := readWorkspace(abs)
			if err != nil {
				return nil, err
			}
			for _, w := range reg.Workspaces {
				if w.ID == ws.ID {
					return nil, storage.ErrExists
				}
			}
			ref := storage.Ref{ID: ws.ID, Path: f.rel(abs)}
			reg.Workspaces = append(reg.Workspaces, registryEntry{ID: ref.ID, Path: ref.Path})
			if err := f.writeRegistryLocked(reg); err != nil {
				return nil, err
			}
			f.refs = refsOf(reg)
			return f.loadLocked(ref)
		}
	}
	if err := writeWorkspaceDoc(abs, id, name); err != nil {
		return nil, err
	}
	ref := storage.Ref{ID: id, Path: f.rel(abs)}
	reg.Workspaces = append(reg.Workspaces, registryEntry{ID: ref.ID, Path: ref.Path})
	if err := f.writeRegistryLocked(reg); err != nil {
		return nil, err
	}
	f.refs = refsOf(reg)
	return f.loadLocked(ref)
}

// UpdateWorkspace 写回 workspace.json。目录名保持稳定，改名不影响路径。
func (f *FS) UpdateWorkspace(ctx context.Context, ref storage.Ref, ws *domain.Workspace) error {
	if ws == nil {
		return errors.New("工作区为空")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	ws.Schema = domain.SchemaWorkspace
	ws.Version = domain.Version
	return writeJSON(filepath.Join(f.abs(ref.Path), workspaceFile), ws, 0o644)
}

// RemoveWorkspace 只取消注册，磁盘目录保持原样。
func (f *FS) RemoveWorkspace(ctx context.Context, ref storage.Ref) error {
	if ref.Default {
		return storage.ErrDefaultWorkspace
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	reg, _, err := f.loadRegistryLocked()
	if err != nil {
		return err
	}
	idx := -1
	for i, w := range reg.Workspaces {
		if w.ID == ref.ID {
			idx = i
			break
		}
	}
	if idx < 0 {
		return storage.ErrNotFound
	}
	reg.Workspaces = append(reg.Workspaces[:idx], reg.Workspaces[idx+1:]...)
	if err := f.writeRegistryLocked(reg); err != nil {
		return err
	}
	delete(f.index, ref.ID)
	f.refs = refsOf(reg)
	return nil
}

// CreateCollection 在 collections/ 下创建一个集合目录。
func (f *FS) CreateCollection(ctx context.Context, ref storage.Ref, in storage.Input) (*domain.Collection, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, errors.New("集合名不能为空")
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return nil, storage.ErrNotFound
	}
	root := filepath.Join(f.abs(ref.Path), collectionsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return nil, err
	}
	dir := filepath.Join(root, uniqueDirBase(root, safeName(in.Name), ""))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	now := time.Now()
	c := &domain.Collection{
		Schema:      domain.SchemaCollection,
		Version:     domain.Version,
		ID:          domain.NewID(),
		Name:        in.Name,
		Description: in.Description,
		Order:       nextCollectionOrder(idx, root),
		CreatedAt:   now,
		UpdatedAt:   now,
		Items:       []*domain.Node{},
	}
	if err := writeJSON(filepath.Join(dir, collectionFile), c, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	idx.collections[c.ID] = dir
	return c, nil
}

// DeleteCollection 删除集合目录及其全部内容。
func (f *FS) DeleteCollection(ctx context.Context, ref storage.Ref, id domain.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	dir, ok := idx.collections[id]
	if !ok {
		return storage.ErrNotFound
	}
	if err := os.RemoveAll(dir); err != nil {
		return err
	}
	delete(idx.collections, id)
	for nid, e := range idx.nodes {
		if e.collID == id {
			delete(idx.nodes, nid)
		}
	}
	return nil
}

// SaveCollection 写集合元数据；改名时目录一起改。
func (f *FS) SaveCollection(ctx context.Context, ref storage.Ref, c *domain.Collection) error {
	if c == nil {
		return errors.New("集合为空")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	dir, ok := idx.collections[c.ID]
	if !ok {
		return storage.ErrNotFound
	}
	c.Schema = domain.SchemaCollection
	c.Version = domain.Version
	c.UpdatedAt = time.Now()

	if c.Name = strings.TrimSpace(c.Name); c.Name == "" {
		c.Name = "未命名集合"
	}
	parent := filepath.Dir(dir)
	base := uniqueDirBase(parent, safeName(c.Name), dir)
	target := filepath.Join(parent, base)
	if target != dir {
		if err := os.Rename(dir, target); err != nil {
			return err
		}
		idx.collections[c.ID] = target
		dir = target
		f.rebuildCollectionLocked(idx, c.ID)
	}
	return writeJSON(filepath.Join(dir, collectionFile), c, 0o644)
}

// SaveNode 新建或更新目录/请求；改名时对应的文件或目录一起改。
func (f *FS) SaveNode(ctx context.Context, ref storage.Ref, collID, parentID domain.ID,
	n *domain.Node) error {
	if n == nil {
		return errors.New("节点为空")
	}
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	collDir, ok := idx.collections[collID]
	if !ok {
		return storage.ErrNotFound
	}
	if n.ID == "" {
		n.ID = domain.NewID()
	}
	if n.Name = strings.TrimSpace(n.Name); n.Name == "" {
		n.Name = "未命名"
	}
	n.Version = domain.Version

	existing, updating := idx.nodes[n.ID]
	if updating {
		if existing.collID != collID {
			return storage.ErrNotFound
		}
		n.Kind = existing.kind
		n.Schema = schemaOf(n.Kind)
		if n.Kind == domain.KindFolder {
			n.Request = nil
		}
		if err := f.saveExistingNodeLocked(idx, collID, existing, n); err != nil {
			return err
		}
		return nil
	}

	kind := n.Kind
	if kind == "" {
		if n.Request != nil {
			kind = domain.KindRequest
		} else {
			kind = domain.KindFolder
		}
	}
	n.Kind = kind
	n.Schema = schemaOf(kind)
	if kind == domain.KindFolder {
		n.Request = nil
	}

	container := collDir
	if parentID != "" {
		parent, ok := idx.nodes[parentID]
		if !ok || parent.kind != domain.KindFolder {
			return storage.ErrNotFound
		}
		container = parent.path
	}

	var path string
	if kind == domain.KindFolder {
		base := uniqueDirBase(container, safeName(n.Name), "")
		dir := filepath.Join(container, base)
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		if err := writeNode(filepath.Join(dir, folderFile), n); err != nil {
			return err
		}
		path = dir
	} else {
		base := uniqueFileBase(container, safeName(n.Name), requestSuffix, "")
		path = filepath.Join(container, base+requestSuffix)
		if err := writeNode(path, n); err != nil {
			return err
		}
	}
	idx.nodes[n.ID] = nodeEntry{path: path, kind: kind, collID: collID, parentID: parentID}
	return nil
}

func (f *FS) saveExistingNodeLocked(idx *wsIndex, collID domain.ID, existing nodeEntry,
	n *domain.Node) error {
	container := filepath.Dir(existing.path)

	var target string
	if existing.kind == domain.KindFolder {
		base := uniqueDirBase(container, safeName(n.Name), existing.path)
		target = filepath.Join(container, base)
		if target != existing.path {
			if err := os.Rename(existing.path, target); err != nil {
				return err
			}
		}
		path := filepath.Join(target, folderFile)
		if err := writeNode(path, n); err != nil {
			return err
		}
		idx.nodes[n.ID] = nodeEntry{path: target, kind: domain.KindFolder,
			collID: collID, parentID: existing.parentID}
		f.rebuildCollectionLocked(idx, collID)
		return nil
	}

	base := uniqueFileBase(container, safeName(n.Name), requestSuffix, existing.path)
	target = filepath.Join(container, base+requestSuffix)
	if target != existing.path {
		if err := os.Rename(existing.path, target); err != nil {
			return err
		}
	}
	if err := writeNode(target, n); err != nil {
		return err
	}
	idx.nodes[n.ID] = nodeEntry{path: target, kind: domain.KindRequest,
		collID: collID, parentID: existing.parentID}
	return nil
}

// DeleteNode 删除目录/请求；目录会连同子树一起删除。
func (f *FS) DeleteNode(ctx context.Context, ref storage.Ref, collID, id domain.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	e, ok := idx.nodes[id]
	if !ok || e.collID != collID {
		return storage.ErrNotFound
	}
	if e.kind == domain.KindFolder {
		if err := os.RemoveAll(e.path); err != nil {
			return err
		}
	} else if err := os.Remove(e.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(idx.nodes, id)
	f.rebuildCollectionLocked(idx, collID)
	return nil
}

// SaveEnvironment 写环境文件；改名时文件一起改。
func (f *FS) SaveEnvironment(ctx context.Context, ref storage.Ref, env domain.Environment) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	root := filepath.Join(f.abs(ref.Path), environmentsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return err
	}
	env.Schema = domain.SchemaEnvironment
	env.Version = domain.Version
	if env.ID == "" {
		env.ID = domain.NewID()
	}
	if env.Name = strings.TrimSpace(env.Name); env.Name == "" {
		env.Name = "未命名环境"
	}
	if env.CreatedAt.IsZero() {
		env.CreatedAt = time.Now()
	}
	env.UpdatedAt = time.Now()

	current, updating := idx.environments[env.ID]
	base := uniqueFileBase(root, safeName(env.Name), environmentSuffix, current)
	target := filepath.Join(root, base+environmentSuffix)
	if updating && target != current {
		if err := os.Rename(current, target); err != nil {
			return err
		}
	}
	if err := writeJSON(target, env, 0o644); err != nil {
		return err
	}
	idx.environments[env.ID] = target
	return nil
}

// DeleteEnvironment 删除环境文件。
func (f *FS) DeleteEnvironment(ctx context.Context, ref storage.Ref, id domain.ID) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	idx := f.index[ref.ID]
	if idx == nil {
		return storage.ErrNotFound
	}
	path, ok := idx.environments[id]
	if !ok {
		return storage.ErrNotFound
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	delete(idx.environments, id)
	return nil
}

// SaveLocal 写本机状态。
func (f *FS) SaveLocal(ctx context.Context, ref storage.Ref, local domain.Local) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	local.Schema = domain.SchemaLocal
	local.Version = domain.Version
	return writeJSON(filepath.Join(f.abs(ref.Path), localFile), local, 0o600)
}

// SaveSecrets 写 secret 真实值，权限固定 0600。
func (f *FS) SaveSecrets(ctx context.Context, ref storage.Ref, values map[domain.ID]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	s := domain.Secrets{Schema: domain.SchemaSecrets, Version: domain.Version, Values: values}
	return writeJSON(filepath.Join(f.abs(ref.Path), secretsFile), s, 0o600)
}

// LoadGlobals 读取跨工作区变量。
func (f *FS) LoadGlobals(ctx context.Context) (domain.Globals, error) {
	g := domain.Globals{Schema: domain.SchemaGlobals, Version: domain.Version}
	err := readDoc(filepath.Join(f.root, globalsFile), domain.SchemaGlobals, &g)
	if errors.Is(err, os.ErrNotExist) {
		return g, nil
	}
	if err != nil {
		return g, err
	}
	g.Schema = domain.SchemaGlobals
	g.Version = domain.Version
	return g, nil
}

// SaveGlobals 写跨工作区变量。
func (f *FS) SaveGlobals(ctx context.Context, g domain.Globals) error {
	g.Schema = domain.SchemaGlobals
	g.Version = domain.Version
	return writeJSON(filepath.Join(f.root, globalsFile), g, 0o644)
}

func (f *FS) rebuildCollectionLocked(idx *wsIndex, collID domain.ID) {
	dir, ok := idx.collections[collID]
	if !ok {
		return
	}
	for id, e := range idx.nodes {
		if e.collID == collID {
			delete(idx.nodes, id)
		}
	}
	_, _ = f.readChildrenLocked(dir, "", collID, idx, &storage.Bundle{})
}

// ---- 注册表与迁移 ----

type registry struct {
	Schema     string          `json:"schema"`
	Version    int             `json:"version"`
	Workspaces []registryEntry `json:"workspaces"`
}

type registryEntry struct {
	ID      domain.ID `json:"id"`
	Path    string    `json:"path"`
	Default bool      `json:"default,omitempty"`
}

type registryV2 struct {
	Version     int               `json:"version"`
	Workspaces  []workspaceV2     `json:"workspaces"`
	Collections []collectionV2Ref `json:"collections"`
}

type workspaceV2 struct {
	ID      domain.ID `json:"id"`
	Name    string    `json:"name"`
	Default bool      `json:"default,omitempty"`
}

type collectionV2Ref struct {
	ID          domain.ID `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	WorkspaceID domain.ID `json:"workspaceID"`
}

func (f *FS) registryPath() string { return filepath.Join(f.root, registryFile) }

// loadRegistryLocked 读取注册表；旧格式或缺失时触发初始化/迁移。
func (f *FS) loadRegistryLocked() (registry, bool, error) {
	data, err := os.ReadFile(f.registryPath())
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return registry{}, false, err
		}
		reg, err := f.migrateLocked()
		return reg, true, err
	}

	var head struct {
		Schema  string `json:"schema"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return registry{}, false, fmt.Errorf("解析工作区注册表: %w", err)
	}
	if head.Version > domain.Version {
		return registry{}, false, storage.ErrNewerVersion
	}
	if head.Schema != domain.SchemaRegistry || head.Version < domain.Version {
		reg, err := f.migrateLocked()
		return reg, true, err
	}

	var reg registry
	if err := json.Unmarshal(data, &reg); err != nil {
		return registry{}, false, fmt.Errorf("解析工作区注册表: %w", err)
	}
	return reg, false, nil
}

// migrateLocked 把 v1/v2 数据迁移到「工作区目录 + 每请求一文件」。
// 先写新目录、再写新注册表，最后才删除旧集合文件；任何一步失败都保留旧文件。
func (f *FS) migrateLocked() (registry, error) {
	old := registryV2{}
	legacy := false
	data, err := os.ReadFile(f.registryPath())
	switch {
	case err == nil:
		if err := json.Unmarshal(data, &old); err != nil {
			return registry{}, fmt.Errorf("解析旧注册表: %w", err)
		}
	case errors.Is(err, os.ErrNotExist):
		legacyData, lerr := os.ReadFile(filepath.Join(f.root, legacyRegistry))
		if lerr == nil {
			legacy = true
			if err := json.Unmarshal(legacyData, &old); err != nil {
				return registry{}, fmt.Errorf("解析旧集合注册表: %w", err)
			}
		} else if errors.Is(lerr, os.ErrNotExist) {
			return f.initDefaultLocked()
		} else {
			return registry{}, lerr
		}
	default:
		return registry{}, err
	}

	// 工作区 → 新目录。
	type space struct {
		id   domain.ID
		name string
		dir  string
		def  bool
	}
	var spaces []space
	defaultIdx := -1
	for i, w := range old.Workspaces {
		if w.Default && defaultIdx < 0 {
			defaultIdx = i
		}
	}
	if len(old.Workspaces) == 0 {
		old.Workspaces = append(old.Workspaces, workspaceV2{Name: defaultName, Default: true})
		defaultIdx = 0
	}
	if defaultIdx < 0 {
		defaultIdx = 0
		old.Workspaces[0].Default = true
	}
	for i, w := range old.Workspaces {
		id := w.ID
		if id == "" {
			id = domain.NewID()
		}
		name := strings.TrimSpace(w.Name)
		if name == "" {
			name = defaultName
		}
		var dir string
		if i == defaultIdx {
			dir = f.defaultWorkspaceDir(id)
		} else {
			dir = f.uniqueWorkspaceDir(name, id)
		}
		if err := writeWorkspaceDoc(dir, id, name); err != nil {
			return registry{}, err
		}
		spaces = append(spaces, space{id: id, name: name, dir: dir, def: i == defaultIdx})
	}

	// 集合 → 工作区目录。
	spaceByID := map[domain.ID]string{}
	for _, s := range spaces {
		spaceByID[s.id] = s.dir
	}
	defaultDir := spaces[defaultIdx].dir

	var oldFiles []string
	for _, ref := range old.Collections {
		dir := spaceByID[ref.WorkspaceID]
		if dir == "" {
			dir = defaultDir
		}
		oldPath := ref.Path
		if !filepath.IsAbs(oldPath) {
			// v2 的集合路径本来就是绝对路径，这里只做兜底。
			oldPath = filepath.Join(f.root, oldPath)
		}
		if _, err := writeCollectionV2(dir, oldPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue // 文件已丢失，保留原状，不删除也不阻塞其他迁移
			}
			return registry{}, err
		}
		oldFiles = append(oldFiles, oldPath)
	}

	reg := registry{Schema: domain.SchemaRegistry, Version: domain.Version}
	for _, s := range spaces {
		reg.Workspaces = append(reg.Workspaces, registryEntry{
			ID: s.id, Path: f.rel(s.dir), Default: s.def,
		})
	}
	if err := f.writeRegistryLocked(reg); err != nil {
		return registry{}, err
	}

	// 新数据已经落盘，删除旧文件。
	for _, p := range oldFiles {
		_ = os.Remove(p)
	}
	if legacy {
		_ = os.Remove(filepath.Join(f.root, legacyRegistry))
	}
	f.refs = refsOf(reg)
	return reg, nil
}

func (f *FS) initDefaultLocked() (registry, error) {
	dir := filepath.Join(f.root, workspacesDir, defaultDirName)
	id := domain.NewID()
	name := defaultName
	if ws, err := readWorkspace(dir); err == nil {
		id, name = ws.ID, ws.Name
	}
	if err := writeWorkspaceDoc(dir, id, name); err != nil {
		return registry{}, err
	}
	reg := registry{
		Schema:  domain.SchemaRegistry,
		Version: domain.Version,
		Workspaces: []registryEntry{{
			ID: id, Path: f.rel(dir), Default: true,
		}},
	}
	if err := f.writeRegistryLocked(reg); err != nil {
		return registry{}, err
	}
	f.refs = refsOf(reg)
	return reg, nil
}

// ensureDefaultLocked 保证注册表里有一个默认工作区。
func (f *FS) ensureDefaultLocked(reg *registry) bool {
	for _, w := range reg.Workspaces {
		if w.Default {
			return false
		}
	}
	if len(reg.Workspaces) > 0 {
		reg.Workspaces[0].Default = true
		return true
	}
	dir := filepath.Join(f.root, workspacesDir, defaultDirName)
	id := domain.NewID()
	name := defaultName
	if ws, err := readWorkspace(dir); err == nil {
		id, name = ws.ID, ws.Name
	}
	if err := writeWorkspaceDoc(dir, id, name); err != nil {
		return false
	}
	reg.Workspaces = append(reg.Workspaces, registryEntry{
		ID: id, Path: f.rel(dir), Default: true,
	})
	return true
}

func (f *FS) writeRegistryLocked(reg registry) error {
	reg.Schema = domain.SchemaRegistry
	reg.Version = domain.Version
	return writeJSON(f.registryPath(), reg, 0o644)
}

func refsOf(reg registry) []storage.Ref {
	out := make([]storage.Ref, 0, len(reg.Workspaces))
	for _, w := range reg.Workspaces {
		out = append(out, storage.Ref{ID: w.ID, Path: w.Path, Default: w.Default})
	}
	return out
}

// ---- 读写辅助 ----

func (f *FS) abs(p string) string {
	if filepath.IsAbs(p) {
		return filepath.Clean(p)
	}
	return filepath.Join(f.root, p)
}

func (f *FS) rel(p string) string {
	r, err := filepath.Rel(f.root, p)
	if err == nil && r != ".." && !strings.HasPrefix(r, ".."+string(os.PathSeparator)) {
		return r
	}
	return p
}

// defaultWorkspaceDir 优先使用 workspaces/default，保证路径稳定、可预测；
// 只有该目录被另一个工作区占用时才退回到带 ID 的目录。
func (f *FS) defaultWorkspaceDir(id domain.ID) string {
	dir := filepath.Join(f.root, workspacesDir, defaultDirName)
	if !exists(dir) {
		return dir
	}
	if ws, err := readWorkspace(dir); err == nil && ws.ID == id {
		return dir
	}
	return f.uniqueWorkspaceDir(defaultDirName, id)
}

func (f *FS) uniqueWorkspaceDir(name string, id domain.ID) string {
	base := safeName(name)
	if base == "未命名" {
		base = defaultDirName
	}
	root := filepath.Join(f.root, workspacesDir)
	for i := 1; ; i++ {
		dirName := fmt.Sprintf("%s-%s", base, shortID(id))
		if i > 1 {
			dirName = fmt.Sprintf("%s-%d", dirName, i)
		}
		dir := filepath.Join(root, dirName)
		if !exists(dir) {
			return dir
		}
		if ws, err := readWorkspace(dir); err == nil && ws.ID == id {
			return dir
		}
	}
}

func writeWorkspaceDoc(dir string, id domain.ID, name string) error {
	if err := os.MkdirAll(filepath.Join(dir, collectionsDir), 0o755); err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Join(dir, environmentsDir), 0o755); err != nil {
		return err
	}
	path := filepath.Join(dir, workspaceFile)
	if exists(path) {
		return nil
	}
	now := time.Now()
	ws := domain.Workspace{
		Schema:    domain.SchemaWorkspace,
		Version:   domain.Version,
		ID:        id,
		Name:      name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	return writeJSON(path, ws, 0o644)
}

func readWorkspace(dir string) (*domain.Workspace, error) {
	var ws domain.Workspace
	if err := readDoc(filepath.Join(dir, workspaceFile), domain.SchemaWorkspace, &ws); err != nil {
		return nil, err
	}
	ws.Schema = domain.SchemaWorkspace
	ws.Version = domain.Version
	if ws.ID == "" {
		ws.ID = domain.NewID()
	}
	return &ws, nil
}

func readLocal(dir string) (domain.Local, error) {
	local := domain.Local{Schema: domain.SchemaLocal, Version: domain.Version}
	err := readDoc(filepath.Join(dir, localFile), domain.SchemaLocal, &local)
	return local, err
}

func readSecrets(dir string) (map[domain.ID]string, error) {
	s := domain.Secrets{Schema: domain.SchemaSecrets, Version: domain.Version}
	err := readDoc(filepath.Join(dir, secretsFile), domain.SchemaSecrets, &s)
	if err != nil {
		return nil, err
	}
	if s.Values == nil {
		s.Values = map[domain.ID]string{}
	}
	return s.Values, nil
}

func readDoc(path, schema string, v any) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var head struct {
		Schema  string `json:"schema"`
		Version int    `json:"version"`
	}
	if err := json.Unmarshal(data, &head); err != nil {
		return fmt.Errorf("解析 %s: %w", path, err)
	}
	if head.Version > domain.Version {
		return fmt.Errorf("%w: %s", storage.ErrNewerVersion, path)
	}
	if head.Schema != "" && schema != "" && head.Schema != schema {
		return fmt.Errorf("解析 %s: 格式为 %q，想要 %q", path, head.Schema, schema)
	}
	if err := json.Unmarshal(data, v); err != nil {
		return fmt.Errorf("解析 %s: %w", path, err)
	}
	return nil
}

func readRequestFile(path string) (*domain.Node, error) {
	var n domain.Node
	if err := readDoc(path, domain.SchemaRequest, &n); err != nil {
		return nil, err
	}
	normalizeNode(&n, domain.KindRequest)
	if n.Name == "" {
		n.Name = strings.TrimSuffix(filepath.Base(path), requestSuffix)
	}
	return &n, nil
}

func writeNode(path string, n *domain.Node) error {
	n.Schema = schemaOf(n.Kind)
	n.Version = domain.Version
	return writeJSON(path, n, 0o644)
}

func schemaOf(kind domain.Kind) string {
	if kind == domain.KindFolder {
		return domain.SchemaFolder
	}
	return domain.SchemaRequest
}

func normalizeCollection(c *domain.Collection) {
	c.Schema = domain.SchemaCollection
	c.Version = domain.Version
	if c.ID == "" {
		c.ID = domain.NewID()
	}
	c.Items = nil
}

func normalizeNode(n *domain.Node, kind domain.Kind) {
	n.Schema = schemaOf(kind)
	n.Version = domain.Version
	n.Kind = kind
	if n.ID == "" {
		n.ID = domain.NewID()
	}
	if kind == domain.KindRequest && n.Request == nil {
		n.Request = &domain.Request{}
	}
}

func normalizeEnvironment(e *domain.Environment, filename string) {
	e.Schema = domain.SchemaEnvironment
	e.Version = domain.Version
	if e.ID == "" {
		e.ID = domain.NewID()
	}
	if e.Name == "" {
		e.Name = strings.TrimSuffix(filename, environmentSuffix)
	}
}

func applySecrets(vars []domain.KV, secret map[domain.ID]string) {
	for i := range vars {
		if vars[i].ID == "" || !vars[i].Secret {
			continue
		}
		if v, ok := secret[vars[i].ID]; ok {
			vars[i].Value = v
		}
	}
}

// applyCollectionSecrets 把 secret 真实值填回集合树里的所有 KV。
func applyCollectionSecrets(c *domain.Collection, secret map[domain.ID]string) {
	if c == nil {
		return
	}
	applySecrets(c.Variables, secret)
	applySecrets(c.Auth.Params, secret)

	var walk func(nodes []*domain.Node)
	walk = func(nodes []*domain.Node) {
		for _, n := range nodes {
			applySecrets(n.Variables, secret)
			applySecrets(n.Auth.Params, secret)
			if r := n.Request; r != nil {
				applySecrets(r.PathVariables, secret)
				applySecrets(r.Query, secret)
				applySecrets(r.Headers, secret)
				applySecrets(r.Cookies, secret)
				applySecrets(r.Body.Fields, secret)
			}
			walk(n.Children)
		}
	}
	walk(c.Items)
}

func nextCollectionOrder(idx *wsIndex, root string) int64 {
	var max int64
	for _, dir := range idx.collections {
		var c domain.Collection
		if err := readDoc(filepath.Join(dir, collectionFile), domain.SchemaCollection, &c); err == nil {
			if c.Order > max {
				max = c.Order
			}
		}
	}
	if max == 0 {
		return orderStep
	}
	return max + orderStep
}

func lessOrdered(orderA int64, nameA string, orderB int64, nameB string) bool {
	if orderA != orderB {
		return orderA < orderB
	}
	return nameA < nameB
}

func exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func shortID(id domain.ID) string {
	s := string(id)
	if len(s) > 6 {
		return s[:6]
	}
	return s
}

func problemText(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}

// safeName 把业务名称转成安全的文件/目录名。保留中文，
// 替换路径分隔符与 Windows 保留字符，避免数据在跨平台同步时出问题。
func safeName(name string) string {
	name = strings.TrimSpace(name)
	repl := strings.NewReplacer(
		"/", "-", "\\", "-", ":", "-", "*", "-", "?", "-",
		"\"", "-", "<", "-", ">", "-", "|", "-", "\x00", "",
	)
	name = strings.Trim(repl.Replace(name), " .")
	if name == "" {
		name = "未命名"
	}
	runes := []rune(name)
	if len(runes) > 80 {
		name = string(runes[:80])
	}
	if isReserved(name) {
		name = "_" + name
	}
	return name
}

func isReserved(name string) bool {
	switch strings.ToUpper(name) {
	case "CON", "PRN", "AUX", "NUL":
		return true
	}
	if len(name) == 4 {
		upper := strings.ToUpper(name)
		if strings.HasPrefix(upper, "COM") || strings.HasPrefix(upper, "LPT") {
			c := upper[3]
			return c >= '1' && c <= '9'
		}
	}
	return false
}

// uniqueDirBase 返回 parent 下可用的目录名；current 是当前节点自己的目录，
// 用来避免「改大小写」这类目标与自身相同的误判。
func uniqueDirBase(parent, base, current string) string {
	candidate := base
	for i := 2; ; i++ {
		path := filepath.Join(parent, candidate)
		if path == current || !exists(path) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", base, i)
	}
}

// uniqueFileBase 返回 parent 下可用的文件名（不含后缀）。
func uniqueFileBase(parent, base, suffix, current string) string {
	candidate := base
	for i := 2; ; i++ {
		path := filepath.Join(parent, candidate+suffix)
		if path == current || !exists(path) {
			return candidate
		}
		candidate = fmt.Sprintf("%s (%d)", base, i)
	}
}

func writeJSON(path string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeAtomic(path, append(data, '\n'), perm)
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

// ---- v2 → v3 迁移辅助 ----

type collectionV2 struct {
	Version int       `json:"version"`
	ID      domain.ID `json:"id"`
	Name    string    `json:"name"`
	Items   []nodeV2  `json:"items"`
}

type nodeV2 struct {
	ID       domain.ID   `json:"id"`
	Kind     domain.Kind `json:"kind"`
	Name     string      `json:"name"`
	Children []nodeV2    `json:"children"`
	Request  *requestV2  `json:"request"`
}

type requestV2 struct {
	Method  string `json:"method"`
	URL     string `json:"url"`
	Params  []kvV2 `json:"params"`
	Headers []kvV2 `json:"headers"`
	Body    string `json:"body"`
	Auth    string `json:"auth"`
}

type kvV2 struct {
	Key     string `json:"key"`
	Value   string `json:"value"`
	Desc    string `json:"desc"`
	Enabled bool   `json:"enabled"`
	Secret  bool   `json:"secret"`
}

// writeCollectionV2 把一个旧集合文件展开成目录树，返回新集合目录。
func writeCollectionV2(wsDir, oldPath string) (string, error) {
	data, err := os.ReadFile(oldPath)
	if err != nil {
		return "", err
	}
	var old collectionV2
	if err := json.Unmarshal(data, &old); err != nil {
		return "", fmt.Errorf("解析旧集合 %s: %w", oldPath, err)
	}
	if old.ID == "" {
		old.ID = domain.NewID()
	}
	if old.Name == "" {
		old.Name = "未命名集合"
	}

	root := filepath.Join(wsDir, collectionsDir)
	if err := os.MkdirAll(root, 0o755); err != nil {
		return "", err
	}
	// 重试迁移时复用已经写好的同 ID 集合目录，并清空上次的半成品，
	// 保证迁移可以安全地重复执行（旧文件此时还没有删除）。
	dir := findCollectionDir(root, old.ID)
	if dir != "" {
		if err := os.RemoveAll(dir); err != nil {
			return "", err
		}
	} else {
		dir = filepath.Join(root, uniqueDirBase(root, safeName(old.Name), ""))
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	now := time.Now()
	c := &domain.Collection{
		Schema:    domain.SchemaCollection,
		Version:   domain.Version,
		ID:        old.ID,
		Name:      old.Name,
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := writeJSON(filepath.Join(dir, collectionFile), c, 0o644); err != nil {
		return "", err
	}
	if err := writeV2Nodes(dir, old.Items); err != nil {
		return "", err
	}
	return dir, nil
}

func writeV2Nodes(dir string, nodes []nodeV2) error {
	for i, old := range nodes {
		order := int64(i+1) * orderStep
		kind := old.Kind
		if kind == "" {
			if old.Request != nil {
				kind = domain.KindRequest
			} else {
				kind = domain.KindFolder
			}
		}
		if old.ID == "" {
			old.ID = domain.NewID()
		}
		name := strings.TrimSpace(old.Name)
		if name == "" {
			name = "未命名"
		}
		now := time.Now()
		n := &domain.Node{
			Schema:    schemaOf(kind),
			Version:   domain.Version,
			ID:        old.ID,
			Name:      name,
			Order:     order,
			Kind:      kind,
			CreatedAt: now,
			UpdatedAt: now,
		}
		if kind == domain.KindFolder {
			sub := filepath.Join(dir, uniqueDirBase(dir, safeName(name), ""))
			if err := os.MkdirAll(sub, 0o755); err != nil {
				return err
			}
			n.Request = nil
			if err := writeJSON(filepath.Join(sub, folderFile), n, 0o644); err != nil {
				return err
			}
			if err := writeV2Nodes(sub, old.Children); err != nil {
				return err
			}
			continue
		}
		if old.Request != nil {
			n.Request = convertV2Request(old.Request)
			n.Auth = convertV2Auth(old.Request.Auth)
		} else {
			n.Request = &domain.Request{}
		}
		base := uniqueFileBase(dir, safeName(name), requestSuffix, "")
		if err := writeJSON(filepath.Join(dir, base+requestSuffix), n, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func convertV2Request(old *requestV2) *domain.Request {
	r := &domain.Request{
		Protocol: "http",
		Method:   old.Method,
		URL:      old.URL,
		Query:    convertV2KVs(old.Params),
		Headers:  convertV2KVs(old.Headers),
	}
	if strings.TrimSpace(old.Body) != "" {
		r.Body = domain.Body{Mode: domain.BodyRaw, Raw: old.Body}
	}
	return r
}

func convertV2KVs(kvs []kvV2) []domain.KV {
	if len(kvs) == 0 {
		return nil
	}
	out := make([]domain.KV, len(kvs))
	for i, kv := range kvs {
		out[i] = domain.KV{
			ID:      domain.NewID(),
			Key:     kv.Key,
			Value:   kv.Value,
			Desc:    kv.Desc,
			Enabled: kv.Enabled,
			Secret:  kv.Secret,
		}
	}
	return out
}

func convertV2Auth(s string) domain.Auth {
	switch domain.AuthType(strings.ToLower(strings.TrimSpace(s))) {
	case "":
		return domain.Auth{}
	case domain.AuthInherit:
		return domain.Auth{Type: domain.AuthInherit}
	case domain.AuthNone:
		return domain.Auth{Type: domain.AuthNone}
	default:
		return domain.Auth{Type: domain.AuthType(strings.TrimSpace(s))}
	}
}

func findCollectionDir(root string, id domain.ID) string {
	entries, err := os.ReadDir(root)
	if err != nil {
		return ""
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		var c domain.Collection
		if err := readDoc(filepath.Join(root, e.Name(), collectionFile), domain.SchemaCollection, &c); err == nil && c.ID == id {
			return filepath.Join(root, e.Name())
		}
	}
	return ""
}

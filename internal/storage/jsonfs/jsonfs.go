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
	"errors"
	"os"
	"path/filepath"
	"sync"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

const (
	globalsFile       = "globals.json"
	workspaceFile     = "workspace.json"
	localFile         = "local.json"
	secretsFile       = "secrets.json"
	collectionsDir    = "collections"
	environmentsDir   = "environments"
	collectionFile    = "collection.json"
	folderFile        = "folder.json"
	readmeFile        = "README.md"
	collectionDocFile = "collection.md"
	folderDocFile     = "folder.md"
	requestSuffix     = ".request.json"
	environmentSuffix = ".environment.json"
	workspacesDir     = "workspaces"
	historyDir        = "history"
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
	maxCollOrder int64                // 集合排序值缓存，避免重复读盘
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

// SaveLocal 写本机状态。
func (f *FS) SaveLocal(ctx context.Context, ref storage.Ref, local domain.Local) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	local.Schema = domain.SchemaLocal
	local.Version = domain.Version
	if local.CreatedAt.IsZero() {
		local.CreatedAt = time.Now()
	}
	local.UpdatedAt = time.Now()
	return writeJSON(filepath.Join(f.abs(ref.Path), localFile), local, 0o600)
}

// SaveSecrets 写 secret 真实值，权限固定 0600。
func (f *FS) SaveSecrets(ctx context.Context, ref storage.Ref, values map[domain.ID]string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	p := filepath.Join(f.abs(ref.Path), secretsFile)
	now := time.Now()
	s := domain.Secrets{
		Schema:    domain.SchemaSecrets,
		Version:   domain.Version,
		Values:    values,
		CreatedAt: now,
		UpdatedAt: now,
	}
	var existing domain.Secrets
	if err := readDoc(p, domain.SchemaSecrets, &existing); err == nil && !existing.CreatedAt.IsZero() {
		s.CreatedAt = existing.CreatedAt
	}
	return writeJSON(p, s, 0o600)
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
	if g.CreatedAt.IsZero() {
		g.CreatedAt = time.Now()
	}
	g.UpdatedAt = time.Now()
	return writeJSON(filepath.Join(f.root, globalsFile), g, 0o644)
}

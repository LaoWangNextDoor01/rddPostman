package jsonfs

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

// CreateWorkspace 创建工作区；dir 为空时在数据根目录自动分配。
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
	if ws.CreatedAt.IsZero() {
		ws.CreatedAt = time.Now()
	}
	ws.UpdatedAt = time.Now()
	return writeJSON(filepath.Join(f.abs(ref.Path), workspaceFile), ws, 0o644)
}

// WorkspaceDir 返回工作区在磁盘上的绝对目录路径。
func (f *FS) WorkspaceDir(ref storage.Ref) string {
	return f.abs(ref.Path)
}

// SaveWorkspaceIcon 将本地图片复制到工作区目录内，命名固定为 .rddpostman.icon.<ext>
// 以避免与工作区元数据或用户项目文件冲突，并自动清理旧后缀图标文件。
// srcPath 为空时删除已有图标并返回空串。
func (f *FS) SaveWorkspaceIcon(ctx context.Context, ref storage.Ref, srcPath string) (string, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	dir := f.abs(ref.Path)
	srcPath = strings.TrimSpace(srcPath)
	if srcPath == "" {
		removeStaleWorkspaceIcons(dir, "")
		return "", nil
	}

	ext := normalizeIconExt(filepath.Ext(srcPath))
	relName := workspaceIconPrefix + ext
	dst := filepath.Join(dir, relName)

	// 源文件已是目标文件本身时直接返回，避免误删或多余拷贝。
	if srcInfo, err := os.Stat(srcPath); err == nil {
		if dstInfo, err := os.Stat(dst); err == nil && os.SameFile(srcInfo, dstInfo) {
			removeStaleWorkspaceIcons(dir, relName)
			return relName, nil
		}
	}

	data, err := os.ReadFile(srcPath)
	if err != nil {
		return "", fmt.Errorf("读取工作区图标失败: %w", err)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if err := writeAtomic(dst, data, 0o644); err != nil {
		return "", fmt.Errorf("写入工作区图标失败: %w", err)
	}
	removeStaleWorkspaceIcons(dir, relName)
	return relName, nil
}

func normalizeIconExt(ext string) string {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case ".png", ".jpg", ".jpeg", ".svg", ".webp", ".gif":
		return strings.ToLower(strings.TrimSpace(ext))
	default:
		return ".png"
	}
}

func removeStaleWorkspaceIcons(dir, keep string) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	prefix := workspaceIconPrefix + "."
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasPrefix(name, prefix) && name != keep {
			_ = os.Remove(filepath.Join(dir, name))
		}
	}
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

package jsonfs

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

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

// SaveEnvironment 保存环境文件。
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

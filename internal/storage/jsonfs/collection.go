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
	if desc, err := readDocMarkdown(dir, readmeFile, collectionDocFile); err == nil && desc != "" {
		c.Description = desc
	}
	idx.collections[c.ID] = dir
	if c.Order > idx.maxCollOrder {
		idx.maxCollOrder = c.Order
	}

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
	if desc, err := readDocMarkdown(dir, readmeFile, folderDocFile); err == nil && desc != "" {
		n.Description = desc
	}
	idx.nodes[n.ID] = nodeEntry{path: dir, kind: domain.KindFolder,
		collID: collID, parentID: parentID}
	children, err := f.readChildrenLocked(dir, n.ID, collID, idx, b)
	if err != nil {
		return nil, err
	}
	n.Children = children
	return &n, nil
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
		Order:       idx.nextCollectionOrder(),
		CreatedAt:   now,
		UpdatedAt:   now,
		Items:       []*domain.Node{},
	}
	if in.Description != "" {
		if err := writeDocMarkdown(dir, readmeFile, in.Description); err != nil {
			_ = os.RemoveAll(dir)
			return nil, err
		}
	}
	metaCopy := *c
	metaCopy.Description = ""
	if err := writeJSON(filepath.Join(dir, collectionFile), &metaCopy, 0o644); err != nil {
		_ = os.RemoveAll(dir)
		return nil, err
	}
	idx.collections[c.ID] = dir
	idx.maxCollOrder = c.Order
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
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now()
	}
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
	if err := writeDocMarkdown(dir, readmeFile, c.Description); err != nil {
		return err
	}
	metaCopy := *c
	metaCopy.Description = ""
	return writeJSON(filepath.Join(dir, collectionFile), &metaCopy, 0o644)
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
	now := time.Now()
	if n.CreatedAt.IsZero() {
		n.CreatedAt = now
	}
	if n.UpdatedAt.IsZero() {
		n.UpdatedAt = now
	}
	if n.Request != nil {
		if n.Request.CreatedAt.IsZero() {
			n.Request.CreatedAt = n.CreatedAt
		}
		if n.Request.UpdatedAt.IsZero() {
			n.Request.UpdatedAt = n.UpdatedAt
		}
	}
	if n.Kind == domain.KindFolder {
		dir := filepath.Dir(path)
		if err := writeDocMarkdown(dir, readmeFile, n.Description); err != nil {
			return err
		}
		metaCopy := *n
		metaCopy.Description = ""
		return writeJSON(path, &metaCopy, 0o644)
	}
	return writeJSON(path, n, 0o644)
}

func readDocMarkdown(dir string, names ...string) (string, error) {
	for _, name := range names {
		p := filepath.Join(dir, name)
		data, err := os.ReadFile(p)
		if err == nil {
			return string(data), nil
		}
	}
	return "", os.ErrNotExist
}

func writeDocMarkdown(dir, name, content string) error {
	p := filepath.Join(dir, name)
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		if exists(p) {
			_ = os.Remove(p)
		}
		return nil
	}
	return writeAtomic(p, []byte(content), 0o644)
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

func (idx *wsIndex) nextCollectionOrder() int64 {
	if idx.maxCollOrder == 0 {
		return orderStep
	}
	return idx.maxCollOrder + orderStep
}

func lessOrdered(orderA int64, nameA string, orderB int64, nameB string) bool {
	if orderA != orderB {
		return orderA < orderB
	}
	return nameA < nameB
}

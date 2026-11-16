package jsonfs

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

const (
	registryFile   = "workspaces.json"
	legacyRegistry = "collections.json"
)

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
	if len(reg.Workspaces) == 0 {
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
	for _, w := range reg.Workspaces {
		if w.Default {
			return false
		}
	}
	for i := range reg.Workspaces {
		abs := f.abs(reg.Workspaces[i].Path)
		if filepath.Base(abs) == defaultDirName {
			reg.Workspaces[i].Default = true
			return true
		}
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

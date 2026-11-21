package workspace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"rddpostman/internal/platform"
)

const (
	maxRecentFolders = 8
	maxShownFolders  = 5
)

// localFolderItem 表示创建页「Open a local folder」卡片中可一键选中的目录项。
type localFolderItem struct {
	Name   string
	Path   string
	Recent bool
}

// folderScanResult 保存后台目录扫描结果，避免打开创建页时在 UI 线程做磁盘 I/O。
type folderScanResult struct {
	mu       sync.Mutex
	scanning bool
	loaded   bool
	pending  bool
	items    []localFolderItem
}

func (r *folderScanResult) start() bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.scanning || r.loaded {
		return false
	}
	r.scanning = true
	return true
}

func (r *folderScanResult) reset() {
	r.mu.Lock()
	r.loaded = false
	r.pending = false
	r.items = nil
	r.mu.Unlock()
}

func (r *folderScanResult) finish(items []localFolderItem) {
	r.mu.Lock()
	r.scanning = false
	r.loaded = true
	r.pending = true
	r.items = items
	r.mu.Unlock()
}

func (r *folderScanResult) drain() ([]localFolderItem, bool) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if !r.pending {
		return nil, false
	}
	r.pending = false
	items := r.items
	r.items = nil
	return items, true
}

// openFolderPicker 弹出系统目录选择器；测试中可替换以避免阻塞系统弹窗。
var openFolderPicker = func(defaultPath string) (string, error) {
	return platform.PickDirectory("Open local folder", defaultPath)
}

// loadLocalFolders 获取创建页展示的本地目录候选列表；测试中可替换。
var loadLocalFolders = defaultLoadLocalFolders

func normalizePathKey(p string) string {
	clean := filepath.Clean(strings.TrimSpace(p))
	if runtime.GOOS == "windows" {
		return strings.ToLower(clean)
	}
	return clean
}

func isExistingDir(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	info, err := os.Stat(p)
	return err == nil && info.IsDir()
}

func recentFoldersPath() (string, error) {
	dir, err := platform.DataDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "recent-folders.json"), nil
}

func readRecentFolders() []string {
	path, err := recentFoldersPath()
	if err != nil {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var raw []string
	if err := json.Unmarshal(data, &raw); err != nil {
		return nil
	}
	seen := make(map[string]bool, len(raw))
	out := make([]string, 0, len(raw))
	for _, p := range raw {
		clean := filepath.Clean(strings.TrimSpace(p))
		key := normalizePathKey(clean)
		if key == "" || seen[key] || !isExistingDir(clean) {
			continue
		}
		seen[key] = true
		out = append(out, clean)
		if len(out) >= maxRecentFolders {
			break
		}
	}
	return out
}

// rememberRecentFolder 将用户选中的本地目录置顶写入 recent-folders.json。
func rememberRecentFolder(dir string) {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if !isExistingDir(clean) {
		return
	}
	path, err := recentFoldersPath()
	if err != nil {
		return
	}
	prev := readRecentFolders()
	key := normalizePathKey(clean)
	next := make([]string, 0, maxRecentFolders)
	next = append(next, clean)
	for _, p := range prev {
		if normalizePathKey(p) == key {
			continue
		}
		next = append(next, p)
		if len(next) >= maxRecentFolders {
			break
		}
	}
	data, err := json.MarshalIndent(next, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

// defaultLoadLocalFolders 合并「最近打开目录」与「浅层项目目录（1 层扫描，优先含 workspace.json / .git）」。
func defaultLoadLocalFolders() []localFolderItem {
	seen := make(map[string]bool)
	out := make([]localFolderItem, 0, maxShownFolders)

	add := func(dir string, recent bool) bool {
		clean := filepath.Clean(strings.TrimSpace(dir))
		key := normalizePathKey(clean)
		if key == "" || seen[key] || !isExistingDir(clean) {
			return false
		}
		name := filepath.Base(clean)
		if name == "" || name == "." || name == string(filepath.Separator) {
			name = clean
		}
		seen[key] = true
		out = append(out, localFolderItem{Name: name, Path: clean, Recent: recent})
		return true
	}

	// 1. 优先列出最近选过的目录
	recents := readRecentFolders()
	for _, dir := range recents {
		add(dir, true)
		if len(out) >= maxShownFolders {
			return out
		}
	}

	// 2. 不足 5 项时，浅扫 1 层候选父目录（最近目录的父目录 → 当前工作目录父目录 → 常见开发目录）
	roots := make([]string, 0, 6)
	if len(recents) > 0 {
		roots = append(roots, filepath.Dir(recents[0]))
	}
	if wd, err := os.Getwd(); err == nil && wd != "" {
		roots = append(roots, filepath.Dir(wd))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		for _, sub := range []string{"Projects", "Code", "Workspace", "src"} {
			roots = append(roots, filepath.Join(home, sub))
		}
	}

	var priority, normal []string
	scannedRoots := make(map[string]bool)
	for _, root := range roots {
		rKey := normalizePathKey(root)
		if rKey == "" || scannedRoots[rKey] || !isExistingDir(root) {
			continue
		}
		scannedRoots[rKey] = true
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			name := e.Name()
			if strings.HasPrefix(name, ".") || name == "node_modules" || name == "vendor" {
				continue
			}
			full := filepath.Join(root, name)
			if seen[normalizePathKey(full)] {
				continue
			}
			if hasMarker(full, "workspace.json") || hasMarker(full, ".git") {
				priority = append(priority, full)
			} else {
				normal = append(normal, full)
			}
		}
	}

	for _, dir := range priority {
		add(dir, false)
		if len(out) >= maxShownFolders {
			return out
		}
	}
	for _, dir := range normal {
		add(dir, false)
		if len(out) >= maxShownFolders {
			return out
		}
	}
	return out
}

func hasMarker(dir, name string) bool {
	_, err := os.Stat(filepath.Join(dir, name))
	return err == nil
}

// prependLocalFolder 把新选中的目录置顶插入内存候选列表（去重并保持最多 maxShownFolders 条）。
func prependLocalFolder(items []localFolderItem, dir string) []localFolderItem {
	clean := filepath.Clean(strings.TrimSpace(dir))
	if clean == "" {
		return items
	}
	key := normalizePathKey(clean)
	name := filepath.Base(clean)
	if name == "" || name == "." || name == string(filepath.Separator) {
		name = clean
	}
	next := make([]localFolderItem, 0, maxShownFolders)
	next = append(next, localFolderItem{Name: name, Path: clean, Recent: true})
	for _, it := range items {
		if normalizePathKey(it.Path) == key {
			continue
		}
		next = append(next, it)
		if len(next) >= maxShownFolders {
			break
		}
	}
	return next
}

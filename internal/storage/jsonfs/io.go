package jsonfs

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

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

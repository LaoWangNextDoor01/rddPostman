package jsonfs

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

// AppendHistory 追加一条历史记录到当天的 JSONL 文件。
func (f *FS) AppendHistory(ctx context.Context, ref storage.Ref, entry domain.HistoryEntry) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	dir := filepath.Join(f.abs(ref.Path), historyDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	if entry.At.IsZero() {
		entry.At = time.Now()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = entry.At
	}
	filename := entry.At.Format("2006-01-02") + ".jsonl"
	path := filepath.Join(dir, filename)

	data, err := json.Marshal(entry)
	if err != nil {
		return err
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	_, err = file.Write(append(data, '\n'))
	return err
}

// LoadHistory 按时间倒序返回历史记录。
func (f *FS) LoadHistory(ctx context.Context, ref storage.Ref, query storage.HistoryQuery) ([]domain.HistoryEntry, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	dir := filepath.Join(f.abs(ref.Path), historyDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, nil
		}
		return nil, err
	}

	// 按文件名倒序（日期倒序）。
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].Name() > entries[j].Name()
	})

	var out []domain.HistoryEntry
	limit := query.Limit
	if limit <= 0 {
		limit = 1000
	}
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".jsonl") {
			continue
		}
		records, err := readJSONL(filepath.Join(dir, e.Name()))
		if err != nil {
			continue
		}
		for i := len(records) - 1; i >= 0; i-- {
			r := records[i]
			if !query.Before.IsZero() && !r.At.Before(query.Before) {
				continue
			}
			out = append(out, r)
			if len(out) >= limit {
				return out, nil
			}
		}
	}
	return out, nil
}

func readJSONL(path string) ([]domain.HistoryEntry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()
	var out []domain.HistoryEntry
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		var entry domain.HistoryEntry
		if err := json.Unmarshal(scanner.Bytes(), &entry); err != nil {
			continue
		}
		if entry.CreatedAt.IsZero() {
			entry.CreatedAt = entry.At
		}
		out = append(out, entry)
	}
	return out, scanner.Err()
}

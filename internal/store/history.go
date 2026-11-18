package store

import (
	"context"
	"time"

	"rddpostman/internal/domain"
	"rddpostman/internal/storage"
)

// RecordHistory 追加一条请求历史记录。
func (s *Store) RecordHistory(wsID domain.ID, entry domain.HistoryEntry) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if entry.ID == "" {
		entry.ID = domain.NewID()
	}
	if entry.At.IsZero() {
		entry.At = time.Now()
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = entry.At
	}
	entry.WorkspaceID = wsID
	return s.backend.AppendHistory(context.Background(), st.ref, entry)
}

// History 按时间倒序返回工作区的请求历史，limit ≤ 0 时默认返回最近 100 条。
func (s *Store) History(wsID domain.ID, limit int) ([]domain.HistoryEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return nil, ErrNotFound
	}
	if limit <= 0 {
		limit = 100
	}
	return s.backend.LoadHistory(context.Background(), st.ref, storage.HistoryQuery{Limit: limit})
}

// ClearHistory 清空指定工作区的历史记录。
func (s *Store) ClearHistory(wsID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	return s.backend.ClearHistory(context.Background(), st.ref)
}

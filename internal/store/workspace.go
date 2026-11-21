package store

import (
	"context"
	"errors"
	"strings"
	"time"

	"rddpostman/internal/domain"
)

// CreateWorkspace 在默认位置新建工作区。
func (s *Store) CreateWorkspace(name string) (*domain.Workspace, error) {
	return s.CreateWorkspaceAt(name, "")
}

// CreateWorkspaceAt 在 dir 建工作区；dir 为空时由后端在数据根目录下分配。
// dir 是用户自选目录时，整个目录就是工作区，可直接复制或用 Git 管理。
func (s *Store) CreateWorkspaceAt(name, dir string) (*domain.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	bundle, err := s.backend.CreateWorkspace(context.Background(), name, dir)
	if err != nil {
		return nil, err
	}
	st := &state{
		ref:         bundle.Ref,
		ws:          bundle.Workspace,
		collections: bundle.Collections,
		envs:        bundle.Environments,
		local:       bundle.Local,
		secrets:     bundle.Secrets,
	}
	_ = s.ensureEnvironmentsLocked(context.Background(), st)
	s.states[bundle.Ref.ID] = st
	for _, c := range st.collections {
		s.collIndex[c.ID] = st
	}
	s.refs = append(s.refs, bundle.Ref)
	s.publishLocked()
	return bundle.Workspace, nil
}

// RenameWorkspace 修改工作区名；目录保持稳定。
func (s *Store) RenameWorkspace(id domain.ID, name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("工作区名不能为空")
	}
	return s.UpdateWorkspace(id, func(ws *domain.Workspace) {
		ws.Name = name
	})
}

// SetWorkspaceStarred 设置工作区收藏状态。
func (s *Store) SetWorkspaceStarred(id domain.ID, starred bool) error {
	return s.UpdateWorkspace(id, func(ws *domain.Workspace) {
		ws.Starred = starred
	})
}

// SetWorkspaceIcon 将 srcPath 指向的本地图片复制到工作区目录（.rddpostman.icon.<ext>），
// 并在 workspace.json 中保存相对文件名；srcPath 为空时清除已有图标。
func (s *Store) SetWorkspaceIcon(id domain.ID, srcPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[id]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	relPath, err := s.backend.SaveWorkspaceIcon(context.Background(), st.ref, srcPath)
	if err != nil {
		return err
	}
	next := st.ws.Clone()
	next.Icon = relPath
	next.UpdatedAt = time.Now()
	if err := s.backend.UpdateWorkspace(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.ws = next
	s.publishLocked()
	return nil
}

// UpdateWorkspace 修改工作区元数据（名称、描述、工作区级变量等）。
func (s *Store) UpdateWorkspace(id domain.ID, fn func(*domain.Workspace)) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[id]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	next := st.ws.Clone()
	fn(next)
	next.UpdatedAt = time.Now()
	if err := s.backend.UpdateWorkspace(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.ws = next
	s.publishLocked()
	return nil
}

// RemoveWorkspace 取消注册，磁盘上的工作区目录保持原样。
func (s *Store) RemoveWorkspace(id domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[id]
	if st == nil {
		return ErrNotFound
	}
	if st.ref.Default {
		return ErrDefaultWorkspace
	}
	if err := s.backend.RemoveWorkspace(context.Background(), st.ref); err != nil {
		return err
	}
	for _, c := range st.collections {
		delete(s.collIndex, c.ID)
	}
	delete(s.states, id)
	out := s.refs[:0]
	for _, ref := range s.refs {
		if ref.ID != id {
			out = append(out, ref)
		}
	}
	s.refs = out
	for key := range s.dirtyNode {
		if key.ws == id {
			delete(s.dirtyNode, key)
		}
	}
	s.publishLocked()
	return nil
}

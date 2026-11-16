package store

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"rddpostman/internal/domain"
)

const flushDelay = 300 * time.Millisecond

// Flush 立即把待写内容落盘。
func (s *Store) Flush() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.flushLocked()
}

// Close 落盘并停止防抖定时器。
func (s *Store) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	return s.flushLocked()
}

func (s *Store) scheduleLocked() {
	if s.timer != nil {
		s.timer.Stop()
	}
	s.timer = time.AfterFunc(flushDelay, func() {
		if err := s.Flush(); err != nil {
			fmt.Fprintf(os.Stderr, "store: 保存失败: %v\n", err)
		}
	})
}

func (s *Store) flushLocked() error {
	if s.timer != nil {
		s.timer.Stop()
		s.timer = nil
	}
	keys := make([]nodeKey, 0, len(s.dirtyNode))
	for key := range s.dirtyNode {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].ws != keys[j].ws {
			return keys[i].ws < keys[j].ws
		}
		if keys[i].coll != keys[j].coll {
			return keys[i].coll < keys[j].coll
		}
		return keys[i].node < keys[j].node
	})

	var firstErr error
	for _, key := range keys {
		n := s.dirtyNode[key]
		st := s.states[key.ws]
		if st == nil || n == nil || !nodeExists(st, key.coll, key.node) {
			delete(s.dirtyNode, key)
			continue
		}
		persist, err := s.preparePersistNodeLocked(st, n)
		if err == nil {
			err = s.backend.SaveNode(context.Background(), st.ref, key.coll, "", persist)
		}
		if err != nil {
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		delete(s.dirtyNode, key)
	}
	return firstErr
}

// preparePersistNodeLocked 生成用于落盘的节点副本：secret 值剥离到 secrets.json。
func (s *Store) preparePersistNodeLocked(st *state, n *domain.Node) (*domain.Node, error) {
	out := n.Clone()
	if err := s.splitSecretsLocked(st, out); err != nil {
		return nil, err
	}
	return out, nil
}

// splitSecretsLocked 把 secret KV 的值移进工作区的 secrets 表并清空副本里的值。
// 目录变量、请求参数、请求头、认证参数都走同一套逻辑。
func (s *Store) splitSecretsLocked(st *state, n *domain.Node) error {
	found := false
	found = s.stashSecretsLocked(st, n.Variables) || found
	found = s.stashSecretsLocked(st, n.Auth.Params) || found
	if n.Request != nil {
		found = s.stashSecretsLocked(st, n.Request.PathVariables) || found
		found = s.stashSecretsLocked(st, n.Request.Query) || found
		found = s.stashSecretsLocked(st, n.Request.Headers) || found
		found = s.stashSecretsLocked(st, n.Request.Cookies) || found
		found = s.stashSecretsLocked(st, n.Request.Body.Fields) || found
	}
	if !found {
		return nil
	}
	return s.backend.SaveSecrets(context.Background(), st.ref, st.secrets)
}

// splitSecretsLocked 的环境版本。
func (s *Store) splitEnvSecretsLocked(st *state, env *domain.Environment) error {
	if !s.stashSecretsLocked(st, env.Vars) {
		return nil
	}
	return s.backend.SaveSecrets(context.Background(), st.ref, st.secrets)
}

// stashSecretsLocked 把 secret 值记进内存 secrets 表，并清空待落盘副本里的值。
// 空值表示用户清除了 secret，需要把旧值从表里删掉。
func (s *Store) stashSecretsLocked(st *state, kvs []domain.KV) bool {
	found := false
	for i := range kvs {
		if !kvs[i].Secret || kvs[i].ID == "" {
			continue
		}
		found = true
		if kvs[i].Value == "" {
			delete(st.secrets, kvs[i].ID)
		} else {
			if st.secrets == nil {
				st.secrets = map[domain.ID]string{}
			}
			st.secrets[kvs[i].ID] = kvs[i].Value
		}
		kvs[i].Value = ""
	}
	return found
}

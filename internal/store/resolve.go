package store

import (
	"sort"

	"rddpostman/internal/domain"
)

// ---- 变量与请求准备 ----

// Variables 合并一个请求可见的全部变量，优先级从低到高：
// 全局 < 工作区 < 当前环境 < 集合 < 目录（越深越优先）< 请求。
func (s *Store) Variables(wsID, collID, nodeID domain.ID) map[string]domain.Variable {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return nil
	}
	return s.variablesLocked(st, collID, nodeID)
}

// PrepareRequest 返回替换变量后的请求与最终生效的认证配置。
func (s *Store) PrepareRequest(wsID, collID, nodeID domain.ID) (domain.Request, domain.Auth, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.states[wsID]
	if st == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}
	var coll *domain.Collection
	for _, c := range st.collections {
		if c.ID == collID {
			coll = c
			break
		}
	}
	if coll == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}
	n := domain.FindIn(coll.Items, nodeID)
	if n == nil || n.Request == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}

	vars := s.variablesLocked(st, collID, nodeID)
	req, missing := domain.Prepare(*n.Request, vars)
	preparedAuth, authMissing := domain.PrepareAuth(s.authChainLocked(coll, nodeID), vars)
	return req, preparedAuth, mergeMissing(missing, authMissing), nil
}

// PrepareRequestWith 基于传入的 Request 和 Auth（包含 UI 实时编辑草稿）解析最终变量插值与认证继承配置。
func (s *Store) PrepareRequestWith(wsID, collID, nodeID domain.ID, draftReq domain.Request, draftAuth domain.Auth) (domain.Request, domain.Auth, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	st := s.states[wsID]
	if st == nil {
		return domain.Request{}, domain.Auth{}, nil, ErrNotFound
	}
	var coll *domain.Collection
	for _, c := range st.collections {
		if c.ID == collID {
			coll = c
			break
		}
	}

	vars := s.variablesLocked(st, collID, nodeID)
	req, missing := domain.Prepare(draftReq, vars)

	var auth domain.Auth
	if draftAuth.Type != "" && draftAuth.Type != domain.AuthInherit {
		auth = draftAuth
	} else if coll != nil {
		auth = s.authChainLocked(coll, nodeID)
	} else {
		auth = domain.Auth{Type: domain.AuthNone}
	}

	preparedAuth, authMissing := domain.PrepareAuth(auth, vars)
	return req, preparedAuth, mergeMissing(missing, authMissing), nil
}

// ResolveAuth 计算指定节点的生效认证配置（沿继承链）。
func (s *Store) ResolveAuth(collID, nodeID domain.ID) domain.Auth {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, st := range s.states {
		for _, c := range st.collections {
			if c.ID == collID {
				return s.authChainLocked(c, nodeID)
			}
		}
	}
	return domain.Auth{Type: domain.AuthNone}
}

func (s *Store) variablesLocked(st *state, collID, nodeID domain.ID) map[string]domain.Variable {
	layers := []domain.Layer{
		{Source: domain.VarGlobal, Vars: s.globals.Variables},
		{Source: domain.VarWorkspace, Vars: st.ws.Variables},
	}
	if env := s.selectedEnvironmentForCollection(st, collID); env != nil {
		layers = append(layers, domain.Layer{
			Source:  domain.VarEnvironment,
			Vars:    env.Vars,
			Current: st.local.CurrentValues,
			Secrets: st.secrets,
		})
	}
	var coll *domain.Collection
	for _, c := range st.collections {
		if c.ID == collID {
			coll = c
			break
		}
	}
	if coll == nil {
		return domain.Merge(layers)
	}
	layers = append(layers, domain.Layer{Source: domain.VarCollection, Vars: coll.Variables})
	if nodeID != "" {
		for _, folder := range domain.AncestorFolders(coll.Items, nodeID) {
			layers = append(layers, domain.Layer{Source: domain.VarFolder, Vars: folder.Variables})
		}
		if n := domain.FindIn(coll.Items, nodeID); n != nil {
			layers = append(layers, domain.Layer{Source: domain.VarRequest, Vars: n.Variables})
		}
	}
	return domain.Merge(layers)
}

func (s *Store) authChainLocked(coll *domain.Collection, nodeID domain.ID) domain.Auth {
	chain := []domain.Auth{coll.Auth}
	for _, folder := range domain.AncestorFolders(coll.Items, nodeID) {
		chain = append(chain, folder.Auth)
	}
	if n := domain.FindIn(coll.Items, nodeID); n != nil {
		chain = append(chain, n.Auth)
	}
	return domain.ResolveAuth(chain)
}

func mergeMissing(a, b []string) []string {
	if len(b) == 0 {
		return a
	}
	seen := map[string]bool{}
	out := make([]string, 0, len(a)+len(b))
	for _, v := range append(append([]string(nil), a...), b...) {
		if seen[v] {
			continue
		}
		seen[v] = true
		out = append(out, v)
	}
	sort.Strings(out)
	return out
}

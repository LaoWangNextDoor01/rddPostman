package store

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"rddpostman/internal/domain"
)

// ---- 环境 ----

// CreateEnvironment 新建一个空白环境。
func (s *Store) CreateEnvironment(wsID domain.ID, name string) (domain.Environment, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return domain.Environment{}, errors.New("环境名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return domain.Environment{}, ErrNotFound
	}
	if st.readOnly {
		return domain.Environment{}, ErrReadOnly
	}
	for _, e := range st.envs {
		if e.Name == name {
			return domain.Environment{}, ErrExists
		}
	}

	now := time.Now()
	env := domain.Environment{
		Schema:    domain.SchemaEnvironment,
		Version:   domain.Version,
		ID:        domain.NewID(),
		Name:      name,
		Vars:      []domain.KV{},
		CreatedAt: now,
		UpdatedAt: now,
	}
	if err := s.backend.SaveEnvironment(context.Background(), st.ref, env); err != nil {
		return domain.Environment{}, err
	}
	st.envs = append(st.envs, env)
	sortEnvironments(st.envs)
	if st.ws.DefaultEnvironmentID == "" {
		st.ws.DefaultEnvironmentID = env.ID
		_ = s.backend.UpdateWorkspace(context.Background(), st.ref, st.ws)
	}
	if st.local.SelectedEnvironmentID == "" {
		next := st.local.Clone()
		next.SelectedEnvironmentID = env.ID
		if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
			return domain.Environment{}, err
		}
		st.local = next
	}
	s.publishLocked()
	return env, nil
}

// CloneEnvironment 基于已有环境克隆一个新环境（方便从 dev 复制出 test / prod 等）。
// 变量键与配置将被复制，每个变量项分配新 ID，secret 真实值不默认跨环境克隆。
func (s *Store) CloneEnvironment(wsID, srcEnvID domain.ID, newName string) (domain.Environment, error) {
	newName = strings.TrimSpace(newName)
	if newName == "" {
		return domain.Environment{}, errors.New("环境名不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return domain.Environment{}, ErrNotFound
	}
	if st.readOnly {
		return domain.Environment{}, ErrReadOnly
	}
	for _, e := range st.envs {
		if e.Name == newName {
			return domain.Environment{}, ErrExists
		}
	}

	var src *domain.Environment
	for i := range st.envs {
		if st.envs[i].ID == srcEnvID {
			src = &st.envs[i]
			break
		}
	}
	if src == nil {
		return domain.Environment{}, ErrNotFound
	}

	vars := make([]domain.KV, len(src.Vars))
	for i, v := range src.Vars {
		val := v.Value
		if v.Secret {
			val = "" // 敏感信息不默认跨环境克隆
		}
		vars[i] = domain.KV{
			ID:      domain.NewID(),
			Key:     v.Key,
			Value:   val,
			Desc:    v.Desc,
			Enabled: v.Enabled,
			Secret:  v.Secret,
		}
	}

	now := time.Now()
	env := domain.Environment{
		Schema:      domain.SchemaEnvironment,
		Version:     domain.Version,
		ID:          domain.NewID(),
		Name:        newName,
		Description: src.Description,
		Vars:        vars,
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	if err := s.backend.SaveEnvironment(context.Background(), st.ref, env); err != nil {
		return domain.Environment{}, err
	}
	st.envs = append(st.envs, env)
	sortEnvironments(st.envs)
	s.publishLocked()
	return env, nil
}

// SaveEnvironment 新建或更新环境；每个环境独立维护变量，不互相污染。
func (s *Store) SaveEnvironment(wsID domain.ID, env domain.Environment) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	return s.saveEnvironmentLocked(st, env)
}

func (s *Store) saveEnvironmentLocked(st *state, env domain.Environment) error {
	now := time.Now()
	for i := range env.Vars {
		if env.Vars[i].ID == "" {
			env.Vars[i].ID = domain.NewID()
		}
		if env.Vars[i].CreatedAt.IsZero() {
			env.Vars[i].CreatedAt = now
		}
		env.Vars[i].UpdatedAt = now
	}
	if env.CreatedAt.IsZero() {
		env.CreatedAt = now
	}
	env.UpdatedAt = now

	persist := env.Clone()
	if err := s.splitEnvSecretsLocked(st, &persist); err != nil {
		return err
	}
	if err := s.backend.SaveEnvironment(context.Background(), st.ref, persist); err != nil {
		return err
	}
	replaced := false
	for i := range st.envs {
		if st.envs[i].ID == env.ID {
			st.envs[i] = env
			replaced = true
			break
		}
	}
	if !replaced {
		st.envs = append(st.envs, env)
		sortEnvironments(st.envs)
	}

	s.publishLocked()
	return nil
}

// DeleteEnvironment 删除环境；默认环境不可删除。
func (s *Store) DeleteEnvironment(wsID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	if envID == st.ws.DefaultEnvironmentID || len(st.envs) <= 1 {
		return ErrDefaultEnvironment
	}
	if err := s.backend.DeleteEnvironment(context.Background(), st.ref, envID); err != nil {
		return err
	}
	out := st.envs[:0]
	for _, e := range st.envs {
		if e.ID != envID {
			out = append(out, e)
		}
	}
	st.envs = out
	changedLocal := false
	nextLocal := st.local.Clone()
	if nextLocal.SelectedEnvironmentID == envID {
		nextLocal.SelectedEnvironmentID = st.ws.DefaultEnvironmentID
		changedLocal = true
	}
	if nextLocal.CollectionEnvironments != nil {
		for cID, boundID := range nextLocal.CollectionEnvironments {
			if boundID == envID {
				delete(nextLocal.CollectionEnvironments, cID)
				changedLocal = true
			}
		}
	}
	if changedLocal {
		if err := s.backend.SaveLocal(context.Background(), st.ref, nextLocal); err != nil {
			return err
		}
		st.local = nextLocal
	}
	s.publishLocked()
	return nil
}

// SetEnvironmentVariable 设置或更新指定环境的某个变量值（常用于脚本中 pm.environment.set 自动写入）。
func (s *Store) SetEnvironmentVariable(wsID, envID domain.ID, key, val string) error {
	key = strings.TrimSpace(key)
	if key == "" {
		return errors.New("变量键不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}

	var target *domain.Environment
	for i := range st.envs {
		if st.envs[i].ID == envID {
			target = &st.envs[i]
			break
		}
	}
	if target == nil {
		return ErrNotFound
	}

	found := false
	for i := range target.Vars {
		if target.Vars[i].Key == key {
			target.Vars[i].Value = val
			target.Vars[i].Enabled = true
			found = true
			break
		}
	}
	if !found {
		target.Vars = append(target.Vars, domain.KV{
			ID:      domain.NewID(),
			Key:     key,
			Value:   val,
			Enabled: true,
		})
	}

	return s.saveEnvironmentLocked(st, *target)
}

// SetDefaultEnvironment 设置工作区的默认环境。
func (s *Store) SetDefaultEnvironment(wsID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	if !hasEnvironment(st.envs, envID) {
		return ErrNotFound
	}
	st.ws.DefaultEnvironmentID = envID
	st.ws.UpdatedAt = time.Now()
	if err := s.backend.UpdateWorkspace(context.Background(), st.ref, st.ws); err != nil {
		return err
	}
	s.publishLocked()
	return nil
}

// DefaultEnvironment 返回工作区的默认环境副本。
func (s *Store) DefaultEnvironment(wsID domain.ID) (domain.Environment, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return domain.Environment{}, ErrNotFound
	}
	env := defaultEnvOf(st)
	if env == nil {
		return domain.Environment{}, ErrNotFound
	}
	return env.Clone(), nil
}

// AddEnvironmentVariable 向指定环境（空 envID 表示默认环境）追加一个变量。
func (s *Store) AddEnvironmentVariable(wsID, envID domain.ID, kv domain.KV) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if st.readOnly {
		return ErrReadOnly
	}
	var target *domain.Environment
	if envID == "" {
		target = defaultEnvOf(st)
	} else {
		for i := range st.envs {
			if st.envs[i].ID == envID {
				target = &st.envs[i]
				break
			}
		}
	}
	if target == nil {
		return ErrNotFound
	}
	kv.Key = strings.TrimSpace(kv.Key)
	if kv.Key == "" {
		return errors.New("变量键不能为空")
	}
	for _, v := range target.Vars {
		if v.Key == kv.Key {
			return ErrExists
		}
	}
	next := target.Clone()
	if kv.ID == "" {
		kv.ID = domain.NewID()
	}
	now := time.Now()
	if kv.CreatedAt.IsZero() {
		kv.CreatedAt = now
	}
	kv.UpdatedAt = now
	next.Vars = append(next.Vars, kv)
	return s.saveEnvironmentLocked(st, next)
}

// SelectEnvironment 切换工作区全局当前环境；空 ID 表示不使用环境。
func (s *Store) SelectEnvironment(wsID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if envID != "" && !hasEnvironment(st.envs, envID) {
		return ErrNotFound
	}
	next := st.local.Clone()
	next.SelectedEnvironmentID = envID
	if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.local = next
	s.publishLocked()
	return nil
}

// SelectCollectionEnvironment 为指定集合设置独立环境。
// envID 为空表示解除覆盖，继承工作区当前环境设置。
func (s *Store) SelectCollectionEnvironment(wsID, collID, envID domain.ID) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	if envID != "" && !hasEnvironment(st.envs, envID) {
		return ErrNotFound
	}
	next := st.local.Clone()
	if next.CollectionEnvironments == nil {
		next.CollectionEnvironments = map[domain.ID]domain.ID{}
	}
	if envID == "" {
		delete(next.CollectionEnvironments, collID)
	} else {
		next.CollectionEnvironments[collID] = envID
	}
	if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.local = next
	s.publishLocked()
	return nil
}

// CollectionEnvironment 返回指定集合当前生效的环境 ID，以及是否继承自工作区。
func (s *Store) CollectionEnvironment(wsID, collID domain.ID) (envID domain.ID, inherited bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return "", true
	}
	if collID != "" && st.local.CollectionEnvironments != nil {
		if id, ok := st.local.CollectionEnvironments[collID]; ok && id != "" && hasEnvironment(st.envs, id) {
			return id, false
		}
	}
	env := selectedEnvironment(st)
	if env != nil {
		return env.ID, true
	}
	return "", true
}

// SetCurrentValue 设置环境变量的本机覆盖值（不同步）。
func (s *Store) SetCurrentValue(wsID, varID domain.ID, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	st := s.states[wsID]
	if st == nil {
		return ErrNotFound
	}
	next := st.local.Clone()
	if next.CurrentValues == nil {
		next.CurrentValues = map[domain.ID]string{}
	}
	next.CurrentValues[varID] = value
	if err := s.backend.SaveLocal(context.Background(), st.ref, next); err != nil {
		return err
	}
	st.local = next
	s.publishLocked()
	return nil
}

func (s *Store) selectedEnvironmentForCollection(st *state, collID domain.ID) *domain.Environment {
	if collID != "" && st.local.CollectionEnvironments != nil {
		if envID, ok := st.local.CollectionEnvironments[collID]; ok && envID != "" {
			for i := range st.envs {
				if st.envs[i].ID == envID {
					return &st.envs[i]
				}
			}
		}
	}
	return selectedEnvironment(st)
}

func defaultEnvOf(st *state) *domain.Environment {
	if st == nil {
		return nil
	}
	if st.ws.DefaultEnvironmentID != "" {
		for i := range st.envs {
			if st.envs[i].ID == st.ws.DefaultEnvironmentID {
				return &st.envs[i]
			}
		}
	}
	if len(st.envs) > 0 {
		return &st.envs[0]
	}
	return nil
}

func (s *Store) ensureEnvironmentsLocked(ctx context.Context, st *state) error {
	now := time.Now()
	if len(st.envs) == 0 {
		defaultEnv := domain.Environment{
			Schema:    domain.SchemaEnvironment,
			Version:   domain.Version,
			ID:        domain.NewID(),
			Name:      domain.DefaultEnvironmentName,
			Vars:      []domain.KV{},
			CreatedAt: now,
			UpdatedAt: now,
		}
		if err := s.backend.SaveEnvironment(ctx, st.ref, defaultEnv); err != nil {
			return err
		}
		st.envs = []domain.Environment{defaultEnv}
		st.ws.DefaultEnvironmentID = defaultEnv.ID
		_ = s.backend.UpdateWorkspace(ctx, st.ref, st.ws)
		if st.local.SelectedEnvironmentID == "" {
			st.local.SelectedEnvironmentID = defaultEnv.ID
			_ = s.backend.SaveLocal(ctx, st.ref, st.local)
		}
		return nil
	}

	if !hasEnvironment(st.envs, st.ws.DefaultEnvironmentID) {
		st.ws.DefaultEnvironmentID = st.envs[0].ID
		_ = s.backend.UpdateWorkspace(ctx, st.ref, st.ws)
	}
	return nil
}

func selectedEnvironment(st *state) *domain.Environment {
	id := st.local.SelectedEnvironmentID
	if id == "" {
		id = st.ws.DefaultEnvironmentID
	}
	if id == "" {
		return nil
	}
	for i := range st.envs {
		if st.envs[i].ID == id {
			return &st.envs[i]
		}
	}
	return nil
}

func hasEnvironment(envs []domain.Environment, id domain.ID) bool {
	for _, e := range envs {
		if e.ID == id {
			return true
		}
	}
	return false
}

func sortEnvironments(list []domain.Environment) {
	sort.Slice(list, func(i, j int) bool { return list[i].Name < list[j].Name })
}

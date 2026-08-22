package alert

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"dockeradmin/internal/model"
)

// Store 告警规则存储：内存索引 + JSON 文件持久化（热更新，重启不丢）。
type Store struct {
	mu     sync.RWMutex
	rules  map[string]*model.AlertRule
	order  []string // 保持创建顺序
	events []model.AlertEvent
	path   string
	log    *slog.Logger
}

func NewStore(path string, log *slog.Logger) (*Store, error) {
	s := &Store{rules: make(map[string]*model.AlertRule), path: path, log: log}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, fmt.Errorf("create data dir: %w", err)
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load 读取并逐条校验规则文件（全局记忆：反序列化须做结构完整性校验，坏条目跳过而非整体失败）。
func (s *Store) load() error {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("read rules file: %w", err)
	}
	var rules []model.AlertRule
	if err := json.Unmarshal(data, &rules); err != nil {
		s.log.Warn("rules file corrupted, starting empty", "err", err)
		return nil
	}
	loaded := 0
	for i := range rules {
		r := rules[i]
		if r.ID == "" || len(model.ValidateRule(&r)) > 0 {
			s.log.Warn("skipping invalid persisted rule", "id", r.ID, "name", r.Name)
			continue
		}
		rc := r
		s.rules[r.ID] = &rc
		s.order = append(s.order, r.ID)
		loaded++
	}
	if loaded > 0 {
		s.log.Info("alert rules loaded", "count", loaded)
	}
	return nil
}

func (s *Store) persistLocked() error {
	rules := make([]model.AlertRule, 0, len(s.order))
	for _, id := range s.order {
		rules = append(rules, *s.rules[id])
	}
	data, err := json.MarshalIndent(rules, "", "  ")
	if err != nil {
		return err
	}
	tmp := s.path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, s.path) // 原子替换，防半写文件
}

func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

func (s *Store) List() []model.AlertRule {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]model.AlertRule, 0, len(s.order))
	for _, id := range s.order {
		out = append(out, *s.rules[id])
	}
	return out
}

func (s *Store) Get(id string) (model.AlertRule, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	r, ok := s.rules[id]
	if !ok {
		return model.AlertRule{}, false
	}
	return *r, true
}

func (s *Store) Create(r *model.AlertRule) (model.AlertRule, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	r.ID = newID()
	r.CreatedAt = time.Now()
	r.UpdatedAt = r.CreatedAt
	rc := *r
	s.rules[r.ID] = &rc
	s.order = append(s.order, r.ID)
	if err := s.persistLocked(); err != nil {
		delete(s.rules, r.ID)
		s.order = s.order[:len(s.order)-1]
		return model.AlertRule{}, fmt.Errorf("persist rule: %w", err)
	}
	return *r, nil
}

func (s *Store) Update(id string, r *model.AlertRule) (model.AlertRule, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	existing, ok := s.rules[id]
	if !ok {
		return model.AlertRule{}, false, nil
	}
	r.ID = id
	r.CreatedAt = existing.CreatedAt
	r.UpdatedAt = time.Now()
	rc := *r
	s.rules[id] = &rc
	if err := s.persistLocked(); err != nil {
		s.rules[id] = existing
		return model.AlertRule{}, true, fmt.Errorf("persist rule: %w", err)
	}
	return *r, true, nil
}

func (s *Store) Delete(id string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.rules[id]; !ok {
		return false, nil
	}
	backup := s.rules[id]
	delete(s.rules, id)
	idx := -1
	for i, oid := range s.order {
		if oid == id {
			idx = i
			break
		}
	}
	s.order = append(s.order[:idx], s.order[idx+1:]...)
	if err := s.persistLocked(); err != nil {
		s.rules[id] = backup
		s.order = append(s.order[:idx], append([]string{id}, s.order[idx:]...)...)
		return true, fmt.Errorf("persist delete: %w", err)
	}
	return true, nil
}

func (s *Store) AddEvent(e model.AlertEvent) {
	s.mu.Lock()
	defer s.mu.Unlock()
	e.ID = newID()
	// 最新在前，上限 200 条
	s.events = append([]model.AlertEvent{e}, s.events...)
	if len(s.events) > 200 {
		s.events = s.events[:200]
	}
}

func (s *Store) Events(limit int) []model.AlertEvent {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if limit <= 0 || limit > len(s.events) {
		limit = len(s.events)
	}
	out := make([]model.AlertEvent, limit)
	copy(out, s.events[:limit])
	return out
}

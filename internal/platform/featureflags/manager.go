package featureflags

import (
	"context"
	"encoding/json"
	"sync"
	"weak"

	"github.com/PauloHFS/goth/internal/platform/logging"
)

type Manager struct {
	repo       *Repository
	mu         sync.RWMutex
	cache      map[string]weak.Pointer[FeatureFlag]
	listeners  []func(string, bool)
	listenerMu sync.RWMutex
}

func NewManager(repo *Repository) *Manager {
	m := &Manager{
		repo:      repo,
		cache:     make(map[string]weak.Pointer[FeatureFlag]),
		listeners: make([]func(string, bool), 0),
	}

	return m
}

func (m *Manager) IsEnabled(ctx context.Context, name, tenantID string) bool {
	key := m.makeKey(name, tenantID)

	if flag, ok := m.getFromCache(key); ok {
		return flag.Enabled
	}

	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		logging.New("warn").Warn("feature flag error", "name", name, "error", err)
		return false
	}

	if flag == nil {
		return false
	}

	m.setToCache(key, flag)

	return flag.Enabled
}

func (m *Manager) GetMetadata(ctx context.Context, name, tenantID string) map[string]interface{} {
	key := m.makeKey(name, tenantID)

	flag, ok := m.getFromCache(key)
	if !ok {
		var err error
		flag, err = m.repo.GetByName(ctx, name, tenantID)
		if err != nil || flag == nil {
			return nil
		}

		m.setToCache(key, flag)
	}

	if flag.Metadata == "" {
		return nil
	}

	var metadata map[string]interface{}
	if err := json.Unmarshal([]byte(flag.Metadata), &metadata); err != nil {
		return nil
	}

	return metadata
}

func (m *Manager) GetMetadataValue(ctx context.Context, name, tenantID, key string) interface{} {
	metadata := m.GetMetadata(ctx, name, tenantID)
	if metadata == nil {
		return nil
	}
	return metadata[key]
}

func (m *Manager) Enable(ctx context.Context, name, tenantID string) error {
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return err
	}

	if flag == nil {
		input := FeatureFlagInput{
			Name:     name,
			TenantID: tenantID,
			Enabled:  true,
		}
		_, err = m.repo.Create(ctx, input)
		return err
	}

	if !flag.Enabled {
		_, err = m.repo.Toggle(ctx, flag.ID)
		if err == nil {
			m.notifyListeners(name, true)
			m.invalidateCache(name, tenantID)
		}
		return err
	}

	return nil
}

func (m *Manager) Disable(ctx context.Context, name, tenantID string) error {
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return err
	}

	if flag == nil {
		input := FeatureFlagInput{
			Name:     name,
			TenantID: tenantID,
			Enabled:  false,
		}
		_, err = m.repo.Create(ctx, input)
		return err
	}

	if flag.Enabled {
		_, err = m.repo.Toggle(ctx, flag.ID)
		if err == nil {
			m.notifyListeners(name, false)
			m.invalidateCache(name, tenantID)
		}
		return err
	}

	return nil
}

func (m *Manager) GetAll(ctx context.Context, tenantID string) ([]FeatureFlag, error) {
	return m.repo.GetAll(ctx, tenantID)
}

func (m *Manager) Create(ctx context.Context, input FeatureFlagInput) (*FeatureFlag, error) {
	flag, err := m.repo.Create(ctx, input)
	if err == nil {
		m.invalidateCache(input.Name, input.TenantID)
	}
	return flag, err
}

func (m *Manager) Update(ctx context.Context, id int64, input FeatureFlagInput) (*FeatureFlag, error) {
	flag, err := m.repo.Update(ctx, id, input)
	if err == nil {
		m.invalidateCache(input.Name, input.TenantID)
	}
	return flag, err
}

func (m *Manager) Delete(ctx context.Context, id int64) error {
	flag, err := m.repo.GetByID(ctx, id)
	if err != nil {
		return err
	}
	if flag != nil {
		m.invalidateCache(flag.Name, flag.TenantID)
	}
	return m.repo.Delete(ctx, id)
}

func (m *Manager) RegisterListener(fn func(string, bool)) {
	m.listenerMu.Lock()
	defer m.listenerMu.Unlock()
	m.listeners = append(m.listeners, fn)
}

func (m *Manager) notifyListeners(name string, enabled bool) {
	m.listenerMu.RLock()
	defer m.listenerMu.RUnlock()

	for _, listener := range m.listeners {
		go listener(name, enabled)
	}
}

func (m *Manager) invalidateCache(name, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(name, tenantID)
	delete(m.cache, key)
}

func (m *Manager) getFromCache(key string) (*FeatureFlag, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wp, ok := m.cache[key]
	if !ok {
		return nil, false
	}

	flag := wp.Value()
	if flag == nil {
		return nil, false
	}

	return flag, true
}

func (m *Manager) setToCache(key string, flag *FeatureFlag) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache[key] = weak.Make[FeatureFlag](flag)
}

func (m *Manager) makeKey(name, tenantID string) string {
	return name + ":" + tenantID
}

func (m *Manager) Refresh(ctx context.Context, name, tenantID string) error {
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(name, tenantID)
	if flag == nil {
		delete(m.cache, key)
	} else {
		m.cache[key] = weak.Make[FeatureFlag](flag)
	}

	return nil
}

func (m *Manager) RefreshAll(ctx context.Context, tenantID string) error {
	flags, err := m.repo.GetAll(ctx, tenantID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.cache = make(map[string]weak.Pointer[FeatureFlag])

	for i := range flags {
		flag := &flags[i]
		key := m.makeKey(flag.Name, flag.TenantID)
		m.cache[key] = weak.Make[FeatureFlag](flag)
	}

	return nil
}

func (m *Manager) Toggle(ctx context.Context, id int64) (*FeatureFlag, error) {
	flag, err := m.repo.Toggle(ctx, id)
	if err != nil {
		return nil, err
	}

	if flag == nil {
		return nil, nil
	}

	m.invalidateCache(flag.Name, flag.TenantID)
	m.notifyListeners(flag.Name, flag.Enabled)

	return flag, nil
}

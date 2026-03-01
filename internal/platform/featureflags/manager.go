package featureflags

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/platform/logging"
)

// Manager gerencia feature flags com cache em memória
type Manager struct {
	repo       *Repository
	mu         sync.RWMutex
	cache      map[string]*FeatureFlag
	cacheTime  map[string]time.Time
	cacheTTL   time.Duration
	listeners  []func(string, bool)
	listenerMu sync.RWMutex
}

// NewManager cria nova instância do gerenciador de feature flags
func NewManager(repo *Repository, ttl time.Duration) *Manager {
	m := &Manager{
		repo:      repo,
		cache:     make(map[string]*FeatureFlag),
		cacheTime: make(map[string]time.Time),
		cacheTTL:  ttl,
		listeners: make([]func(string, bool), 0),
	}

	// Iniciar goroutine de cleanup do cache
	go m.startCacheCleanup()

	return m
}

// startCacheCleanup remove entradas expiradas do cache periodicamente
func (m *Manager) startCacheCleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()
		for key, t := range m.cacheTime {
			if now.Sub(t) > m.cacheTTL {
				delete(m.cache, key)
				delete(m.cacheTime, key)
			}
		}
		m.mu.Unlock()
	}
}

// IsEnabled verifica se uma feature flag está habilitada (com cache)
func (m *Manager) IsEnabled(ctx context.Context, name, tenantID string) bool {
	key := m.makeKey(name, tenantID)

	// Tentar cache primeiro
	if flag, ok := m.getFromCache(key); ok {
		return flag.Enabled
	}

	// Buscar do repositório
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		logging.New("warn").Warn("feature flag error", "name", name, "error", err)
		return false
	}

	if flag == nil {
		return false
	}

	// Atualizar cache
	m.mu.Lock()
	m.cache[key] = flag
	m.cacheTime[key] = time.Now()
	m.mu.Unlock()

	return flag.Enabled
}

// GetMetadata retorna metadata de uma feature flag
func (m *Manager) GetMetadata(ctx context.Context, name, tenantID string) map[string]interface{} {
	key := m.makeKey(name, tenantID)

	flag, ok := m.getFromCache(key)
	if !ok {
		var err error
		flag, err = m.repo.GetByName(ctx, name, tenantID)
		if err != nil || flag == nil {
			return nil
		}

		m.mu.Lock()
		m.cache[key] = flag
		m.cacheTime[key] = time.Now()
		m.mu.Unlock()
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

// GetMetadataValue retorna um valor específico do metadata
func (m *Manager) GetMetadataValue(ctx context.Context, name, tenantID, key string) interface{} {
	metadata := m.GetMetadata(ctx, name, tenantID)
	if metadata == nil {
		return nil
	}
	return metadata[key]
}

// Enable habilita uma feature flag
func (m *Manager) Enable(ctx context.Context, name, tenantID string) error {
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return err
	}

	if flag == nil {
		// Criar nova flag
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

// Disable desabilita uma feature flag
func (m *Manager) Disable(ctx context.Context, name, tenantID string) error {
	flag, err := m.repo.GetByName(ctx, name, tenantID)
	if err != nil {
		return err
	}

	if flag == nil {
		// Criar nova flag desabilitada
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

// GetAll retorna todas as feature flags
func (m *Manager) GetAll(ctx context.Context, tenantID string) ([]FeatureFlag, error) {
	return m.repo.GetAll(ctx, tenantID)
}

// Create cria uma nova feature flag
func (m *Manager) Create(ctx context.Context, input FeatureFlagInput) (*FeatureFlag, error) {
	flag, err := m.repo.Create(ctx, input)
	if err == nil {
		m.invalidateCache(input.Name, input.TenantID)
	}
	return flag, err
}

// Update atualiza uma feature flag
func (m *Manager) Update(ctx context.Context, id int64, input FeatureFlagInput) (*FeatureFlag, error) {
	flag, err := m.repo.Update(ctx, id, input)
	if err == nil {
		m.invalidateCache(input.Name, input.TenantID)
	}
	return flag, err
}

// Delete remove uma feature flag
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

// RegisterListener registra um listener para mudanças de feature flags
func (m *Manager) RegisterListener(fn func(string, bool)) {
	m.listenerMu.Lock()
	defer m.listenerMu.Unlock()
	m.listeners = append(m.listeners, fn)
}

// notifyListeners notifica todos os listeners sobre mudança
func (m *Manager) notifyListeners(name string, enabled bool) {
	m.listenerMu.RLock()
	defer m.listenerMu.RUnlock()

	for _, listener := range m.listeners {
		go listener(name, enabled)
	}
}

// invalidateCache remove entrada do cache
func (m *Manager) invalidateCache(name, tenantID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key := m.makeKey(name, tenantID)
	delete(m.cache, key)
	delete(m.cacheTime, key)
}

// getFromCache tenta obter do cache
func (m *Manager) getFromCache(key string) (*FeatureFlag, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	flag, ok := m.cache[key]
	if !ok {
		return nil, false
	}

	// Check TTL
	if time.Since(m.cacheTime[key]) > m.cacheTTL {
		return nil, false
	}

	return flag, true
}

// makeKey cria chave única para cache
func (m *Manager) makeKey(name, tenantID string) string {
	return name + ":" + tenantID
}

// Refresh atualiza cache para uma flag específica
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
		delete(m.cacheTime, key)
	} else {
		m.cache[key] = flag
		m.cacheTime[key] = time.Now()
	}

	return nil
}

// RefreshAll recarrega todas as flags do cache
func (m *Manager) RefreshAll(ctx context.Context, tenantID string) error {
	flags, err := m.repo.GetAll(ctx, tenantID)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	// Limpar cache atual
	m.cache = make(map[string]*FeatureFlag)
	m.cacheTime = make(map[string]time.Time)

	// Recarregar
	for _, flag := range flags {
		key := m.makeKey(flag.Name, flag.TenantID)
		m.cache[key] = &flag
		m.cacheTime[key] = time.Now()
	}

	return nil
}

// Toggle alterna o estado de uma feature flag por ID
func (m *Manager) Toggle(ctx context.Context, id int64) (*FeatureFlag, error) {
	flag, err := m.repo.Toggle(ctx, id)
	if err != nil {
		return nil, err
	}

	if flag == nil {
		return nil, nil
	}

	// Invalidar cache
	m.invalidateCache(flag.Name, flag.TenantID)

	// Notificar listeners
	m.notifyListeners(flag.Name, flag.Enabled)

	return flag, nil
}

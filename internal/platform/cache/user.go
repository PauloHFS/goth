package cache

import (
	"context"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/db"
	"github.com/hashicorp/golang-lru/v2"
)

// UserCacheItem representa um item cacheado do usuário
type UserCacheItem struct {
	User      db.User
	ExpiresAt time.Time
}

// IsExpired verifica se o item expirou
func (i *UserCacheItem) IsExpired() bool {
	return time.Now().After(i.ExpiresAt)
}

// UserCache é um cache LRU para usuários
type UserCache struct {
	cache   *lru.Cache[int64, *UserCacheItem]
	ttl     time.Duration
	mu      sync.RWMutex
	hits    int64
	misses  int64
	evicts  int64
	onEvict func(int64, *UserCacheItem)
}

// UserCacheConfig configura o cache
type UserCacheConfig struct {
	// MaxItems máximo de itens no cache
	MaxItems int
	// TTL tempo de vida do cache
	TTL time.Duration
	// OnEvict callback quando item é removido
	OnEvict func(int64, *UserCacheItem)
}

// DefaultUserCacheConfig retorna configuração padrão
func DefaultUserCacheConfig() UserCacheConfig {
	return UserCacheConfig{
		MaxItems: 1000,
		TTL:      15 * time.Minute,
	}
}

// NewUserCache cria um novo cache de usuários
func NewUserCache(config UserCacheConfig) (*UserCache, error) {
	cache, err := lru.NewWithEvict[int64, *UserCacheItem](config.MaxItems, func(key int64, item *UserCacheItem) {
		if config.OnEvict != nil {
			config.OnEvict(key, item)
		}
	})
	if err != nil {
		return nil, err
	}

	return &UserCache{
		cache:   cache,
		ttl:     config.TTL,
		onEvict: config.OnEvict,
	}, nil
}

// Get retorna usuário do cache
func (c *UserCache) Get(userID int64) (db.User, bool) {
	c.mu.RLock()
	item, exists := c.cache.Get(userID)
	c.mu.RUnlock()

	if !exists {
		c.mu.Lock()
		c.misses++
		c.mu.Unlock()
		return db.User{}, false
	}

	if item.IsExpired() {
		c.mu.Lock()
		c.cache.Remove(userID)
		c.misses++
		c.mu.Unlock()
		return db.User{}, false
	}

	c.mu.Lock()
	c.hits++
	c.mu.Unlock()

	return item.User, true
}

// Set adiciona usuário ao cache
func (c *UserCache) Set(userID int64, user db.User) {
	c.mu.Lock()
	defer c.mu.Unlock()

	item := &UserCacheItem{
		User:      user,
		ExpiresAt: time.Now().Add(c.ttl),
	}

	if c.cache.Contains(userID) {
		c.cache.Remove(userID)
	}
	c.cache.Add(userID, item)
}

// Delete remove usuário do cache
func (c *UserCache) Delete(userID int64) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Remove(userID)
}

// Clear limpa todo o cache
func (c *UserCache) Clear() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cache.Purge()
}

// Len retorna número de itens no cache
func (c *UserCache) Len() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.cache.Len()
}

// Stats retorna estatísticas do cache
func (c *UserCache) Stats() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()

	total := c.hits + c.misses
	hitRate := float64(0)
	if total > 0 {
		hitRate = float64(c.hits) / float64(total) * 100
	}

	return map[string]interface{}{
		"items":     c.cache.Len(),
		"hits":      c.hits,
		"misses":    c.misses,
		"evictions": c.evicts,
		"hit_rate":  hitRate,
		"ttl_secs":  c.ttl.Seconds(),
	}
}

// GetOrLoad retorna do cache ou carrega via loader
func (c *UserCache) GetOrLoad(ctx context.Context, userID int64, loader func(context.Context, int64) (db.User, error)) (db.User, error) {
	// Try cache first
	if user, found := c.Get(userID); found {
		return user, nil
	}

	// Load from source
	user, err := loader(ctx, userID)
	if err != nil {
		return db.User{}, err
	}

	// Cache the result
	c.Set(userID, user)

	return user, nil
}

package middleware

import (
	"context"
	"net/http"
	"strconv"
	"sync"
	"time"

	"github.com/PauloHFS/goth/internal/contextkeys"
	"github.com/PauloHFS/goth/internal/db"
	"golang.org/x/time/rate"
)

// RateLimitConfig configura o rate limiter
type RateLimitConfig struct {
	// Requests por segundo
	RPS float64
	// Burst (requests instantâneos permitidos)
	Burst int
}

// DefaultRateLimitConfigs retorna configs para endpoints críticos de auth.
// Rate limiting global é feito pelo Traefik (ver traefik/dynamic/config.yml).
// Estes limites são POR USUÁRIO/IP para prevenir abuso mesmo quando o Traefik permite.
func DefaultRateLimitConfigs() map[string]RateLimitConfig {
	return map[string]RateLimitConfig{
		"/login":           {RPS: 0.1, Burst: 3},  // 3 requests por 10 segundos
		"/register":        {RPS: 0.05, Burst: 2}, // 2 requests por 20 segundos
		"/forgot-password": {RPS: 0.1, Burst: 3},  // 3 requests por 10 segundos
		"/reset-password":  {RPS: 0.1, Burst: 3},  // 3 requests por 10 segundos
	}
}

// rateLimiter mantém limiters por IP e por usuário
type rateLimiter struct {
	mu       sync.RWMutex
	limiters map[string]*rate.Limiter
	config   RateLimitConfig
}

// newRateLimiter cria um novo rate limiter
func newRateLimiter(config RateLimitConfig) *rateLimiter {
	return &rateLimiter{
		limiters: make(map[string]*rate.Limiter),
		config:   config,
	}
}

// getLimiter retorna ou cria um limiter para uma chave (IP ou user_id)
func (rl *rateLimiter) getLimiter(key string) *rate.Limiter {
	rl.mu.RLock()
	limiter, exists := rl.limiters[key]
	rl.mu.RUnlock()

	if exists {
		return limiter
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	// Double-check after acquiring write lock
	if limiter, exists = rl.limiters[key]; exists {
		return limiter
	}

	limiter = rate.NewLimiter(rate.Limit(rl.config.RPS), rl.config.Burst)
	rl.limiters[key] = limiter

	// Cleanup old limiters (simples GC - em produção usar LRU cache)
	go func() {
		time.Sleep(10 * time.Minute)
		rl.mu.Lock()
		delete(rl.limiters, key)
		rl.mu.Unlock()
	}()

	return limiter
}

// RateLimitMiddleware cria middleware de rate limiting por IP e usuário
// para endpoints críticos de auth apenas.
//
// O Traefik faz rate limiting global no nível de infraestrutura.
// Este middleware é uma camada adicional de proteção para prevenir
// abuso de endpoints sensíveis (login, register, password reset).
func RateLimitMiddleware(configs map[string]RateLimitConfig) func(http.Handler) http.Handler {
	if configs == nil {
		configs = DefaultRateLimitConfigs()
	}

	// Criar limiters por path de auth
	limiters := make(map[string]*rateLimiter)
	for path, config := range configs {
		limiters[path] = newRateLimiter(config)
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Encontrar limiter para este path de auth
			var limiter *rateLimiter

			for path, l := range limiters {
				if r.URL.Path == path {
					limiter = l
					break
				}
			}

			// Se não é endpoint de auth, skip (Traefik faz rate limiting global)
			if limiter == nil {
				next.ServeHTTP(w, r)
				return
			}

			// Identificador: user_id se autenticado, IP se não
			key := r.RemoteAddr

			// Tentar pegar user_id do contexto (se middleware de auth rodou antes)
			if userID := getUserIDFromContext(r.Context()); userID > 0 {
				key = "user:" + strconv.FormatInt(userID, 10)
			}

			// Get limiter para esta chave
			rl := limiter.getLimiter(key)

			// Check rate limit
			if !rl.Allow() {
				// Rate limited
				w.Header().Set("Content-Type", "application/json")
				w.Header().Set("Retry-After", "60")
				w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.config.Burst))
				w.Header().Set("X-RateLimit-Remaining", "0")
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":"rate_limit_exceeded","message":"Too many requests. Please try again later."}`))
				return
			}

			// Adicionar headers de rate limit
			w.Header().Set("X-RateLimit-Limit", strconv.Itoa(limiter.config.Burst))
			w.Header().Set("X-RateLimit-Remaining", strconv.Itoa(limiter.config.Burst-1))

			next.ServeHTTP(w, r)
		})
	}
}

// getUserIDFromContext tenta extrair user_id do contexto
func getUserIDFromContext(ctx context.Context) int64 {
	if user, ok := ctx.Value(contextkeys.UserContextKey).(db.User); ok {
		return user.ID
	}
	return 0
}

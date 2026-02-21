package middleware

import (
	"encoding/json"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type RateLimitConfig struct {
	Rate      rate.Limit
	Burst     int
	Window    time.Duration
	KeyFunc   func(*http.Request) string
	OnLimited func(http.ResponseWriter, *http.Request, time.Duration)
}

var DefaultRateLimitConfigs = map[string]RateLimitConfig{
	"auth": {
		Rate:   rate.Limit(5),
		Burst:  10,
		Window: time.Minute,
	},
	"api": {
		Rate:   rate.Limit(20),
		Burst:  40,
		Window: time.Minute,
	},
	"webhook": {
		Rate:   rate.Limit(100),
		Burst:  200,
		Window: time.Minute,
	},
	"upload": {
		Rate:   rate.Limit(2),
		Burst:  5,
		Window: time.Minute,
	},
	"default": {
		Rate:   rate.Limit(10),
		Burst:  20,
		Window: time.Minute,
	},
}

type limiterEntry struct {
	limiter  *rate.Limiter
	lastSeen time.Time
}

type RateLimiter struct {
	limiters sync.Map
	config   RateLimitConfig
	category string
	stopCh   chan struct{}
	mu       sync.Mutex
}

func NewRateLimiter(category string, cfg RateLimitConfig) *RateLimiter {
	rl := &RateLimiter{
		config:   cfg,
		category: category,
		stopCh:   make(chan struct{}),
	}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := rl.getKey(r)

		entry, _ := rl.limiters.LoadOrStore(key, &limiterEntry{
			limiter: rate.NewLimiter(rl.config.Rate, rl.config.Burst),
		})

		e := entry.(*limiterEntry)
		e.lastSeen = time.Now()

		if !e.limiter.Allow() {
			rl.onLimited(w, r)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func (rl *RateLimiter) getKey(r *http.Request) string {
	if rl.config.KeyFunc != nil {
		return rl.config.KeyFunc(r)
	}
	ip := ExtractIP(r)
	return rl.category + ":" + ip
}

func (rl *RateLimiter) onLimited(w http.ResponseWriter, r *http.Request) {
	if rl.config.OnLimited != nil {
		rl.config.OnLimited(w, r, rl.config.Window)
		return
	}

	retryAfter := int(rl.config.Window.Seconds())
	w.Header().Set("Retry-After", strconv.Itoa(retryAfter))
	w.Header().Set("X-RateLimit-Limit", strconv.Itoa(int(rl.config.Rate)*int(rl.config.Window.Seconds())))
	w.Header().Set("X-RateLimit-Remaining", "0")
	w.Header().Set("X-RateLimit-Reset", strconv.Itoa(int(time.Now().Add(rl.config.Window).Unix())))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusTooManyRequests)
	json.NewEncoder(w).Encode(map[string]interface{}{
		"error":       "rate limit exceeded",
		"retry_after": retryAfter,
		"category":    rl.category,
	})
}

func (rl *RateLimiter) cleanup() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			rl.limiters.Range(func(key, value interface{}) bool {
				entry := value.(*limiterEntry)
				if time.Since(entry.lastSeen) > 3*time.Minute {
					rl.limiters.Delete(key)
				}
				return true
			})
		case <-rl.stopCh:
			return
		}
	}
}

func (rl *RateLimiter) Stop() {
	close(rl.stopCh)
}

func ExtractIP(r *http.Request) string {
	if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
		if idx := strings.Index(xff, ","); idx > 0 {
			return strings.TrimSpace(xff[:idx])
		}
		return strings.TrimSpace(xff)
	}

	if xri := r.Header.Get("X-Real-IP"); xri != "" {
		return strings.TrimSpace(xri)
	}

	ip, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return ip
}

func RateLimitAuth(next http.Handler) http.Handler {
	return NewRateLimiter("auth", DefaultRateLimitConfigs["auth"]).Middleware(next)
}

func RateLimitAPI(next http.Handler) http.Handler {
	return NewRateLimiter("api", DefaultRateLimitConfigs["api"]).Middleware(next)
}

func RateLimitWebhook(next http.Handler) http.Handler {
	return NewRateLimiter("webhook", DefaultRateLimitConfigs["webhook"]).Middleware(next)
}

func RateLimitUpload(next http.Handler) http.Handler {
	return NewRateLimiter("upload", DefaultRateLimitConfigs["upload"]).Middleware(next)
}

func RateLimitDefault(next http.Handler) http.Handler {
	return NewRateLimiter("default", DefaultRateLimitConfigs["default"]).Middleware(next)
}

func RateLimit(cfg RateLimitConfig) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return NewRateLimiter("custom", cfg).Middleware(next)
	}
}

package worker

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type JobRateConfig struct {
	Concurrency int
	Rate        rate.Limit
	Burst       int
}

var DefaultJobRateConfigs = map[string]JobRateConfig{
	"send_email":                {Concurrency: 5, Rate: 2, Burst: 5},
	"send_verification_email":   {Concurrency: 5, Rate: 2, Burst: 5},
	"send_password_reset_email": {Concurrency: 5, Rate: 2, Burst: 5},
	"process_ai":                {Concurrency: 3, Rate: 1, Burst: 3},
	"process_webhook":           {Concurrency: 10, Rate: 5, Burst: 10},
}

type JobRateLimiter struct {
	semaphores map[string]chan struct{}
	limiters   map[string]*rate.Limiter
	mu         sync.RWMutex
}

func NewJobRateLimiter() *JobRateLimiter {
	jrl := &JobRateLimiter{
		semaphores: make(map[string]chan struct{}),
		limiters:   make(map[string]*rate.Limiter),
	}

	for jobType, cfg := range DefaultJobRateConfigs {
		jrl.semaphores[jobType] = make(chan struct{}, cfg.Concurrency)
		jrl.limiters[jobType] = rate.NewLimiter(cfg.Rate, cfg.Burst)
	}

	jrl.semaphores["default"] = make(chan struct{}, 5)
	jrl.limiters["default"] = rate.NewLimiter(1, 5)

	return jrl
}

func (jrl *JobRateLimiter) Acquire(ctx context.Context, jobType string) error {
	limiter := jrl.getLimiter(jobType)
	if err := limiter.Wait(ctx); err != nil {
		return err
	}

	semaphore := jrl.getSemaphore(jobType)
	select {
	case semaphore <- struct{}{}:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (jrl *JobRateLimiter) Release(jobType string) {
	semaphore := jrl.getSemaphore(jobType)
	select {
	case <-semaphore:
	default:
	}
}

func (jrl *JobRateLimiter) getLimiter(jobType string) *rate.Limiter {
	jrl.mu.RLock()
	defer jrl.mu.RUnlock()

	if limiter, ok := jrl.limiters[jobType]; ok {
		return limiter
	}
	return jrl.limiters["default"]
}

func (jrl *JobRateLimiter) getSemaphore(jobType string) chan struct{} {
	jrl.mu.RLock()
	defer jrl.mu.RUnlock()

	if sem, ok := jrl.semaphores[jobType]; ok {
		return sem
	}
	return jrl.semaphores["default"]
}

func (jrl *JobRateLimiter) GetStats() map[string]interface{} {
	jrl.mu.RLock()
	defer jrl.mu.RUnlock()

	stats := make(map[string]interface{})
	for jobType := range jrl.semaphores {
		stats[jobType] = map[string]interface{}{
			"concurrency": cap(jrl.semaphores[jobType]),
			"in_use":      len(jrl.semaphores[jobType]),
			"rate":        float64(jrl.limiters[jobType].Limit()),
		}
	}
	return stats
}

func IsExternalRateLimitError(err error) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	return strings.Contains(errStr, "rate limit") ||
		strings.Contains(errStr, "too many requests") ||
		strings.Contains(errStr, "429") ||
		strings.Contains(errStr, "quota exceeded") ||
		strings.Contains(errStr, "throttl")
}

func GetRetryAfterDuration(err error) time.Duration {
	if err == nil {
		return 0
	}

	errStr := strings.ToLower(err.Error())

	if strings.Contains(errStr, "retry-after") {
		parts := strings.Split(errStr, "retry-after")
		if len(parts) > 1 {
			rest := strings.TrimSpace(parts[1])
			rest = strings.TrimPrefix(rest, ":")
			rest = strings.TrimPrefix(rest, "=")
			rest = strings.TrimSpace(rest)

			if seconds := parseRetrySeconds(rest); seconds > 0 {
				return time.Duration(seconds) * time.Second
			}
		}
	}

	if IsExternalRateLimitError(err) {
		return time.Minute
	}

	return 0
}

func parseRetrySeconds(s string) int {
	s = strings.TrimSpace(s)
	s = strings.Split(s, " ")[0]
	s = strings.Split(s, ",")[0]
	s = strings.Split(s, "}")[0]

	var n int
	if _, err := strconv.ParseInt(s, 10, 64); err == nil {
		n, _ = strconv.Atoi(s)
		return n
	}

	if duration, err := time.ParseDuration(s); err == nil {
		return int(duration.Seconds())
	}

	return 0
}

var (
	ErrRateLimitExceeded = errors.New("rate limit exceeded")
	ErrConcurrencyLimit  = errors.New("concurrency limit reached")
)

package worker

import (
	"context"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type APIThrottler struct {
	limiters map[string]*rate.Limiter
	configs  map[string]*apiThrottleConfig
	mu       sync.RWMutex
	stopChan chan struct{}
}

type apiThrottleConfig struct {
	rps   float64
	burst int
}

var DefaultAPILimits = map[string]apiThrottleConfig{
	"send_email":                {rps: 5, burst: 10},
	"send_password_reset_email": {rps: 2, burst: 5},
	"send_verification_email":   {rps: 2, burst: 5},
	"process_ai":                {rps: 1, burst: 3},
	"process_webhook":           {rps: 50, burst: 100},
	"process_asaas_webhook":     {rps: 10, burst: 20},
	"asaas_payment":             {rps: 10, burst: 20},
	"asaas_subscription":        {rps: 10, burst: 20},
}

func NewAPIThrottler() *APIThrottler {
	t := &APIThrottler{
		limiters: make(map[string]*rate.Limiter),
		configs:  make(map[string]*apiThrottleConfig),
		stopChan: make(chan struct{}),
	}

	for apiName, cfg := range DefaultAPILimits {
		cfg := cfg
		t.configs[apiName] = &cfg
		t.limiters[apiName] = rate.NewLimiter(rate.Limit(cfg.rps), cfg.burst)
	}

	return t
}

func (t *APIThrottler) Allow(apiName string) bool {
	t.mu.RLock()
	limiter, exists := t.limiters[apiName]
	t.mu.RUnlock()

	if !exists {
		t.mu.Lock()
		if cfg, ok := DefaultAPILimits[apiName]; ok {
			t.configs[apiName] = &cfg
			limiter = rate.NewLimiter(rate.Limit(cfg.rps), cfg.burst)
			t.limiters[apiName] = limiter
		} else {
			limiter = rate.NewLimiter(10, 20)
			t.limiters[apiName] = limiter
		}
		t.mu.Unlock()
	}

	return limiter.Allow()
}

func (t *APIThrottler) Wait(ctx context.Context, apiName string) error {
	t.mu.RLock()
	limiter, exists := t.limiters[apiName]
	t.mu.RUnlock()

	if !exists {
		t.mu.Lock()
		cfg := apiThrottleConfig{rps: 10, burst: 20}
		if c, ok := DefaultAPILimits[apiName]; ok {
			cfg = c
		}
		limiter = rate.NewLimiter(rate.Limit(cfg.rps), cfg.burst)
		t.limiters[apiName] = limiter
		t.configs[apiName] = &cfg
		t.mu.Unlock()
	}

	return limiter.Wait(ctx)
}

func (t *APIThrottler) Delay(apiName string) time.Duration {
	t.mu.RLock()
	limiter, exists := t.limiters[apiName]
	t.mu.RUnlock()

	if !exists {
		return 0
	}

	return limiter.Reserve().Delay()
}

func (t *APIThrottler) Stop() {
	close(t.stopChan)
}

func (t *APIThrottler) GetConfig(apiName string) (float64, int) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if cfg := t.configs[apiName]; cfg != nil {
		return cfg.rps, cfg.burst
	}

	if cfg, ok := DefaultAPILimits[apiName]; ok {
		return cfg.rps, cfg.burst
	}

	return 10, 20
}

func (t *APIThrottler) GetAllConfigs() map[string]apiThrottleConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()

	result := make(map[string]apiThrottleConfig)
	for apiName, cfg := range t.configs {
		result[apiName] = *cfg
	}
	return result
}

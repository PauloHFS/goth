package worker

import (
	"context"
	"errors"
	"math/rand"
	"strings"
	"time"
)

type BackoffConfig struct {
	BaseDelay time.Duration
	MaxDelay  time.Duration
}

var DefaultBackoffConfig = BackoffConfig{
	BaseDelay: 100 * time.Millisecond,
	MaxDelay:  30 * time.Second,
}

func FullJitter(attempt int, cfg BackoffConfig) time.Duration {
	if attempt <= 0 {
		return cfg.BaseDelay
	}

	exp := min(cfg.BaseDelay*time.Duration(1<<attempt), cfg.MaxDelay)

	jitter := time.Duration(rand.Int63n(int64(exp)))

	return exp/2 + jitter
}

func Exponential(attempt int, cfg BackoffConfig) time.Duration {
	if attempt <= 0 {
		return cfg.BaseDelay
	}

	exp := min(cfg.BaseDelay*time.Duration(1<<attempt), cfg.MaxDelay)

	return exp
}

var retryableErrors = []string{
	"rate limit",
	"quota",
	"too many requests",
	"429",
	"500",
	"502",
	"503",
	"504",
	"connection refused",
	"context deadline exceeded",
	"i/o timeout",
	"TEMPORARY_FAILURE",
	"TRY_AGAIN",
}

func IsRetryableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := strings.ToLower(err.Error())

	for _, pattern := range retryableErrors {
		if strings.Contains(errStr, strings.ToLower(pattern)) {
			return true
		}
	}

	return errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled)
}

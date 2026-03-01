package httpclient

import (
	"context"
	"errors"
	"fmt"
	"math"
	"math/rand"
	"time"
)

// RetryConfig configura o retry com backoff
type RetryConfig struct {
	// MaxRetries máximo de tentativas
	MaxRetries int
	// InitialBackoff tempo inicial de espera
	InitialBackoff time.Duration
	// MaxBackoff tempo máximo de espera
	MaxBackoff time.Duration
	// Multiplier do backoff exponencial
	Multiplier float64
	// Jitter adiciona aleatoriedade (0-1)
	Jitter float64
}

// DefaultRetryConfig retorna configuração padrão
func DefaultRetryConfig() RetryConfig {
	return RetryConfig{
		MaxRetries:     3,
		InitialBackoff: 100 * time.Millisecond,
		MaxBackoff:     10 * time.Second,
		Multiplier:     2.0,
		Jitter:         0.1, // 10% jitter
	}
}

// RetryableError é um erro que pode ser retry
type RetryableError struct {
	Err error
}

func (e *RetryableError) Error() string {
	return fmt.Sprintf("retryable: %v", e.Err)
}

func (e *RetryableError) Unwrap() error {
	return e.Err
}

// IsRetryable verifica se erro é retryable
func IsRetryable(err error) bool {
	var re *RetryableError
	return errors.As(err, &re)
}

// Retry executa fn com retry e backoff exponencial
func Retry(ctx context.Context, config RetryConfig, fn func() error) error {
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		// Execute function
		err := fn()
		if err == nil {
			return nil // Success
		}

		lastErr = err

		// Check if retryable
		if !IsRetryable(err) && attempt < config.MaxRetries {
			// Non-retryable error, don't retry
			return err
		}

		// Last attempt, return error
		if attempt == config.MaxRetries {
			return err
		}

		// Calculate backoff with exponential increase
		backoff := calculateBackoff(attempt, config)

		// Wait with context support
		select {
		case <-ctx.Done():
			return fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue to next attempt
		}
	}

	return lastErr
}

// calculateBackoff calcula backoff com jitter
func calculateBackoff(attempt int, config RetryConfig) time.Duration {
	// Exponential backoff: initial * multiplier^attempt
	mult := math.Pow(config.Multiplier, float64(attempt))
	backoff := float64(config.InitialBackoff) * mult

	// Cap at max backoff
	if backoff > float64(config.MaxBackoff) {
		backoff = float64(config.MaxBackoff)
	}

	// Add jitter: backoff * (1 + random(-jitter, +jitter))
	if config.Jitter > 0 {
		jitterRange := backoff * config.Jitter
		jitter := (rand.Float64() * 2 * jitterRange) - jitterRange
		backoff = backoff + jitter
	}

	return time.Duration(backoff)
}

// RetryWithResult executa fn que retorna valor com retry
func RetryWithResult[T any](ctx context.Context, config RetryConfig, fn func() (T, error)) (T, error) {
	var zero T
	var lastErr error

	for attempt := 0; attempt <= config.MaxRetries; attempt++ {
		result, err := fn()
		if err == nil {
			return result, nil
		}

		lastErr = err

		// Check if retryable
		if !IsRetryable(err) && attempt < config.MaxRetries {
			return zero, err
		}

		// Last attempt
		if attempt == config.MaxRetries {
			return zero, err
		}

		// Wait before retry
		backoff := calculateBackoff(attempt, config)

		select {
		case <-ctx.Done():
			return zero, fmt.Errorf("retry cancelled: %w", ctx.Err())
		case <-time.After(backoff):
			// Continue
		}
	}

	return zero, lastErr
}
